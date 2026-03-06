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

	"dev-switchboard/internal/app"
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
		port, name, err := parsePortNameCommand("add", args[1:])
		if err != nil {
			return err
		}
		candidate, err := client.Add(ctx, port, name)
		if err != nil {
			return err
		}
		fmt.Printf("added %s\n", formatApp(candidate))
		return nil
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
			fmt.Printf("%s %s\n", marker, formatApp(candidate))
		}
		return nil
	case "activate":
		port, name, err := parsePortNameCommand("activate", args[1:])
		if err != nil {
			return err
		}
		candidate, err := client.Activate(ctx, port, name)
		if err != nil {
			return err
		}
		fmt.Printf("active app: %s\n", formatApp(*candidate))
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
		fmt.Println(formatApp(*candidate))
		return nil
	case "rename":
		oldName, newName, err := parseRenameCommand(args[1:])
		if err != nil {
			return err
		}
		candidate, err := client.Rename(ctx, oldName, newName)
		if err != nil {
			return err
		}
		fmt.Printf("renamed %s\n", formatApp(candidate))
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
	return fmt.Errorf("usage: dev-switchboard <serve|add|list|activate|active|rename|remove>")
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

func parsePortNameCommand(command string, args []string) (int, string, error) {
	var (
		name    string
		portArg string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--name", "-n":
			if i+1 >= len(args) {
				return 0, "", fmt.Errorf("usage: dev-switchboard %s <port> [--name <name>]", command)
			}
			name = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return 0, "", fmt.Errorf("usage: dev-switchboard %s <port> [--name <name>]", command)
			}
			if portArg != "" {
				return 0, "", fmt.Errorf("usage: dev-switchboard %s <port> [--name <name>]", command)
			}
			portArg = args[i]
		}
	}

	if portArg == "" {
		return 0, "", fmt.Errorf("usage: dev-switchboard %s <port> [--name <name>]", command)
	}

	port, err := strconv.Atoi(portArg)
	if err != nil {
		return 0, "", fmt.Errorf("invalid port: %w", err)
	}

	return port, strings.TrimSpace(name), nil
}

func parseRenameCommand(args []string) (string, string, error) {
	if len(args) != 2 {
		return "", "", fmt.Errorf("usage: dev-switchboard rename <old-name> <new-name>")
	}

	return strings.TrimSpace(args[0]), strings.TrimSpace(args[1]), nil
}

func formatApp(candidate app.App) string {
	return fmt.Sprintf("%s -> %d", candidate.Name, candidate.Port)
}
