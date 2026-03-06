package proxy

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"dev-switchboard/internal/app"
	"dev-switchboard/internal/registry"
)

func TestHandlerReturns503WhenNoActiveApp(t *testing.T) {
	reg := registry.New()
	recorder := httptest.NewRecorder()

	NewHandler(reg).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://switchboard/", nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestHandlerSwitchesTargets(t *testing.T) {
	reg := registry.New()
	if err := reg.Add(app.App{Name: "alpha", Port: 5174}); err != nil {
		t.Fatalf("add alpha: %v", err)
	}
	if err := reg.Add(app.App{Name: "beta", Port: 5175}); err != nil {
		t.Fatalf("add beta: %v", err)
	}
	if _, err := reg.Activate("alpha"); err != nil {
		t.Fatalf("activate alpha: %v", err)
	}

	handler := &Handler{
		registry: reg,
		transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return responseForHost(req.URL.Host), nil
		}),
	}

	assertProxyBody(t, handler, "alpha")

	if _, err := reg.Activate("beta"); err != nil {
		t.Fatalf("activate beta: %v", err)
	}

	assertProxyBody(t, handler, "beta")
}

func TestHandlerReturns502WhenBackendIsDown(t *testing.T) {
	reg := registry.New()
	if err := reg.Add(app.App{Name: "ghost", Port: 5179}); err != nil {
		t.Fatalf("add ghost: %v", err)
	}
	if _, err := reg.Activate("ghost"); err != nil {
		t.Fatalf("activate ghost: %v", err)
	}

	handler := &Handler{
		registry: reg,
		transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		}),
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://switchboard/", nil))
	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "5179") {
		t.Fatalf("expected error body to mention port, got %q", recorder.Body.String())
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func responseForHost(host string) *http.Response {
	body := "unknown"
	switch host {
	case "127.0.0.1:5174":
		body = "alpha"
	case "127.0.0.1:5175":
		body = "beta"
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func assertProxyBody(t *testing.T, handler http.Handler, want string) {
	t.Helper()

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://switchboard/", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != want {
		t.Fatalf("unexpected body: got %q want %q", recorder.Body.String(), want)
	}
}
