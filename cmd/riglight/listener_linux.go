//go:build linux

package main

import (
	"context"
	"net"
	"syscall"
)

type socketStringSetter func(int, int, int, string) error

func tailnetListen(ctx context.Context, network, address string) (net.Listener, error) {
	return tailnetListenWithSocketSetter(ctx, network, address, syscall.SetsockoptString)
}

func tailnetListenWithSocketSetter(ctx context.Context, network, address string, setSocketString socketStringSetter) (net.Listener, error) {
	configuration := net.ListenConfig{Control: tailnetControl(setSocketString)}
	return configuration.Listen(ctx, network, address)
}

func tailnetControl(setSocketString socketStringSetter) func(string, string, syscall.RawConn) error {
	return func(_, _ string, connection syscall.RawConn) error {
		var setError error
		if err := connection.Control(func(fd uintptr) {
			setError = setSocketString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, tailnetInterfaceName)
		}); err != nil {
			return err
		}
		return setError
	}
}
