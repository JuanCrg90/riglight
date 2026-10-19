//go:build linux

package main

import (
	"errors"
	"syscall"
	"testing"
)

func TestTailnetControlBindsSocketToTailscaleInterface(t *testing.T) {
	t.Parallel()

	var got struct {
		fd    int
		level int
		name  int
		value string
	}
	control := tailnetControl(func(fd, level, name int, value string) error {
		got.fd, got.level, got.name, got.value = fd, level, name, value
		return nil
	})
	if err := control("tcp", "100.64.0.10:9090", fakeRawConn{fd: 42}); err != nil {
		t.Fatalf("control: %v", err)
	}
	if got.fd != 42 || got.level != syscall.SOL_SOCKET || got.name != syscall.SO_BINDTODEVICE || got.value != tailnetInterfaceName {
		t.Errorf("setsockopt = %#v", got)
	}
}

func TestTailnetControlFailsClosed(t *testing.T) {
	t.Parallel()

	setError := errors.New("setsockopt denied")
	if err := tailnetControl(func(int, int, int, string) error { return setError })("tcp", "100.64.0.10:9090", fakeRawConn{}); !errors.Is(err, setError) {
		t.Errorf("setsockopt error = %v, want %v", err, setError)
	}
	controlError := errors.New("raw control failed")
	if err := tailnetControl(func(int, int, int, string) error { return nil })("tcp", "100.64.0.10:9090", fakeRawConn{controlError: controlError}); !errors.Is(err, controlError) {
		t.Errorf("control error = %v, want %v", err, controlError)
	}
}

type fakeRawConn struct {
	fd           uintptr
	controlError error
}

func (connection fakeRawConn) Control(function func(uintptr)) error {
	if connection.controlError != nil {
		return connection.controlError
	}
	function(connection.fd)
	return nil
}

func (fakeRawConn) Read(func(uintptr) bool) error {
	return nil
}

func (fakeRawConn) Write(func(uintptr) bool) error {
	return nil
}
