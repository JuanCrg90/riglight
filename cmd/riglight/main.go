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
	"runtime"
	"strconv"
	"syscall"
	"time"

	"riglight/internal/metrics"
	"riglight/internal/server"
	"riglight/web"
)

const (
	defaultListen = "127.0.0.1:9090"
	devOrigin     = "http://127.0.0.1:5173"
)

var (
	errInvalidArguments = errors.New("invalid arguments")
	errUnableToStart    = errors.New("unable to start")
)

type config struct {
	listen string
	demo   bool
	dev    bool
}

type publisher interface {
	Publish(context.Context, time.Time) error
}

type livePublisherFactory func(*metrics.Hub) (publisher, error)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], newLivePublisher); err != nil {
		_, _ = io.WriteString(os.Stderr, "riglight: unable to start\n")
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string, newLive livePublisherFactory) error {
	return runForOS(ctx, arguments, newLive, runtime.GOOS)
}

func runForOS(ctx context.Context, arguments []string, newLive livePublisherFactory, goos string) error {
	config, err := parseConfig(arguments)
	if err != nil {
		return errInvalidArguments
	}
	if !config.demo && goos != "linux" {
		return errUnableToStart
	}

	now := time.Now()
	hub, err := metrics.NewHub(initialSample(now, config.demo))
	if err != nil {
		return errUnableToStart
	}
	defer hub.Close()
	publisher, err := selectPublisher(config.demo, hub, newLive)
	if err != nil {
		return errUnableToStart
	}
	listener, err := net.Listen("tcp", config.listen)
	if err != nil {
		return errUnableToStart
	}

	application, cancelApplication := context.WithCancel(ctx)
	defer cancelApplication()
	origins := []string(nil)
	if config.dev {
		origins = []string{devOrigin}
	}
	handler, err := server.New(server.Config{
		Hub:               hub,
		AllowedHost:       config.listen,
		AllowedOrigin:     "http://" + config.listen,
		AdditionalOrigins: origins,
		Context:           application,
		Assets:            web.Assets,
	})
	if err != nil {
		_ = listener.Close()
		return errUnableToStart
	}
	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    8 << 10,
	}
	defer func() {
		cancelApplication()
		shutdownServer(httpServer)
	}()
	serveError := make(chan error, 1)
	go func() { serveError <- httpServer.Serve(listener) }()

	if err := publisher.Publish(application, time.Now()); err != nil {
		return errUnableToStart
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-serveError:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return errUnableToStart
		case tick := <-ticker.C:
			if err := publisher.Publish(application, tick); err != nil {
				return errUnableToStart
			}
		}
	}
}

func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func selectPublisher(demo bool, hub *metrics.Hub, newLive livePublisherFactory) (publisher, error) {
	if demo {
		return metrics.NewDemo(hub), nil
	}
	return newLive(hub)
}

func newLivePublisher(hub *metrics.Hub) (publisher, error) {
	return metrics.NewLive(hub), nil
}

func initialSample(at time.Time, demo bool) metrics.Sample {
	mode := metrics.ModeLive
	if demo {
		mode = metrics.ModeDemo
	}
	return metrics.Sample{
		SchemaVersion: metrics.SchemaVersion,
		SampledAt:     at,
		Mode:          mode,
		CPU:           metrics.CPU{Status: metrics.StatusNotReady},
		Memory:        metrics.Memory{Status: metrics.StatusNotReady},
	}
}

func parseConfig(arguments []string) (config, error) {
	flags := flag.NewFlagSet("riglight", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	listen := flags.String("listen", defaultListen, "")
	demo := flags.Bool("demo", false, "")
	dev := flags.Bool("dev", false, "")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return config{}, errInvalidArguments
	}
	canonicalListen, ok := canonicalLoopbackListen(*listen)
	if !ok {
		return config{}, errInvalidArguments
	}
	return config{listen: canonicalListen, demo: *demo, dev: *dev}, nil
}

func canonicalLoopbackListen(address string) (string, bool) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", false
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return "", false
	}
	value, err := strconv.Atoi(port)
	if err != nil || value <= 0 || value > 65535 {
		return "", false
	}
	return net.JoinHostPort(ip.String(), strconv.Itoa(value)), true
}
