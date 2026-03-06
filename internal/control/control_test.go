package control

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"dev-switchboard/internal/registry"
)

func TestServerHandlersLifecycle(t *testing.T) {
	server := NewServer("/tmp/dev-switchboard-test.sock", registry.New(5173), ServerOptions{})

	postJSON(t, server, http.MethodPost, "/apps", addRequest{Port: 5174}, http.StatusCreated)

	listRecorder := request(t, server, http.MethodGet, "/apps", nil)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected list status: %d", listRecorder.Code)
	}
	var listed listResponse
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(listed.Apps) != 1 || listed.ActiveName != "" {
		t.Fatalf("unexpected list response: %+v", listed)
	}
	if listed.Apps[0].Name != "5174" {
		t.Fatalf("unexpected default app name: %+v", listed.Apps[0])
	}

	activeRecorder := request(t, server, http.MethodGet, "/active", nil)
	if activeRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected active status: %d", activeRecorder.Code)
	}
	var inactive activeResponse
	if err := json.Unmarshal(activeRecorder.Body.Bytes(), &inactive); err != nil {
		t.Fatalf("unmarshal active: %v", err)
	}
	if inactive.App != nil {
		t.Fatalf("expected no active app, got %+v", inactive.App)
	}

	postJSON(t, server, http.MethodPut, "/active", activateRequest{Target: "5174", Name: "alpha"}, http.StatusOK)

	activeRecorder = request(t, server, http.MethodGet, "/active", nil)
	var active activeResponse
	if err := json.Unmarshal(activeRecorder.Body.Bytes(), &active); err != nil {
		t.Fatalf("unmarshal active after activate: %v", err)
	}
	if active.App == nil || active.App.Name != "alpha" || active.App.Port != 5174 {
		t.Fatalf("unexpected active app: %+v", active.App)
	}

	deleteRecorder := request(t, server, http.MethodDelete, "/apps/alpha", nil)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected delete status: %d", deleteRecorder.Code)
	}
}

func TestServerRenameByOldName(t *testing.T) {
	server := NewServer("/tmp/dev-switchboard-test.sock", registry.New(5173), ServerOptions{})

	postJSON(t, server, http.MethodPost, "/apps", addRequest{Port: 5175}, http.StatusCreated)
	renameRecorder := request(t, server, http.MethodPut, "/rename", renameRequest{OldName: "5175", NewName: "my-app"})
	if renameRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected rename status: %d body=%s", renameRecorder.Code, renameRecorder.Body.String())
	}

	listRecorder := request(t, server, http.MethodGet, "/apps", nil)
	var listed listResponse
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if listed.Apps[0].Name != "my-app" {
		t.Fatalf("unexpected renamed app: %+v", listed.Apps[0])
	}
}

func TestServerActivatesExistingName(t *testing.T) {
	server := NewServer("/tmp/dev-switchboard-test.sock", registry.New(5173), ServerOptions{})

	postJSON(t, server, http.MethodPost, "/apps", addRequest{Port: 5175, Name: "my-app"}, http.StatusCreated)
	postJSON(t, server, http.MethodPut, "/active", activateRequest{Target: "my-app"}, http.StatusOK)

	activeRecorder := request(t, server, http.MethodGet, "/active", nil)
	var active activeResponse
	if err := json.Unmarshal(activeRecorder.Body.Bytes(), &active); err != nil {
		t.Fatalf("unmarshal active: %v", err)
	}
	if active.App == nil || active.App.Name != "my-app" || active.App.Port != 5175 {
		t.Fatalf("unexpected active app: %+v", active.App)
	}
}

func TestServerRejectsInvalidNames(t *testing.T) {
	server := NewServer("/tmp/dev-switchboard-test.sock", registry.New(5173), ServerOptions{})

	recorder := request(t, server, http.MethodDelete, "/apps/bad/name", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	recorder = request(t, server, http.MethodPut, "/rename", renameRequest{OldName: "5175", NewName: "bad/name"})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected rename status: %d", recorder.Code)
	}
}

func TestServerStatusAndShutdown(t *testing.T) {
	shutdownCalled := make(chan struct{}, 1)
	server := NewServer("/tmp/dev-switchboard-test.sock", registry.New(5173), ServerOptions{
		Status: func() StatusData {
			return StatusData{Running: true, PID: 123, AppCount: 2, ProxyListenAddrs: []string{"127.0.0.1:5173"}}
		},
		Shutdown: func() {
			shutdownCalled <- struct{}{}
		},
	})

	statusRecorder := request(t, server, http.MethodGet, "/status", nil)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", statusRecorder.Code)
	}
	var status StatusData
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}
	if !status.Running || status.PID != 123 || status.AppCount != 2 {
		t.Fatalf("unexpected status payload: %+v", status)
	}

	shutdownRecorder := request(t, server, http.MethodPost, "/shutdown", nil)
	if shutdownRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected shutdown status code: %d", shutdownRecorder.Code)
	}
	select {
	case <-shutdownCalled:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected shutdown callback to be invoked")
	}
}

func request(t *testing.T, server *Server, method string, target string, payload any) *httptest.ResponseRecorder {
	t.Helper()

	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		body = bytes.NewReader(encoded)
	}

	req := httptest.NewRequest(method, target, body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	server.server.Handler.ServeHTTP(recorder, req)
	return recorder
}

func postJSON(t *testing.T, server *Server, method string, target string, payload any, wantStatus int) {
	t.Helper()
	recorder := request(t, server, method, target, payload)
	if recorder.Code != wantStatus {
		t.Fatalf("unexpected status for %s %s: got %d want %d body=%s", method, target, recorder.Code, wantStatus, recorder.Body.String())
	}
}
