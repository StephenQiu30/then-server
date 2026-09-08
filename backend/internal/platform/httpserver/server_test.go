package httpserver

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdownDrainsActiveRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	entered, release, drained := make(chan struct{}), make(chan struct{}), make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		_, _ = io.WriteString(w, "complete")
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- Serve(ctx, listener, handler, time.Second, func() { close(drained) }) }()
	response := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		r, e := client.Get("http://" + listener.Addr().String())
		if e != nil {
			response <- e
			return
		}
		defer r.Body.Close()
		b, e := io.ReadAll(r.Body)
		if e == nil && string(b) != "complete" {
			e = errors.New("response truncated")
		}
		response <- e
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request never entered")
	}
	cancel()
	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("drain was not called")
	}
	close(release)
	if err := <-response; err != nil {
		t.Fatal(err)
	}
	if err := <-served; err != nil {
		t.Fatal(err)
	}
}

func TestShutdownDeadlineClosesConnections(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- Serve(ctx, listener, handler, 20*time.Millisecond, func() {}) }()
	response := make(chan struct{})
	go func() {
		defer close(response)
		client := &http.Client{Timeout: time.Second}
		if r, e := client.Get("http://" + listener.Addr().String()); e == nil {
			r.Body.Close()
		}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request never entered")
	}
	cancel()
	if err := <-served; !errors.Is(err, ErrShutdown) {
		t.Fatalf("expected bounded shutdown error, got %v", err)
	}
	<-response
}

func TestListenerFailureClosesActiveConnections(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	entered, canceled, drained := make(chan struct{}), make(chan struct{}), make(chan struct{})
	request, stopRequest := context.WithCancel(context.Background())
	defer stopRequest()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
		close(canceled)
	})
	served := make(chan error, 1)
	go func() {
		served <- Serve(context.Background(), listener, handler, time.Second, func() { close(drained) })
	}()
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		req, err := http.NewRequestWithContext(request, http.MethodGet, "http://"+listener.Addr().String(), nil)
		if err != nil {
			return
		}
		if response, err := http.DefaultClient.Do(req); err == nil {
			response.Body.Close()
		}
	}()
	defer func() { stopRequest(); <-finished }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("request never entered")
	}
	// Fail Accept while a request remains active. Serve must release that connection.
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-served:
		if err == nil {
			t.Fatal("listener failure was ignored")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return")
	}
	select {
	case <-drained:
	default:
		t.Fatal("listener failure did not drain")
	}
	select {
	case <-canceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("active request survived listener failure")
	}
}
