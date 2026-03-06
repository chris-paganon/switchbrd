package control

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"dev-switchboard/internal/registry"
)

func TestServerHandlersLifecycle(t *testing.T) {
	server := NewServer("/tmp/dev-switchboard-test.sock", registry.New())

	postJSON(t, server, http.MethodPost, "/apps", addRequest{Name: "alpha", Port: 5174}, http.StatusCreated)

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

	postJSON(t, server, http.MethodPut, "/active", activateRequest{Name: "alpha"}, http.StatusOK)

	activeRecorder = request(t, server, http.MethodGet, "/active", nil)
	var active activeResponse
	if err := json.Unmarshal(activeRecorder.Body.Bytes(), &active); err != nil {
		t.Fatalf("unmarshal active after activate: %v", err)
	}
	if active.App == nil || active.App.Name != "alpha" {
		t.Fatalf("unexpected active app: %+v", active.App)
	}

	deleteRecorder := request(t, server, http.MethodDelete, "/apps/alpha", nil)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("unexpected delete status: %d", deleteRecorder.Code)
	}
}

func TestServerRejectsInvalidNames(t *testing.T) {
	server := NewServer("/tmp/dev-switchboard-test.sock", registry.New())
	recorder := request(t, server, http.MethodDelete, "/apps/bad/name", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
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
