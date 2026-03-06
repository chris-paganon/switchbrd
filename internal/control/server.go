package control

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"dev-switchboard/internal/app"
	"dev-switchboard/internal/registry"
)

type Server struct {
	registry   *registry.Registry
	server     *http.Server
	socket     string
	statusFn   func() StatusData
	shutdownFn func()
}

type ServerOptions struct {
	Status   func() StatusData
	Shutdown func()
}

func NewServer(socket string, reg *registry.Registry, options ServerOptions) *Server {
	s := &Server{
		registry:   reg,
		socket:     socket,
		statusFn:   options.Status,
		shutdownFn: options.Shutdown,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/shutdown", s.handleShutdown)
	mux.HandleFunc("/apps", s.handleApps)
	mux.HandleFunc("/active", s.handleActive)
	mux.HandleFunc("/rename", s.handleRename)
	mux.HandleFunc("/apps/", s.handleNamedApps)
	s.server = &http.Server{Handler: mux}
	return s
}

func (s *Server) Start() error {
	if err := os.Remove(s.socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	listener, err := net.Listen("unix", s.socket)
	if err != nil {
		return err
	}
	if err := os.Chmod(s.socket, 0o600); err != nil {
		_ = listener.Close()
		return err
	}
	go func() {
		_ = s.server.Serve(listener)
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	err := s.server.Shutdown(ctx)
	removeErr := os.Remove(s.socket)
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) && err == nil {
		return removeErr
	}
	return err
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	status := StatusData{
		Running:  true,
		PID:      os.Getpid(),
		AppCount: len(s.registry.List()),
	}
	if active, ok := s.registry.Active(); ok {
		status.Active = &active
	}
	if s.statusFn != nil {
		status = s.statusFn()
	}

	respondJSON(w, http.StatusOK, status)
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"status": "shutting down"})
	if s.shutdownFn != nil {
		go s.shutdownFn()
	}
}

func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		active, _ := s.registry.Active()
		activeName := active.Name
		respondJSON(w, http.StatusOK, listResponse{Apps: s.registry.List(), ActiveName: activeName})
	case http.MethodPost:
		var req addRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		candidate := app.App{Name: req.Name, Port: req.Port}
		if candidate.Name == "" {
			candidate.Name = strconv.Itoa(candidate.Port)
		}
		if err := s.registry.Add(candidate); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSON(w, http.StatusCreated, appResponse{App: candidate})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleActive(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		candidate, ok := s.registry.Active()
		if !ok {
			respondJSON(w, http.StatusOK, activeResponse{})
			return
		}
		respondJSON(w, http.StatusOK, activeResponse{App: &candidate})
	case http.MethodPut:
		var req activateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Target == "" {
			respondError(w, http.StatusBadRequest, "activation target is required")
			return
		}

		port, err := strconv.Atoi(req.Target)
		if err == nil {
			candidate, ok := s.registry.FindByPort(port)
			if ok {
				if req.Name != "" && req.Name != candidate.Name {
					candidate, err = s.registry.Rename(candidate.Name, req.Name)
					if err != nil {
						respondError(w, http.StatusBadRequest, err.Error())
						return
					}
				}
			} else {
				candidate = app.App{Name: req.Name, Port: port}
				if candidate.Name == "" {
					candidate.Name = strconv.Itoa(candidate.Port)
				}
				if err := s.registry.Add(candidate); err != nil {
					respondError(w, http.StatusBadRequest, err.Error())
					return
				}
			}

			candidate, err = s.registry.Activate(candidate.Name)
			if err != nil {
				respondError(w, http.StatusBadRequest, err.Error())
				return
			}
			respondJSON(w, http.StatusOK, activeResponse{App: &candidate})
			return
		}

		if req.Name != "" {
			respondError(w, http.StatusBadRequest, "--name can only be used when activating a port")
			return
		}

		candidate, err := s.registry.Activate(req.Target)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, activeResponse{App: &candidate})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req renameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	candidate, err := s.registry.Rename(req.OldName, req.NewName)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, appResponse{App: candidate})
}

func (s *Server) handleNamedApps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/apps/")
	if name == "" || strings.Contains(name, "/") {
		respondError(w, http.StatusBadRequest, "invalid app name")
		return
	}
	if err := s.registry.Remove(name); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, errorResponse{Error: msg})
}
