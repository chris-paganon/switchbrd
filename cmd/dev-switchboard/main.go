package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dev-switchboard/internal/control"
	"dev-switchboard/internal/proxy"
	"dev-switchboard/internal/registry"
)

var proxyListenAddrs = []string{
	"127.0.0.1:5173",
	"[::1]:5173",
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	client := control.NewClient(control.SocketPath())

	switch args[0] {
	case "serve":
		return serve(ctx)
	case "add":
		if len(args) != 3 {
			return fmt.Errorf("usage: dev-switchboard add <name> <port>")
		}
		port, err := strconv.Atoi(args[2])
		if err != nil {
			return fmt.Errorf("invalid port: %w", err)
		}
		return client.Add(ctx, args[1], port)
	case "list":
		apps, activeName, err := client.List(ctx)
		if err != nil {
			return err
		}
		if len(apps) == 0 {
			fmt.Println("no apps registered")
			return nil
		}
		for _, candidate := range apps {
			marker := " "
			if candidate.Name == activeName {
				marker = "*"
			}
			fmt.Printf("%s %s %d\n", marker, candidate.Name, candidate.Port)
		}
		return nil
	case "activate":
		if len(args) != 2 {
			return fmt.Errorf("usage: dev-switchboard activate <name>")
		}
		candidate, err := client.Activate(ctx, args[1])
		if err != nil {
			return err
		}
		fmt.Printf("active app: %s %d\n", candidate.Name, candidate.Port)
		return nil
	case "active":
		candidate, err := client.Active(ctx)
		if err != nil {
			return err
		}
		if candidate == nil {
			fmt.Println("none")
			return nil
		}
		fmt.Printf("%s %d\n", candidate.Name, candidate.Port)
		return nil
	case "remove":
		if len(args) != 2 {
			return fmt.Errorf("usage: dev-switchboard remove <name>")
		}
		return client.Remove(ctx, args[1])

	default:
		return usageError()
	}
}

func serve(ctx context.Context) error {
	reg := registry.New()
	controlServer := control.NewServer(control.SocketPath(), reg)
	if err := controlServer.Start(); err != nil {
		return fmt.Errorf("start control server: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = controlServer.Shutdown(shutdownCtx)
	}()

	handler := proxy.NewHandler(reg)

	proxyServers, listeners, err := startProxyServers(handler, proxyListenAddrs)
	if err != nil {
		return err
	}
	defer closeListeners(listeners)

	errCh := make(chan error, len(proxyServers))
	for i := range proxyServers {
		server := proxyServers[i]
		listener := listeners[i]
		go func() {
			errCh <- server.Serve(listener)
		}()
	}

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-sigCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, server := range proxyServers {
			if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
		}
		for range proxyServers {
			serveErr := <-errCh
			if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				return serveErr
			}
		}
		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func usageError() error {
	return fmt.Errorf("usage: dev-switchboard <serve|add|list|activate|active|remove>")
}

func startProxyServers(handler http.Handler, addrs []string) ([]*http.Server, []net.Listener, error) {
	servers := make([]*http.Server, 0, len(addrs))
	listeners := make([]net.Listener, 0, len(addrs))

	for _, addr := range addrs {
		server := &http.Server{Addr: addr, Handler: handler}
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			if isOptionalIPv6Loopback(addr, err) {
				continue
			}
			closeListeners(listeners)
			return nil, nil, fmt.Errorf("listen on %s: %w", addr, err)
		}
		servers = append(servers, server)
		listeners = append(listeners, listener)
	}

	if len(servers) == 0 {
		return nil, nil, fmt.Errorf("could not bind switchboard proxy listeners")
	}

	return servers, listeners, nil
}

func closeListeners(listeners []net.Listener) {
	for _, listener := range listeners {
		_ = listener.Close()
	}
}

func isOptionalIPv6Loopback(addr string, err error) bool {
	return addr == "[::1]:5173" && strings.Contains(err.Error(), "address family not supported")
}
