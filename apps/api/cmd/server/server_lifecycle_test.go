package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/Ilia-Shakeri/stackplore/apps/api/internal/httpapi"
)

func TestServeServerCancelsOpenStreamBeforeGracefulDrain(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	streamDone := make(chan struct{})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Errorf("stream flush: %v", err)
			return
		}
		<-r.Context().Done()
		close(streamDone)
	})}
	readiness := httpapi.NewReadiness()
	readiness.SetDatabaseReady(true)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serveServer(ctx, server, listener, readiness, 500*time.Millisecond) }()
	waitFor(t, readiness.IsReady)
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	response, err := client.Get("http://" + listener.Addr().String() + "/api/v1/streams/alerts")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("open stream forced shutdown timeout: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not finish within its bound")
	}
	select {
	case <-streamDone:
	default:
		t.Fatal("stream handler outlived server shutdown")
	}
}
