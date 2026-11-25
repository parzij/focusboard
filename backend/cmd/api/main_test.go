package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServeReportsInvalidAddress(t *testing.T) {
	server := &http.Server{Addr: "invalid-address"}

	if err := serve(t.Context(), server, time.Second); err == nil {
		t.Fatal("serve succeeded with an invalid listening address")
	}
}

func TestServeStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	server := &http.Server{Addr: "127.0.0.1:0"}
	if err := serve(ctx, server, time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestServeDrainsActiveRequest(t *testing.T) {
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
		}
	})}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- serveListener(ctx, server, listener, 3*time.Second) }()

	client := &http.Client{Timeout: 3 * time.Second}
	response := make(chan int, 1)
	go func() {
		defer close(response)
		result, err := client.Get("http://" + listener.Addr().String())
		if err == nil {
			defer result.Body.Close()
			response <- result.StatusCode
		}
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("test request did not reach the server")
	}
	shutdownStarted := make(chan struct{})
	server.RegisterOnShutdown(func() { close(shutdownStarted) })
	cancel()
	select {
	case <-shutdownStarted:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not start")
	}
	close(release)
	if status := <-response; status != http.StatusNoContent {
		t.Fatalf("in-flight request returned %d", status)
	}
	if err := <-stopped; err != nil {
		t.Fatal(err)
	}
}
