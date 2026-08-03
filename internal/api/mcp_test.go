package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkLord19/foglet/internal/mcp"
)

func newTestServerWithMCP(t *testing.T) (*Server, *mcp.Proxy, string) {
	t.Helper()
	srv := newTestServer(t)
	fogHome := t.TempDir()
	proxy := mcp.NewProxy(nil, srv.stateStore)
	srv.SetMCP(proxy, fogHome)
	return srv, proxy, fogHome
}

func TestHandleMCPGetEmpty(t *testing.T) {
	srv, _, _ := newTestServerWithMCP(t)
	req := httptest.NewRequest(http.MethodGet, "/api/mcp", nil)
	w := httptest.NewRecorder()
	srv.handleMCP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", w.Code, w.Body.String())
	}
	var resp MCPConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(resp.Upstreams) != 0 {
		t.Fatalf("expected empty upstreams, got %d", len(resp.Upstreams))
	}
}

func TestHandleMCPPutAndGet(t *testing.T) {
	srv, proxy, fogHome := newTestServerWithMCP(t)

	body := `{"upstreams":[{"name":"acme","url":"https://acme.example.com/mcp","token":"tok123"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleMCP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", w.Code, w.Body.String())
	}
	var resp MCPConfigResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(resp.Upstreams) != 1 || resp.Upstreams[0].Name != "acme" {
		t.Fatalf("unexpected upstreams: %+v", resp.Upstreams)
	}
	if !resp.Upstreams[0].HasToken {
		t.Fatal("expected has_token=true")
	}

	// Secret must be saved under mcp_token_acme.
	secret, found, err := srv.stateStore.GetSecret("mcp_token_acme")
	if err != nil || !found || secret != "tok123" {
		t.Fatalf("secret mismatch: found=%v secret=%q err=%v", found, secret, err)
	}

	// Live proxy must be updated.
	ups := proxy.Upstreams()
	if len(ups) != 1 || ups[0].Name != "acme" || ups[0].URL != "https://acme.example.com/mcp" {
		t.Fatalf("proxy upstreams mismatch: %+v", ups)
	}

	// mcp.json must be written.
	data, err := os.ReadFile(filepath.Join(fogHome, "mcp.json"))
	if err != nil {
		t.Fatalf("read mcp.json: %v", err)
	}
	var saved []mcp.Upstream
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("parse mcp.json: %v", err)
	}
	if len(saved) != 1 || saved[0].Name != "acme" {
		t.Fatalf("mcp.json content mismatch: %+v", saved)
	}
}

func TestHandleMCPPutBlankTokenKeepsExisting(t *testing.T) {
	srv, _, _ := newTestServerWithMCP(t)
	if err := srv.stateStore.SaveSecret("mcp_token_x", "old_tok"); err != nil {
		t.Fatal(err)
	}

	// PUT with no token field — existing secret must be preserved.
	body := `{"upstreams":[{"name":"x","url":"https://x.example.com/mcp"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleMCP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", w.Code, w.Body.String())
	}
	// Secret must not be overwritten.
	secret, found, _ := srv.stateStore.GetSecret("mcp_token_x")
	if !found || secret != "old_tok" {
		t.Fatalf("expected old_tok, got %q (found=%v)", secret, found)
	}
	var resp MCPConfigResponse
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Upstreams) == 0 || !resp.Upstreams[0].HasToken {
		t.Fatal("expected has_token=true for kept secret")
	}
}

func TestHandleMCPPutDeleteOrphansSecret(t *testing.T) {
	srv, _, _ := newTestServerWithMCP(t)
	// Seed proxy with an upstream whose secret will be orphaned.
	srv.mcpProxy.SetUpstreams([]mcp.Upstream{{Name: "old", URL: "https://old.example.com/mcp", SecretKey: "mcp_token_old"}})
	if err := srv.stateStore.SaveSecret("mcp_token_old", "todelete"); err != nil {
		t.Fatal(err)
	}

	// PUT with "old" removed.
	body := `{"upstreams":[]}`
	req := httptest.NewRequest(http.MethodPut, "/api/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleMCP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", w.Code, w.Body.String())
	}
	_, found, _ := srv.stateStore.GetSecret("mcp_token_old")
	if found {
		t.Fatal("orphaned secret was not deleted")
	}
}

func TestHandleMCPPutRejectsDunderName(t *testing.T) {
	srv, _, _ := newTestServerWithMCP(t)
	body := `{"upstreams":[{"name":"bad__name","url":"https://x.example.com/mcp"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleMCP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleMCPPutRejectsBadScheme(t *testing.T) {
	srv, _, _ := newTestServerWithMCP(t)
	body := `{"upstreams":[{"name":"x","url":"ftp://x.example.com/mcp"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleMCP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHandleMCPPutRejectsDuplicateNames(t *testing.T) {
	srv, _, _ := newTestServerWithMCP(t)
	body := `{"upstreams":[{"name":"a","url":"https://a.example.com/mcp"},{"name":"a","url":"https://b.example.com/mcp"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/mcp", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleMCP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
