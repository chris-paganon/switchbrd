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
	"syscall"
	"time"

	"dev-switchboard/internal/control"
	"dev-switchboard/internal/proxy"
	"dev-switchboard/internal/registry"
)

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

	proxyServer := &http.Server{
		Addr:    "127.0.0.1:5173",
		Handler: proxy.NewHandler(reg),
	}

	listener, err := net.Listen("tcp", proxyServer.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", proxyServer.Addr, err)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- proxyServer.Serve(listener)
	}()

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-sigCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := proxyServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		serveErr := <-errCh
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
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
