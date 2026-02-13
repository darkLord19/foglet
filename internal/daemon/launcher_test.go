package daemon

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestIsHealthyWithClientOK(t *testing.T) {
	client := &http.Client{Transport: roundTrip(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString("ok")),
			Header:     make(http.Header),
		}
	})}

	if !isHealthyWithClient(client, "http://example.local/health") {
		t.Fatal("expected health check to pass")
	}
}

func TestIsHealthyWithClientErrorStatus(t *testing.T) {
	client := &http.Client{Transport: roundTrip(func(req *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       io.NopCloser(bytes.NewBufferString("err")),
			Header:     make(http.Header),
		}
	})}

	if isHealthyWithClient(client, "http://example.local/health") {
		t.Fatal("expected health check to fail")
	}
}

func TestWaitForHealthTimeout(t *testing.T) {
	start := time.Now()
	err := waitForHealth("http://127.0.0.1:1/health", 900*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if time.Since(start) < 900*time.Millisecond {
		t.Fatal("expected waitForHealth to wait for timeout duration")
	}
}

func TestEnsureRunningStartsEmbeddedDaemon(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "operation not permitted") {
			t.Skip("sandbox does not permit local tcp bind")
		}
		t.Fatalf("reserve local port failed: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	baseURL, _, err := EnsureRunning(t.TempDir(), port, 5*time.Second)
	if err != nil {
		t.Fatalf("ensure running failed: %v", err)
	}

	resp, err := (&http.Client{Timeout: 2 * time.Second}).Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("unexpected health status: %d", resp.StatusCode)
	}
}

type roundTrip func(*http.Request) *http.Response

func (f roundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}
