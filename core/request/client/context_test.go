package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer srv.Close()
	c := NewClient("", 1, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	c.Context = ctx
	if _, err := c.Get(srv.URL, nil); err == nil {
		t.Fatal("canceled request succeeded")
	}
}
func TestHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if _, err := NewClient("", 1, 0).Get(srv.URL, nil); err == nil {
		t.Fatal("404 treated as HTML")
	}
}

func TestCancellationWhileProxyCONNECTWaits(t *testing.T) {
	release := make(chan struct{})
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer proxy.Close()
	defer close(release)
	c := NewClient(proxy.URL, 1, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	c.Context = ctx
	start := time.Now()
	if _, err := c.Get("https://example.com", nil); err == nil || time.Since(start) > time.Second {
		t.Fatalf("CONNECT ignored cancellation: %v", err)
	}
}
