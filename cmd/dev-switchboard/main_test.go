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
