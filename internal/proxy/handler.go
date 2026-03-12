package proxy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"switchbrd/internal/registry"
)

type Handler struct {
	registry   *registry.Registry
	transport  http.RoundTripper
	targetHost string
}

func NewHandler(reg *registry.Registry) *Handler {
	return &Handler{
		registry:   reg,
		transport:  newLoopbackTransport(),
		targetHost: "localhost",
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	candidate, ok := h.registry.Active()
	if !ok {
		http.Error(w, registry.ErrNoActiveApp.Error(), http.StatusServiceUnavailable)
		return
	}

	target, err := url.Parse(fmt.Sprintf("http://%s:%d", h.targetHost, candidate.Port))
	if err != nil {
		http.Error(w, "invalid target app", http.StatusBadGateway)
		return
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(target)
	reverseProxy.Transport = h.transport
	reverseProxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, proxyErr error) {
		http.Error(rw, fmt.Sprintf("active app on port %d is unavailable", candidate.Port), http.StatusBadGateway)
	}
	reverseProxy.ServeHTTP(w, r)
}

func newLoopbackTransport() *http.Transport {
	base := http.DefaultTransport.(*http.Transport).Clone()
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}

	base.DialContext = func(ctx context.Context, network string, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		candidates := []string{addr}
		if strings.EqualFold(host, "localhost") {
			candidates = []string{
				net.JoinHostPort("::1", port),
				net.JoinHostPort("127.0.0.1", port),
			}
		}

		var lastErr error
		for _, candidate := range candidates {
			conn, dialErr := dialer.DialContext(ctx, network, candidate)
			if dialErr == nil {
				return conn, nil
			}
			lastErr = dialErr
		}
		return nil, lastErr
	}

	return base
}
