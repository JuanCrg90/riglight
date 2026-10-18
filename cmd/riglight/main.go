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

	"riglight/internal/gpu"
	"riglight/internal/llamaswap"
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
	listen       string
	demo         bool
	dev          bool
	gpu          bool
	llamaSwap    bool
	llamaSwapURL string
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
	return runWithFactories(ctx, arguments, newLive, defaultNativeGPUPoller, runtime.GOOS)
}

func runForOS(ctx context.Context, arguments []string, newLive livePublisherFactory, goos string) error {
	return runWithFactories(ctx, arguments, newLive, defaultNativeGPUPoller, goos)
}

func runWithFactories(ctx context.Context, arguments []string, newLive livePublisherFactory, newNative nativeGPUPollerFactory, goos string) error {
	return runWithAllFactories(ctx, arguments, newLive, newNative, defaultLlamaSwapSource, goos)
}

func runWithAllFactories(ctx context.Context, arguments []string, newLive livePublisherFactory, newNative nativeGPUPollerFactory, newLlamaSwap llamaSwapSourceFactory, goos string) error {
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
	gpuHub, err := gpu.NewHub(initialGPUSample(now, config.demo, config.gpu))
	if err != nil {
		return errUnableToStart
	}
	defer gpuHub.Close()
	llamaSwapHub, err := llamaswap.NewHub(initialLlamaSwapSample(now, config))
	if err != nil {
		return errUnableToStart
	}
	defer llamaSwapHub.Close()
	publisher, err := selectPublisher(config.demo, hub, newLive)
	if err != nil {
		return errUnableToStart
	}
	listener, err := net.Listen("tcp", config.listen)
	if err != nil {
		return errUnableToStart
	}

	application, cancelApplication := context.WithCancel(ctx)
	gpuWorker := selectGPUWorkerWithFactory(gpuHub, config.demo, config.gpu, newNative)
	llamaSwapWorker, err := selectLlamaSwapWorker(llamaSwapHub, config, newLlamaSwap)
	if err != nil {
		cancelApplication()
		_ = listener.Close()
		return errUnableToStart
	}
	origins := []string(nil)
	if config.dev {
		origins = []string{devOrigin}
	}
	handler, err := server.New(server.Config{
		Hub:               hub,
		GPUHub:            gpuHub,
		ModelsHub:         llamaSwapHub,
		AllowedHost:       config.listen,
		AllowedOrigin:     "http://" + config.listen,
		AdditionalOrigins: origins,
		Context:           application,
		Assets:            web.Assets,
	})
	if err != nil {
		cancelApplication()
		_ = listener.Close()
		return errUnableToStart
	}
	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       time.Minute,
		MaxHeaderBytes:    8 << 10,
	}
	defer func() { shutdownApplication(cancelApplication, httpServer, gpuWorker, llamaSwapWorker) }()
	serveError := make(chan error, 1)
	go func() { serveError <- httpServer.Serve(listener) }()
	if gpuWorker != nil {
		gpuWorker.Start(application)
	}
	if llamaSwapWorker != nil {
		llamaSwapWorker.Start(application)
	}

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

func shutdownApplication(cancelApplication context.CancelFunc, server *http.Server, gpuWorker *gpuWorker, llamaSwapWorker *llamaSwapWorker) {
	// One deadline covers HTTP stream cancellation, HTTP shutdown and normal GPU
	// cleanup. A blocked native call is left loaded rather than racing unload.
	cancelApplication()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	if gpuWorker != nil {
		gpuWorker.Wait(ctx)
	}
	if llamaSwapWorker != nil {
		llamaSwapWorker.Wait(ctx)
	}
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
	gpu := flags.Bool("gpu", false, "")
	llamaSwap := flags.Bool("llama-swap", false, "")
	llamaSwapURL := flags.String("llama-swap-url", defaultLlamaSwapURL, "")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return config{}, errInvalidArguments
	}
	canonicalListen, ok := canonicalLoopbackListen(*listen)
	if !ok {
		return config{}, errInvalidArguments
	}
	urlSet := false
	flags.Visit(func(flag *flag.Flag) {
		urlSet = urlSet || flag.Name == "llama-swap-url"
	})
	if urlSet && !*llamaSwap {
		return config{}, errInvalidArguments
	}
	if *llamaSwap && !*demo {
		if _, err := llamaswap.CanonicalURL(*llamaSwapURL); err != nil {
			return config{}, errInvalidArguments
		}
	}
	return config{listen: canonicalListen, demo: *demo, dev: *dev, gpu: *gpu, llamaSwap: *llamaSwap, llamaSwapURL: *llamaSwapURL}, nil
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
