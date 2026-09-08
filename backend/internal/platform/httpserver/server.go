// Package httpserver provides bounded HTTP shutdown without owning business work.
package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

var ErrShutdown = errors.New("HTTP shutdown deadline exceeded")

// Serve owns listener and waits for its serving goroutine on every exit path.
func Serve(ctx context.Context, listener net.Listener, handler http.Handler, timeout time.Duration, drain func()) error {
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case err := <-done:
		drain()
		// Serve stops accepting after a listener failure but leaves active connections open.
		// Close them before the caller releases shared dependencies such as the database.
		_ = server.Close()
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP listener failed")
	case <-ctx.Done():
		drain()
		shutdown, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := server.Shutdown(shutdown)
		if err != nil {
			_ = server.Close()
		}
		<-done
		if err != nil {
			return ErrShutdown
		}
		return nil
	}
}
