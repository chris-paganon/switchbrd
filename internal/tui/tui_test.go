package tui

import (
	"testing"

	"switchbrd/internal/app"
)

func TestSortedAppsOrdersByPort(t *testing.T) {
	apps := []app.App{
		{Name: "charlie", Port: 5180},
		{Name: "alpha", Port: 5174},
		{Name: "bravo", Port: 5176},
	}

	sorted := sortedApps(apps)

	if len(sorted) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(sorted))
	}
	if sorted[0].Port != 5174 || sorted[1].Port != 5176 || sorted[2].Port != 5180 {
		t.Fatalf("expected apps sorted by port, got %+v", sorted)
	}
}
