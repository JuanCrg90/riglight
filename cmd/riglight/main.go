package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"riglight/internal/containers"
	"riglight/internal/gpu"
	"riglight/internal/llamaswap"
	"riglight/internal/metrics"
	"riglight/internal/server"
	"riglight/internal/systemmetrics"
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
	listen            string
	demo              bool
	dev               bool
	gpu               bool
	llamaSwap         bool
	llamaSwapURL      string
	docker            bool
	dockerProxyURL    string
	systemDetails     bool
	networkInterfaces []string
	tailnet           bool
	host              string
	listenIP          netip.Addr
	allowedClients    []netip.Addr
	allowAllTailnet   bool
}

type publisher interface {
	Publish(context.Context, time.Time) error
}

type livePublisherFactory func(*metrics.Hub) (publisher, error)

type tailnetListenerFactory func(context.Context, string, string) (net.Listener, error)

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
	return runWithAllFactoriesAndTailnetLookup(ctx, arguments, newLive, newNative, newLlamaSwap, goos, lookupTailnetInterface)
}

func runWithAllFactoriesAndTailnetLookup(ctx context.Context, arguments []string, newLive livePublisherFactory, newNative nativeGPUPollerFactory, newLlamaSwap llamaSwapSourceFactory, goos string, lookup func() (tailnetInterfaceState, error)) error {
	return runWithAllFactoriesAndTailnetListener(ctx, arguments, newLive, newNative, newLlamaSwap, goos, lookup, tailnetListen)
}

func runWithAllFactoriesAndTailnetListener(ctx context.Context, arguments []string, newLive livePublisherFactory, newNative nativeGPUPollerFactory, newLlamaSwap llamaSwapSourceFactory, goos string, lookup func() (tailnetInterfaceState, error), newTailnetListener tailnetListenerFactory) error {
	config, err := parseConfig(arguments)
	if err != nil {
		return errInvalidArguments
	}
	if (!config.demo || config.tailnet) && goos != "linux" {
		return errUnableToStart
	}
	if config.tailnet {
		state, err := lookup()
		if err != nil || !tailnetInterfaceHasAddress(config.listenIP, state) {
			return errUnableToStart
		}
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
	containerHub, err := containers.NewHub(initialContainerSample(now, config))
	if err != nil {
		return errUnableToStart
	}
	defer containerHub.Close()
	systemHub, err := systemmetrics.NewHub(initialSystemSample(now, config))
	if err != nil {
		return errUnableToStart
	}
	defer systemHub.Close()
	publisher, err := selectPublisher(config.demo, hub, newLive)
	if err != nil {
		return errUnableToStart
	}
	var listener net.Listener
	if config.tailnet {
		listener, err = newTailnetListener(ctx, "tcp", config.listen)
	} else {
		listener, err = net.Listen("tcp", config.listen)
	}
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
	containerWorker, err := selectContainerWorker(containerHub, config, defaultContainerSource)
	if err != nil {
		cancelApplication()
		_ = listener.Close()
		return errUnableToStart
	}
	systemWorker, err := selectSystemWorker(systemHub, config, defaultSystemPublisher)
	if err != nil {
		cancelApplication()
		_ = listener.Close()
		return errUnableToStart
	}
	origins := []string(nil)
	allowedHost := config.listen
	if config.tailnet {
		allowedHost = config.host
	} else if config.dev {
		origins = []string{devOrigin}
	}
	handler, err := server.New(server.Config{
		Hub:               hub,
		GPUHub:            gpuHub,
		ModelsHub:         llamaSwapHub,
		ContainersHub:     containerHub,
		AllowedHost:       allowedHost,
		AllowedOrigin:     "http://" + allowedHost,
		AdditionalOrigins: origins,
		AllowedClients:    config.allowedClients,
		AllowAllTailnet:   config.allowAllTailnet,
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
	defer func() {
		shutdownApplication(cancelApplication, httpServer, gpuWorker, llamaSwapWorker, containerWorker, systemWorker)
	}()
	serveError := make(chan error, 1)
	go func() { serveError <- httpServer.Serve(listener) }()
	if gpuWorker != nil {
		gpuWorker.Start(application)
	}
	if llamaSwapWorker != nil {
		llamaSwapWorker.Start(application)
	}
	if containerWorker != nil {
		containerWorker.Start(application)
	}
	if systemWorker != nil {
		systemWorker.Start(application)
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

func shutdownApplication(cancelApplication context.CancelFunc, server *http.Server, gpuWorker *gpuWorker, llamaSwapWorker *llamaSwapWorker, containerWorker *containerWorker, systemWorker *systemWorker) {
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
	if containerWorker != nil {
		containerWorker.Wait(ctx)
	}
	if systemWorker != nil {
		systemWorker.Wait(ctx)
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
	docker := flags.Bool("docker", false, "")
	dockerProxyURL := flags.String("docker-proxy-url", "", "")
	systemDetails := flags.Bool("system-details", false, "")
	var networkInterfaces stringValues
	flags.Var(&networkInterfaces, "network-interface", "")
	tailnet := flags.Bool("tailnet", false, "")
	host := flags.String("host", "", "")
	allowAllTailnet := flags.Bool("allow-all-tailnet", false, "")
	var allowedClientValues stringValues
	flags.Var(&allowedClientValues, "allow-client", "")
	if err := flags.Parse(arguments); err != nil || flags.NArg() != 0 {
		return config{}, errInvalidArguments
	}
	listenSet, hostSet, allowClientSet, allowAllTailnetSet, urlSet, dockerURLSet := false, false, false, false, false, false
	flags.Visit(func(flag *flag.Flag) {
		switch flag.Name {
		case "listen":
			listenSet = true
		case "host":
			hostSet = true
		case "allow-client":
			allowClientSet = true
		case "allow-all-tailnet":
			allowAllTailnetSet = true
		case "llama-swap-url":
			urlSet = true
		case "docker-proxy-url":
			dockerURLSet = true
		}
	})
	if (*tailnet && (*dev || !listenSet || !hostSet || (*allowAllTailnet == allowClientSet))) || (!*tailnet && (hostSet || allowClientSet || allowAllTailnetSet)) {
		return config{}, errInvalidArguments
	}
	parsed := config{demo: *demo, dev: *dev, gpu: *gpu, llamaSwap: *llamaSwap, llamaSwapURL: *llamaSwapURL, docker: *docker, dockerProxyURL: *dockerProxyURL, systemDetails: *systemDetails, tailnet: *tailnet, allowAllTailnet: *allowAllTailnet}
	if len(networkInterfaces) > 0 && !*systemDetails {
		return config{}, errInvalidArguments
	}
	if *systemDetails {
		if !*demo && len(networkInterfaces) == 0 {
			return config{}, errInvalidArguments
		}
		if len(networkInterfaces) > 0 {
			selection, selectionErr := systemmetrics.ValidateInterfaces(networkInterfaces)
			if selectionErr != nil {
				return config{}, errInvalidArguments
			}
			parsed.networkInterfaces = selection
		}
	}
	if *tailnet {
		canonicalListen, listenIP, ok := canonicalTailnetListen(*listen)
		if !ok {
			return config{}, errInvalidArguments
		}
		_, port, _ := net.SplitHostPort(canonicalListen)
		canonicalHost, ok := canonicalTailnetHost(*host, port)
		if !ok {
			return config{}, errInvalidArguments
		}
		for _, value := range allowedClientValues {
			client, ok := canonicalTailnetAddr(value)
			if !ok {
				return config{}, errInvalidArguments
			}
			parsed.allowedClients = append(parsed.allowedClients, client)
		}
		parsed.listen, parsed.listenIP, parsed.host = canonicalListen, listenIP, canonicalHost
	} else {
		canonicalListen, ok := canonicalLoopbackListen(*listen)
		if !ok {
			return config{}, errInvalidArguments
		}
		parsed.listen = canonicalListen
	}
	if urlSet && !*llamaSwap {
		return config{}, errInvalidArguments
	}
	if *llamaSwap && !*demo {
		if _, err := llamaswap.CanonicalURL(*llamaSwapURL); err != nil {
			return config{}, errInvalidArguments
		}
	}
	if dockerURLSet && !*docker {
		return config{}, errInvalidArguments
	}
	if *docker && !*demo {
		if !dockerURLSet {
			return config{}, errInvalidArguments
		}
		if _, err := containers.CanonicalURL(*dockerProxyURL); err != nil {
			return config{}, errInvalidArguments
		}
	}
	return parsed, nil
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
