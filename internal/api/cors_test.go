package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithCORSSetsHeadersForWailsOrigin(t *testing.T) {
	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.Header.Set("Origin", "wails://wails")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "wails://wails" {
		t.Fatalf("unexpected allow origin header: %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatal("expected allow methods header")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Fatal("expected allow headers")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected response code: %d", rec.Code)
	}
}

func TestWithCORSSetsHeadersForLocalhostOrigin(t *testing.T) {
	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.Header.Set("Origin", "http://localhost:8080")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8080" {
		t.Fatalf("unexpected allow origin header: %q", got)
	}
}

func TestWithCORSRejectsUnknownOrigin(t *testing.T) {
	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header for evil origin, got %q", got)
	}
	// Request still succeeds — CORS is browser-enforced, but the header is absent.
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: %d", rec.Code)
	}
}

func TestWithCORSHandlesOptions(t *testing.T) {
	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not run for OPTIONS")
	}))

	req := httptest.NewRequest(http.MethodOptions, "/api/tasks", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8080")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected response code: %d", rec.Code)
	}
}

func TestWithCORSNoOriginHeader(t *testing.T) {
	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no CORS header without Origin, got %q", got)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("unexpected status code: %d", rec.Code)
	}
}

func TestAllowedCORSOriginVariants(t *testing.T) {
	cases := []struct {
		origin string
		want   bool
	}{
		{"wails://wails", true},
		{"http://wails.localhost", true},
		{"http://wails.localhost:3000", true},
		{"http://localhost:8080", true},
		{"http://localhost", true},
		{"http://127.0.0.1:8080", true},
		{"http://127.0.0.1", true},
		{"https://evil.com", false},
		{"http://example.com:8080", false},
		{"", false},
	}
	for _, tc := range cases {
		got := allowedCORSOrigin(tc.origin)
		if tc.want && got == "" {
			t.Errorf("expected origin %q to be allowed", tc.origin)
		}
		if !tc.want && got != "" {
			t.Errorf("expected origin %q to be rejected, got %q", tc.origin, got)
		}
	}
}
