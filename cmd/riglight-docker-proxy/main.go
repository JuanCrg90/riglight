package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"riglight/internal/containers"
	"riglight/internal/dockerproxy"
)

type stringValues []string

func (s *stringValues) String() string         { return "" }
func (s *stringValues) Set(value string) error { *s = append(*s, value); return nil }
func parseConfig(args []string) (dockerproxy.Config, error) {
	flags := flag.NewFlagSet("riglight-docker-proxy", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	socket := flags.String("socket", "", "")
	listen := flags.String("listen", "127.0.0.1:9123", "")
	var selections stringValues
	flags.Var(&selections, "container", "")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *socket == "" {
		return dockerproxy.Config{}, errors.New("invalid arguments")
	}
	return dockerproxy.NewConfig(*socket, *listen, selections)
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		_, _ = io.WriteString(os.Stderr, "riglight-docker-proxy: unable to start\n")
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string) error {
	config, err := parseConfig(args)
	if err != nil {
		return err
	}
	collector, err := dockerproxy.NewCollector(config)
	if err != nil {
		return err
	}
	defer collector.CloseIdleConnections()
	hub, err := containers.NewHub(containers.NotReadySample(time.Now(), containers.ModeLive))
	if err != nil {
		return err
	}
	defer hub.Close()
	handler, err := dockerproxy.NewHTTPServer(hub, config.Listen, nil)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", config.Listen)
	if err != nil {
		return err
	}
	application, cancel := context.WithCancel(ctx)
	worker := dockerproxy.StartWorker(hub, collector, application)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: time.Minute, MaxHeaderBytes: 8 << 10}
	serve := make(chan error, 1)
	go func() { serve <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
	case err := <-serve:
		cancel()
		_ = listener.Close()
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
	cancel()
	deadline, stopShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopShutdown()
	_ = server.Shutdown(deadline)
	worker.Wait(deadline)
	return nil
}
