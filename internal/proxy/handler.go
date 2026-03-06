package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"dev-switchboard/internal/registry"
)

type Handler struct {
	registry  *registry.Registry
	transport http.RoundTripper
}

func NewHandler(reg *registry.Registry) *Handler {
	return &Handler{registry: reg, transport: http.DefaultTransport}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	candidate, ok := h.registry.Active()
	if !ok {
		http.Error(w, registry.ErrNoActiveApp.Error(), http.StatusServiceUnavailable)
		return
	}

	target, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", candidate.Port))
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
