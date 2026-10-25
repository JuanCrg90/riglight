package relisten

import (
	"context"
	"net"
	"net/http"
	"time"
)

const pollInterval = 2 * time.Second

type Readiness func() (bool, error)
type ListenerFactory func(context.Context) (net.Listener, error)
type Logger func(string)
type serverStarter func(*http.Server, net.Listener, chan<- error)

// Run owns serving and, when readiness is provided, listener rebinding.
func Run(ctx context.Context, server *http.Server, listener net.Listener, ready Readiness, bind ListenerFactory, log Logger) error {
	return run(ctx, server, listener, ready, bind, log, startServer)
}

func run(ctx context.Context, server *http.Server, listener net.Listener, ready Readiness, bind ListenerFactory, log Logger, start serverStarter) error {
	if ready == nil {
		return serveOnce(ctx, server, listener, start)
	}
	currentServer, currentListener := server, listener
	for {
		if ctx.Err() != nil {
			_ = currentListener.Close()
			return http.ErrServerClosed
		}
		serveErr := make(chan error, 1)
		start(currentServer, currentListener, serveErr)
		for {
			timer := time.NewTimer(pollInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				shutdown(currentServer)
				return http.ErrServerClosed
			case err := <-serveErr:
				timer.Stop()
				return err
			case <-timer.C:
			}
			isReady, err := ready()
			if err != nil {
				isReady = false
			}
			if isReady {
				continue
			}
			if log != nil {
				log("tailnet interface unavailable; waiting to rebind")
			}
			_ = currentServer.Close()
			<-serveErr
			for {
				select {
				case <-ctx.Done():
					shutdown(currentServer)
					return http.ErrServerClosed
				case <-time.After(pollInterval):
				}
				isReady, err = ready()
				if err == nil && isReady {
					break
				}
			}
			if ctx.Err() != nil {
				shutdown(currentServer)
				return http.ErrServerClosed
			}
			newListener, err := bind(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return http.ErrServerClosed
				}
				return err
			}
			if ctx.Err() != nil {
				// Cancellation can land between the readiness check and the
				// bind return; never start a new server once shutdown has begun.
				_ = newListener.Close()
				return http.ErrServerClosed
			}
			currentServer = cloneServerConfig(server)
			currentListener = newListener
			if log != nil {
				log("tailnet interface restored; listener rebound")
			}
			break
		}
	}
}

// cloneServerConfig mirrors every exported http.Server configuration field
// because Close permanently makes a server instance unservable and Go's
// http.Server has no Clone method. Addr is intentionally omitted: it only
// applies to ListenAndServe, and rebind always serves on an explicitly bound
// listener.
func cloneServerConfig(server *http.Server) *http.Server {
	return &http.Server{
		Handler:                      server.Handler,
		ReadTimeout:                  server.ReadTimeout,
		ReadHeaderTimeout:            server.ReadHeaderTimeout,
		WriteTimeout:                 server.WriteTimeout,
		IdleTimeout:                  server.IdleTimeout,
		MaxHeaderBytes:               server.MaxHeaderBytes,
		MaxHeaderValueCount:          server.MaxHeaderValueCount,
		ConnState:                    server.ConnState,
		ErrorLog:                     server.ErrorLog,
		BaseContext:                  server.BaseContext,
		ConnContext:                  server.ConnContext,
		HTTP2:                        server.HTTP2,
		Protocols:                    server.Protocols,
		DisableClientPriority:        server.DisableClientPriority,
		DisableGeneralOptionsHandler: server.DisableGeneralOptionsHandler,
		TLSConfig:                    server.TLSConfig,
		TLSNextProto:                 server.TLSNextProto,
	}
}

func shutdown(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func startServer(server *http.Server, listener net.Listener, result chan<- error) {
	go func() { result <- server.Serve(listener) }()
}

func serveOnce(ctx context.Context, server *http.Server, listener net.Listener, start serverStarter) error {
	if ctx.Err() != nil {
		_ = listener.Close()
		return http.ErrServerClosed
	}
	done := make(chan error, 1)
	start(server, listener, done)
	select {
	case <-ctx.Done():
		shutdown(server)
		return http.ErrServerClosed
	case err := <-done:
		return err
	}
}
