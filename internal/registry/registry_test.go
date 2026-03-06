package registry

import (
	"errors"
	"testing"

	"dev-switchboard/internal/app"
)

func TestRegistryLifecycle(t *testing.T) {
	reg := New()
	if err := reg.Add(app.App{Name: "alpha", Port: 5174}); err != nil {
		t.Fatalf("add alpha: %v", err)
	}
	if err := reg.Add(app.App{Name: "beta", Port: 5175}); err != nil {
		t.Fatalf("add beta: %v", err)
	}

	active, ok := reg.Active()
	if ok {
		t.Fatalf("expected no active app, got %+v", active)
	}

	activated, err := reg.Activate("beta")
	if err != nil {
		t.Fatalf("activate beta: %v", err)
	}
	if activated.Name != "beta" {
		t.Fatalf("unexpected active app: %+v", activated)
	}

	if err := reg.Remove("beta"); err != nil {
		t.Fatalf("remove beta: %v", err)
	}
	if _, ok := reg.Active(); ok {
		t.Fatal("expected active app to be cleared")
	}
}

func TestRegistryRejectsDuplicates(t *testing.T) {
	reg := New()
	if err := reg.Add(app.App{Name: "alpha", Port: 5174}); err != nil {
		t.Fatalf("add alpha: %v", err)
	}
	if err := reg.Add(app.App{Name: "alpha", Port: 5175}); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("expected duplicate name error, got %v", err)
	}
	if err := reg.Add(app.App{Name: "beta", Port: 5174}); !errors.Is(err, ErrDuplicatePort) {
		t.Fatalf("expected duplicate port error, got %v", err)
	}
}

func TestRegistryValidatesPorts(t *testing.T) {
	reg := New()
	if err := reg.Add(app.App{Name: "bad", Port: 0}); !errors.Is(err, ErrInvalidPort) {
		t.Fatalf("expected invalid port error, got %v", err)
	}
	if err := reg.Add(app.App{Name: "reserved", Port: 5173}); !errors.Is(err, ErrReservedPort) {
		t.Fatalf("expected reserved port error, got %v", err)
	}
}

func TestRegistryValidatesNames(t *testing.T) {
	reg := New()
	if err := reg.Add(app.App{Name: "", Port: 5174}); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("expected invalid name error, got %v", err)
	}
	if err := reg.Add(app.App{Name: "bad/name", Port: 5175}); !errors.Is(err, ErrInvalidName) {
		t.Fatalf("expected invalid name error, got %v", err)
	}
	if err := reg.Add(app.App{Name: "  alpha  ", Port: 5176}); err != nil {
		t.Fatalf("expected trimmed name to be accepted, got %v", err)
	}
	apps := reg.List()
	if len(apps) != 1 || apps[0].Name != "alpha" {
		t.Fatalf("unexpected apps: %+v", apps)
	}
}
