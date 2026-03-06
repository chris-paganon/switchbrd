package main

import (
	"errors"
	"testing"
)

func TestIsOptionalIPv6Loopback(t *testing.T) {
	if !isOptionalIPv6Loopback("[::1]:5173", errors.New("listen tcp [::1]:5173: socket: address family not supported by protocol")) {
		t.Fatal("expected IPv6 loopback bind failure to be optional")
	}
	if isOptionalIPv6Loopback("127.0.0.1:5173", errors.New("listen tcp 127.0.0.1:5173: bind: address already in use")) {
		t.Fatal("expected IPv4 bind failure to remain fatal")
	}
	if isOptionalIPv6Loopback("[::1]:5173", errors.New("listen tcp [::1]:5173: bind: address already in use")) {
		t.Fatal("expected IPv6 port-in-use failure to remain fatal")
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
