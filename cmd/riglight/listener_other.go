//go:build !linux

package main

import (
	"context"
	"errors"
	"net"
)

func tailnetListen(context.Context, string, string) (net.Listener, error) {
	return nil, errors.New("tailnet interface binding is unavailable")
}
