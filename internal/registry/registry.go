package registry

import (
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"

	"switchbrd/internal/app"
)

var (
	ErrAppNotFound   = errors.New("app not found")
	ErrNoActiveApp   = errors.New("no active app")
	ErrDuplicateName = errors.New("app name already exists")
	ErrDuplicatePort = errors.New("app port already exists")
	ErrReservedPort  = errors.New("port is reserved for switchbrd")
	ErrInvalidPort   = errors.New("invalid port")
	ErrInvalidName   = errors.New("invalid app name")
)

var appNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type Registry struct {
	mu           sync.RWMutex
	apps         map[string]app.App
	activeName   string
	reservedPort int
}

func New(reservedPort int) *Registry {
	return &Registry{
		apps:         make(map[string]app.App),
		reservedPort: reservedPort,
	}
}

func (r *Registry) Add(candidate app.App) error {
	candidate.Name = strings.TrimSpace(candidate.Name)
	if candidate.Port < 1 || candidate.Port > 65535 {
		return ErrInvalidPort
	}
	if candidate.Port == r.reservedPort {
		return ErrReservedPort
	}
	if !appNamePattern.MatchString(candidate.Name) {
		return ErrInvalidName
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.apps[candidate.Name]; exists {
		return ErrDuplicateName
	}
	for _, existing := range r.apps {
		if existing.Port == candidate.Port {
			return ErrDuplicatePort
		}
	}

	r.apps[candidate.Name] = candidate
	return nil
}

func (r *Registry) Remove(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.apps[name]; !exists {
		return ErrAppNotFound
	}
	delete(r.apps, name)
	if r.activeName == name {
		r.activeName = ""
	}
	return nil
}

func (r *Registry) Activate(name string) (app.App, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	candidate, exists := r.apps[name]
	if !exists {
		return app.App{}, ErrAppNotFound
	}
	r.activeName = name
	return candidate, nil
}

func (r *Registry) FindByPort(port int) (app.App, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, candidate := range r.apps {
		if candidate.Port == port {
			return candidate, true
		}
	}

	return app.App{}, false
}

func (r *Registry) Rename(oldName string, newName string) (app.App, error) {
	newName = strings.TrimSpace(newName)
	if !appNamePattern.MatchString(newName) {
		return app.App{}, ErrInvalidName
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	candidate, exists := r.apps[oldName]
	if !exists {
		return app.App{}, ErrAppNotFound
	}
	if oldName != newName {
		if _, exists := r.apps[newName]; exists {
			return app.App{}, ErrDuplicateName
		}
		delete(r.apps, oldName)
	}

	candidate.Name = newName
	r.apps[newName] = candidate
	if r.activeName == oldName {
		r.activeName = newName
	}

	return candidate, nil
}

func (r *Registry) RenamePort(port int, newName string) (app.App, error) {
	candidate, ok := r.FindByPort(port)
	if !ok {
		return app.App{}, ErrAppNotFound
	}
	return r.Rename(candidate.Name, newName)
}

func (r *Registry) Active() (app.App, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.activeName == "" {
		return app.App{}, false
	}
	candidate, exists := r.apps[r.activeName]
	if !exists {
		return app.App{}, false
	}
	return candidate, true
}

func (r *Registry) List() []app.App {
	r.mu.RLock()
	defer r.mu.RUnlock()

	apps := make([]app.App, 0, len(r.apps))
	for _, candidate := range r.apps {
		apps = append(apps, candidate)
	}
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].Name < apps[j].Name
	})
	return apps
}
