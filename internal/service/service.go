package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"dev-switchboard/internal/control"
	"dev-switchboard/internal/proxy"
	"dev-switchboard/internal/registry"
)

type Config struct {
	SocketPath       string
	ProxyListenAddrs []string
}

func Run(ctx context.Context, cfg Config) error {
	reg := registry.New()
	serviceCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.RWMutex
	activeListenAddrs := make([]string, 0, len(cfg.ProxyListenAddrs))
	statusFn := func() control.StatusData {
		status := control.StatusData{
			Running:  true,
			PID:      os.Getpid(),
			AppCount: len(reg.List()),
		}
		if active, ok := reg.Active(); ok {
			status.Active = &active
		}
		mu.RLock()
		status.ProxyListenAddrs = append([]string(nil), activeListenAddrs...)
		mu.RUnlock()
		return status
	}

	controlServer := control.NewServer(cfg.SocketPath, reg, control.ServerOptions{
		Status:   statusFn,
		Shutdown: cancel,
	})
	if err := controlServer.Start(); err != nil {
		return fmt.Errorf("start control server: %w", err)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		_ = controlServer.Shutdown(shutdownCtx)
	}()

	handler := proxy.NewHandler(reg)
	proxyServers, listeners, err := startProxyServers(handler, cfg.ProxyListenAddrs)
	if err != nil {
		return err
	}
	defer closeListeners(listeners)

	mu.Lock()
	for _, listener := range listeners {
		activeListenAddrs = append(activeListenAddrs, listener.Addr().String())
	}
	mu.Unlock()

	errCh := make(chan error, len(proxyServers))
	for i := range proxyServers {
		server := proxyServers[i]
		listener := listeners[i]
		go func() {
			errCh <- server.Serve(listener)
		}()
	}

	select {
	case <-serviceCtx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
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
		cancel()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
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
