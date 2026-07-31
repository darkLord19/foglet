package slack

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestVerifySlackSignature(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	body := []byte("command=%2Ffog&text=hello+world")
	req := signedRequest(t, "signing-secret", body, now)

	if err := verifySlackSignature("signing-secret", req.Header, body, now); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}

	tampered := append([]byte(nil), body...)
	tampered[0] = 'C'
	if err := verifySlackSignature("signing-secret", req.Header, tampered, now); err == nil || !strings.Contains(err.Error(), "signature mismatch") {
		t.Fatalf("tampered body error = %v, want signature mismatch", err)
	}
}

func TestVerifySlackSignatureRejectsMissingAndStaleRequests(t *testing.T) {
	now := time.Date(2026, time.July, 31, 12, 0, 0, 0, time.UTC)
	body := []byte("command=%2Ffog")

	if err := verifySlackSignature("signing-secret", http.Header{}, body, now); err == nil {
		t.Fatal("missing signature headers must be rejected")
	}

	stale := signedRequest(t, "signing-secret", body, now.Add(-slackSignatureMaxAge-time.Second))
	if err := verifySlackSignature("signing-secret", stale.Header, body, now); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("stale request error = %v, want timestamp out of range", err)
	}

	future := signedRequest(t, "signing-secret", body, now.Add(slackSignatureMaxAge+time.Second))
	if err := verifySlackSignature("signing-secret", future.Header, body, now); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("future request error = %v, want timestamp out of range", err)
	}
}

func TestHandleCommandRejectsUnsignedHTTPRequests(t *testing.T) {
	h := New(nil, nil, "signing-secret")
	req := httptest.NewRequest(http.MethodPost, "/slack/command", strings.NewReader("command=%2Ffog"))
	rec := httptest.NewRecorder()

	h.HandleCommand(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func signedRequest(t *testing.T, secret string, body []byte, now time.Time) *http.Request {
	t.Helper()

	timestamp := strconv.FormatInt(now.Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("v0:" + timestamp + ":" + string(body)))
	signature := "v0=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest(http.MethodPost, "/slack/command", bytes.NewReader(body))
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", signature)
	return req
}
