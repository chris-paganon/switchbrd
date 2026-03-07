package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"dev-switchboard/internal/control"
)

func TestRunWithoutArgsDefaultsToTUI(t *testing.T) {
	originalEnsure := ensureServerRunningFunc
	originalRunTUI := runTUIFunc
	t.Cleanup(func() {
		ensureServerRunningFunc = originalEnsure
		runTUIFunc = originalRunTUI
	})

	calledEnsure := false
	ensureServerRunningFunc = func(ctx context.Context, client *control.Client, port int) (bool, error) {
		calledEnsure = true
		if port != defaultProxyPort {
			t.Fatalf("unexpected port: %d", port)
		}
		return false, nil
	}

	calledTUI := false
	runTUIFunc = func(client *control.Client, ownedServer bool) error {
		calledTUI = true
		if ownedServer {
			t.Fatalf("expected ownedServer false")
		}
		return nil
	}

	if err := run(context.Background(), nil, &bytes.Buffer{}); err != nil {
		t.Fatalf("run without args: %v", err)
	}
	if !calledEnsure {
		t.Fatal("expected run without args to ensure the server is running")
	}
	if !calledTUI {
		t.Fatal("expected run without args to launch the TUI")
	}
}

func TestRunHelpFlagPrintsHelp(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"-h"}} {
		var stdout bytes.Buffer

		if err := run(context.Background(), args, &stdout); err != nil {
			t.Fatalf("run %v: %v", args, err)
		}

		output := stdout.String()
		if !strings.Contains(output, "Options:") {
			t.Fatalf("expected options in help output for %v, got %q", args, output)
		}
		if !strings.Contains(output, "Examples:") {
			t.Fatalf("expected examples in help output for %v, got %q", args, output)
		}
	}
}

func TestParseServerCommandDefaultsPort(t *testing.T) {
	port, err := parseServerCommand(nil)
	if err != nil {
		t.Fatalf("parse server command: %v", err)
	}
	if port != defaultProxyPort {
		t.Fatalf("unexpected default port: %d", port)
	}
}

func TestParseServerCommandAcceptsPortFlag(t *testing.T) {
	port, err := parseServerCommand([]string{"--port", "6000"})
	if err != nil {
		t.Fatalf("parse server command: %v", err)
	}
	if port != 6000 {
		t.Fatalf("unexpected port: %d", port)
	}
}

func TestParseServerCommandAcceptsShortPortFlag(t *testing.T) {
	port, err := parseServerCommand([]string{"-p", "6000"})
	if err != nil {
		t.Fatalf("parse server command: %v", err)
	}
	if port != 6000 {
		t.Fatalf("unexpected port: %d", port)
	}
}

func TestParseActivateCommandForPort(t *testing.T) {
	target, name, err := parseActivateCommand([]string{"5175", "--name", "my-app"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if target != "5175" || name != "my-app" {
		t.Fatalf("unexpected parse result: target=%q name=%q", target, name)
	}
}

func TestParseActivateCommandForName(t *testing.T) {
	target, name, err := parseActivateCommand([]string{"my-app"})
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if target != "my-app" || name != "" {
		t.Fatalf("unexpected parse result: target=%q name=%q", target, name)
	}
}

func TestParseRenameCommand(t *testing.T) {
	oldName, newName, err := parseRenameCommand([]string{"5175", "my-app"})
	if err != nil {
		t.Fatalf("parse rename: %v", err)
	}
	if oldName != "5175" || newName != "my-app" {
		t.Fatalf("unexpected rename parse result: old=%q new=%q", oldName, newName)
	}
}

func TestProxyListenAddrs(t *testing.T) {
	addrs := proxyListenAddrs(6000)
	if len(addrs) != 2 {
		t.Fatalf("unexpected addr count: %d", len(addrs))
	}
	if addrs[0] != "127.0.0.1:6000" || addrs[1] != "[::1]:6000" {
		t.Fatalf("unexpected addrs: %#v", addrs)
	}
}
