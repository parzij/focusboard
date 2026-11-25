package main

import (
	"context"
	"net/http"
	"testing"
)

func TestServeReportsInvalidAddress(t *testing.T) {
	server := &http.Server{Addr: "invalid-address"}

	if err := serve(t.Context(), server); err == nil {
		t.Fatal("serve succeeded with an invalid listening address")
	}
}

func TestServeStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	server := &http.Server{Addr: "127.0.0.1:0"}
	if err := serve(ctx, server); err != nil {
		t.Fatal(err)
	}
}
