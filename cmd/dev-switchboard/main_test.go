package main

import "testing"

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
