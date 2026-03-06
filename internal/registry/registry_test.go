package registry

import (
	"errors"
	"testing"

	"dev-switchboard/internal/app"
)

func TestRegistryLifecycle(t *testing.T) {
	reg := New(5173)
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
	reg := New(5173)
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
	reg := New(5173)
	if err := reg.Add(app.App{Name: "bad", Port: 0}); !errors.Is(err, ErrInvalidPort) {
		t.Fatalf("expected invalid port error, got %v", err)
	}
	if err := reg.Add(app.App{Name: "reserved", Port: 5173}); !errors.Is(err, ErrReservedPort) {
		t.Fatalf("expected reserved port error, got %v", err)
	}
}

func TestRegistryValidatesNames(t *testing.T) {
	reg := New(5173)
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

func TestRegistryFindByPort(t *testing.T) {
	reg := New(5173)
	if err := reg.Add(app.App{Name: "5175", Port: 5175}); err != nil {
		t.Fatalf("add app: %v", err)
	}

	candidate, ok := reg.FindByPort(5175)
	if !ok {
		t.Fatal("expected to find app by port")
	}
	if candidate.Name != "5175" {
		t.Fatalf("unexpected app: %+v", candidate)
	}
}

func TestRegistryRenamePreservesActiveSelection(t *testing.T) {
	reg := New(5173)
	if err := reg.Add(app.App{Name: "5175", Port: 5175}); err != nil {
		t.Fatalf("add app: %v", err)
	}
	if _, err := reg.Activate("5175"); err != nil {
		t.Fatalf("activate app: %v", err)
	}

	renamed, err := reg.Rename("5175", "my-app")
	if err != nil {
		t.Fatalf("rename app: %v", err)
	}
	if renamed.Name != "my-app" || renamed.Port != 5175 {
		t.Fatalf("unexpected renamed app: %+v", renamed)
	}

	active, ok := reg.Active()
	if !ok || active.Name != "my-app" {
		t.Fatalf("unexpected active app after rename: %+v", active)
	}
}

func TestRegistryRenamePort(t *testing.T) {
	reg := New(5173)
	if err := reg.Add(app.App{Name: "existing", Port: 5175}); err != nil {
		t.Fatalf("add app: %v", err)
	}

	renamed, err := reg.RenamePort(5175, "renamed")
	if err != nil {
		t.Fatalf("rename port: %v", err)
	}
	if renamed.Name != "renamed" {
		t.Fatalf("unexpected renamed app: %+v", renamed)
	}
}

func TestRegistryReservesConfiguredPort(t *testing.T) {
	reg := New(6000)
	if err := reg.Add(app.App{Name: "reserved", Port: 6000}); !errors.Is(err, ErrReservedPort) {
		t.Fatalf("expected reserved port error, got %v", err)
	}
}
