package cloud

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestHandleInstallRedirectsToSlackOAuth(t *testing.T) {
	store := newCloudStore(t)
	defer func() { _ = store.Close() }()

	srv, err := NewServer(store, Config{
		ClientID:      "client-id",
		ClientSecret:  "client-secret",
		SigningSecret: "signing-secret",
		PublicURL:     "https://fogcloud.example",
	})
	if err != nil {
		t.Fatalf("new server failed: %v", err)
	}

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/slack/install", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusFound)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://slack.com/oauth/v2/authorize?") {
		t.Fatalf("unexpected redirect: %q", loc)
	}
	u, err := url.Parse(loc)
	if err != nil {
		t.Fatalf("parse location failed: %v", err)
	}
	q := u.Query()
	if q.Get("client_id") != "client-id" {
		t.Fatalf("unexpected client_id: %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "https://fogcloud.example/slack/oauth/callback" {
		t.Fatalf("unexpected redirect_uri: %q", q.Get("redirect_uri"))
	}
	if strings.TrimSpace(q.Get("state")) == "" {
		t.Fatal("expected oauth state")
	}
}

func TestHandleOAuthCallbackStoresInstallation(t *testing.T) {
	store := newCloudStore(t)
	defer func() { _ = store.Close() }()

	const (
		teamID   = "T123"
		botToken = "xoxb-cloud-token"
		botUser  = "U_BOT"
	)

	slackMux := http.NewServeMux()
	slackMux.HandleFunc("/oauth.v2.access", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form failed: %v", err)
		}
		if r.Form.Get("client_id") != "cid" || r.Form.Get("client_secret") != "csecret" {
			t.Fatalf("missing oauth client credentials")
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":           true,
			"access_token": botToken,
			"bot_user_id":  botUser,
			"team": map[string]string{
				"id": teamID,
			},
		})
	})
	slackServer := newHTTPTestServerOrSkip(t, slackMux)
	defer slackServer.Close()

	server, err := NewServer(store, Config{
		ClientID:       "cid",
		ClientSecret:   "csecret",
		SigningSecret:  "secret",
		PublicURL:      "https://fogcloud.example",
		OAuthAccessURL: slackServer.URL + "/oauth.v2.access",
	})
	if err != nil {
		t.Fatalf("new server failed: %v", err)
	}

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	installReq := httptest.NewRequest(http.MethodGet, "/slack/install", nil)
	installRec := httptest.NewRecorder()
	mux.ServeHTTP(installRec, installReq)
	if installRec.Code != http.StatusFound {
		t.Fatalf("unexpected install status: %d", installRec.Code)
	}
	loc, err := url.Parse(installRec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse install redirect failed: %v", err)
	}
	stateToken := strings.TrimSpace(loc.Query().Get("state"))
	if stateToken == "" {
		t.Fatal("expected state token")
	}

	callbackReq := httptest.NewRequest(http.MethodGet, "/slack/oauth/callback?state="+url.QueryEscape(stateToken)+"&code=auth-code", nil)
	callbackRec := httptest.NewRecorder()
	mux.ServeHTTP(callbackRec, callbackReq)

	if callbackRec.Code != http.StatusOK {
		t.Fatalf("unexpected callback status: got=%d body=%q", callbackRec.Code, callbackRec.Body.String())
	}

	inst, found, err := store.GetInstallation(teamID)
	if err != nil {
		t.Fatalf("get installation failed: %v", err)
	}
	if !found {
		t.Fatal("expected installation to exist")
	}
	if inst.BotToken != botToken || inst.BotUserID != botUser {
		t.Fatalf("unexpected installation values: %+v", inst)
	}
}

func TestHandleEventsURLVerification(t *testing.T) {
	store := newCloudStore(t)
	defer func() { _ = store.Close() }()

	server, err := NewServer(store, Config{
		ClientID:      "cid",
		ClientSecret:  "secret",
		SigningSecret: "signing-secret",
		PublicURL:     "https://fogcloud.example",
	})
	if err != nil {
		t.Fatalf("new server failed: %v", err)
	}

	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	body := []byte(`{"type":"url_verification","challenge":"abc123"}`)
	req := signedSlackRequest(t, "/slack/events", "signing-secret", body, time.Now().UTC())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}
	var out map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if out["challenge"] != "abc123" {
		t.Fatalf("unexpected challenge: %q", out["challenge"])
	}
}

func TestHandleEventsAppMentionUnpairedPostsEphemeral(t *testing.T) {
	store := newCloudStore(t)
	defer func() { _ = store.Close() }()

	if err := store.SaveInstallation("T111", "U_BOT", "xoxb-test-token"); err != nil {
		t.Fatalf("save installation failed: %v", err)
	}

	ephemeralCh := make(chan map[string]string, 1)
	slackMux := http.NewServeMux()
	slackMux.HandleFunc("/chat.postEphemeral", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer xoxb-test-token" {
			t.Fatalf("unexpected auth header: %q", got)
		}
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload failed: %v", err)
		}
		ephemeralCh <- payload
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})
	slackServer := newHTTPTestServerOrSkip(t, slackMux)
	defer slackServer.Close()

	server, err := NewServer(store, Config{
		ClientID:      "cid",
		ClientSecret:  "secret",
		SigningSecret: "signing-secret",
		PublicURL:     "https://fogcloud.example",
		APIBaseURL:    slackServer.URL,
	})
	if err != nil {
		t.Fatalf("new server failed: %v", err)
	}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	body := []byte(`{
		"type":"event_callback",
		"team_id":"T111",
		"event_id":"Ev-1",
		"event":{"type":"app_mention","channel":"C1","user":"U1","text":"<@U_BOT> hi","ts":"123.456"}
	}`)
	req := signedSlackRequest(t, "/slack/events", "signing-secret", body, time.Now().UTC())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	select {
	case payload := <-ephemeralCh:
		if payload["channel"] != "C1" || payload["user"] != "U1" {
			t.Fatalf("unexpected payload routing fields: %+v", payload)
		}
		if !strings.Contains(payload["text"], "not paired") {
			t.Fatalf("unexpected payload text: %q", payload["text"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for chat.postEphemeral call")
	}
}

func TestHandleEventsAppMentionPairedQueuesJobAndDeviceClaimsIt(t *testing.T) {
	store := newCloudStore(t)
	defer func() { _ = store.Close() }()

	if err := store.SaveInstallation("T111", "U_BOT", "xoxb-test-token"); err != nil {
		t.Fatalf("save installation failed: %v", err)
	}
	pairReq, err := store.CreatePairingRequest("T111", "U1", "C1", "123.456", 5*time.Minute)
	if err != nil {
		t.Fatalf("create pairing request failed: %v", err)
	}
	claim, err := store.ClaimPairingRequest(pairReq.Code, "device-a", "")
	if err != nil {
		t.Fatalf("claim pairing request failed: %v", err)
	}

	msgCh := make(chan map[string]string, 1)
	slackMux := http.NewServeMux()
	slackMux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload failed: %v", err)
		}
		msgCh <- payload
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})
	slackServer := newHTTPTestServerOrSkip(t, slackMux)
	defer slackServer.Close()

	server, err := NewServer(store, Config{
		ClientID:      "cid",
		ClientSecret:  "secret",
		SigningSecret: "signing-secret",
		PublicURL:     "https://fogcloud.example",
		APIBaseURL:    slackServer.URL,
	})
	if err != nil {
		t.Fatalf("new server failed: %v", err)
	}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	body := []byte(`{
		"type":"event_callback",
		"team_id":"T111",
		"event_id":"Ev-2",
		"event":{
			"type":"app_mention",
			"channel":"C1",
			"user":"U1",
			"text":"<@U_BOT> [repo='owner/repo' tool='claude' autopr=true] implement auth",
			"ts":"123.456"
		}
	}`)
	req := signedSlackRequest(t, "/slack/events", "signing-secret", body, time.Now().UTC())
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", rec.Code)
	}

	select {
	case payload := <-msgCh:
		if payload["channel"] != "C1" || payload["thread_ts"] != "123.456" {
			t.Fatalf("unexpected message payload: %+v", payload)
		}
		if !strings.Contains(payload["text"], "Queued on your paired Fog device") {
			t.Fatalf("unexpected message text: %q", payload["text"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for queued message")
	}

	claimReq := httptest.NewRequest(http.MethodPost, "/v1/device/jobs/claim", nil)
	claimReq.Header.Set("X-Fog-Device-ID", "device-a")
	claimReq.Header.Set("Authorization", "Bearer "+claim.DeviceToken)
	claimRec := httptest.NewRecorder()
	mux.ServeHTTP(claimRec, claimReq)
	if claimRec.Code != http.StatusOK {
		t.Fatalf("unexpected claim status: got=%d body=%q", claimRec.Code, claimRec.Body.String())
	}
	var job Job
	if err := json.NewDecoder(claimRec.Body).Decode(&job); err != nil {
		t.Fatalf("decode claimed job failed: %v", err)
	}
	if job.Kind != jobKindStartSession || job.Repo != "owner/repo" || job.Tool != "claude" {
		t.Fatalf("unexpected claimed job: %+v", job)
	}
}

func TestDeviceJobCompleteMapsThreadSessionAndPostsCompletion(t *testing.T) {
	store := newCloudStore(t)
	defer func() { _ = store.Close() }()

	if err := store.SaveInstallation("T111", "U_BOT", "xoxb-test-token"); err != nil {
		t.Fatalf("save installation failed: %v", err)
	}
	pairReq, err := store.CreatePairingRequest("T111", "U1", "C1", "123.456", 5*time.Minute)
	if err != nil {
		t.Fatalf("create pairing request failed: %v", err)
	}
	claim, err := store.ClaimPairingRequest(pairReq.Code, "device-a", "")
	if err != nil {
		t.Fatalf("claim pairing request failed: %v", err)
	}
	job, err := store.EnqueueJob(Job{
		DeviceID:    "device-a",
		TeamID:      "T111",
		ChannelID:   "C1",
		RootTS:      "123.456",
		SlackUserID: "U1",
		Kind:        jobKindStartSession,
		Repo:        "owner/repo",
		Prompt:      "implement auth",
	})
	if err != nil {
		t.Fatalf("enqueue job failed: %v", err)
	}

	msgCh := make(chan map[string]string, 1)
	slackMux := http.NewServeMux()
	slackMux.HandleFunc("/chat.postMessage", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload failed: %v", err)
		}
		msgCh <- payload
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
	})
	slackServer := newHTTPTestServerOrSkip(t, slackMux)
	defer slackServer.Close()

	server, err := NewServer(store, Config{
		ClientID:      "cid",
		ClientSecret:  "secret",
		SigningSecret: "signing-secret",
		PublicURL:     "https://fogcloud.example",
		APIBaseURL:    slackServer.URL,
	})
	if err != nil {
		t.Fatalf("new server failed: %v", err)
	}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	payload := map[string]interface{}{
		"success":    true,
		"session_id": "session-1",
		"run_id":     "run-1",
		"branch":     "fog/auth",
		"pr_url":     "https://github.com/acme/repo/pull/1",
	}
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/v1/device/jobs/"+job.ID+"/complete", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Fog-Device-ID", "device-a")
	req.Header.Set("Authorization", "Bearer "+claim.DeviceToken)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got=%d body=%q", rec.Code, rec.Body.String())
	}

	sessionID, found, err := store.GetThreadSession("T111", "C1", "123.456")
	if err != nil {
		t.Fatalf("get thread session failed: %v", err)
	}
	if !found || sessionID != "session-1" {
		t.Fatalf("unexpected thread session mapping: found=%v id=%q", found, sessionID)
	}

	select {
	case msg := <-msgCh:
		if !strings.Contains(msg["text"], "Completed on branch") {
			t.Fatalf("unexpected completion message: %q", msg["text"])
		}
		if !strings.Contains(msg["text"], "https://github.com/acme/repo/pull/1") {
			t.Fatalf("expected pr url in completion message: %q", msg["text"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for completion message")
	}
}

func TestHandleEventsRejectsInvalidSignature(t *testing.T) {
	store := newCloudStore(t)
	defer func() { _ = store.Close() }()

	server, err := NewServer(store, Config{
		ClientID:      "cid",
		ClientSecret:  "secret",
		SigningSecret: "signing-secret",
		PublicURL:     "https://fogcloud.example",
	})
	if err != nil {
		t.Fatalf("new server failed: %v", err)
	}
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/slack/events", bytes.NewReader([]byte(`{"type":"event_callback"}`)))
	req.Header.Set("X-Slack-Request-Timestamp", "1")
	req.Header.Set("X-Slack-Signature", "v0=invalid")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status: got=%d want=%d", rec.Code, http.StatusUnauthorized)
	}
}

func signedSlackRequest(t *testing.T, path, secret string, body []byte, now time.Time) *http.Request {
	t.Helper()

	ts := now.UTC().Unix()
	tsStr := strconv.FormatInt(ts, 10)
	base := "v0:" + tsStr + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(base))
	sig := "v0=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slack-Request-Timestamp", tsStr)
	req.Header.Set("X-Slack-Signature", sig)
	return req
}

func newCloudStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new cloud store failed: %v", err)
	}
	return store
}

func newHTTPTestServerOrSkip(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("skipping http server test: %v", r)
		}
	}()
	return httptest.NewServer(handler)
}
