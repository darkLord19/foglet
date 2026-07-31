package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type fakeSecrets map[string]string

func (f fakeSecrets) GetSecret(key string) (string, bool, error) {
	v, ok := f[key]
	return v, ok, nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestProxyInitializeAndList(t *testing.T) {
	proxy := NewProxy([]Upstream{{Name: "docs", URL: "http://docs.test/mcp"}}, nil)
	proxy.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var req jsonRPCRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method != "tools/list" {
			t.Fatalf("upstream method = %q", req.Method)
		}
		body, _ := json.Marshal(jsonRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Result: json.RawMessage(`{"tools":[{"name":"search","description":"Search"}]}`),
		})
		return jsonResponse(string(body)), nil
	})}

	initReq, _ := http.NewRequest(http.MethodPost, "/mcp", stringsReader(`{"jsonrpc":"2.0","id":1,"method":"initialize"}`))
	initRec := &recordingResponse{header: make(http.Header)}
	proxy.ServeHTTP(initRec, initReq)
	if initRec.status != http.StatusOK {
		t.Fatalf("initialize status = %d", initRec.status)
	}

	listReq, _ := http.NewRequest(http.MethodPost, "/mcp", stringsReader(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	listRec := &recordingResponse{header: make(http.Header)}
	proxy.ServeHTTP(listRec, listReq)
	if listRec.status != http.StatusOK {
		t.Fatalf("list status = %d", listRec.status)
	}
	var body struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(listRec.body, &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Result.Tools) != 1 || body.Result.Tools[0].Name != "docs__search" {
		t.Fatalf("tools = %+v", body.Result.Tools)
	}
}

func TestProxyCallUsesSecretReference(t *testing.T) {
	proxy := NewProxy([]Upstream{{Name: "docs", URL: "http://docs.test/mcp", SecretKey: "mcp.docs"}}, fakeSecrets{"mcp.docs": "secret"})
	proxy.client = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Fatalf("authorization = %q", got)
		}
		body, _ := json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: json.RawMessage(`3`), Result: json.RawMessage(`{"content":[{"type":"text","text":"ok"}]}`)})
		return jsonResponse(string(body)), nil
	})}
	req, _ := http.NewRequest(http.MethodPost, "/mcp", stringsReader(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"docs__search","arguments":{"q":"fog"}}}`))
	rec := &recordingResponse{header: make(http.Header)}
	proxy.ServeHTTP(rec, req)
	if rec.status != http.StatusOK {
		t.Fatalf("call status = %d", rec.status)
	}
}

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }

type recordingResponse struct {
	header http.Header
	status int
	body   []byte
}

func (r *recordingResponse) Header() http.Header    { return r.header }
func (r *recordingResponse) WriteHeader(status int) { r.status = status }
func (r *recordingResponse) Write(body []byte) (int, error) {
	r.body = append(r.body, body...)
	return len(body), nil
}
