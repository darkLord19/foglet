package slack

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const slackSignatureMaxAge = 5 * time.Minute

// verifySlackSignature authenticates an HTTP request sent by Slack.
//
// Slack's timestamp window is also the replay protection: a signed request
// outside the window is rejected before its payload is parsed or acted on.
func verifySlackSignature(signingSecret string, headers http.Header, body []byte, now time.Time) error {
	signingSecret = strings.TrimSpace(signingSecret)
	if signingSecret == "" {
		return errors.New("signing secret is required")
	}

	timestamp := strings.TrimSpace(headers.Get("X-Slack-Request-Timestamp"))
	signature := strings.TrimSpace(headers.Get("X-Slack-Signature"))
	if timestamp == "" || signature == "" {
		return errors.New("missing slack signature headers")
	}

	timestampUnix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid slack timestamp: %w", err)
	}

	requestTime := time.Unix(timestampUnix, 0)
	currentTime := now.UTC()
	if currentTime.Sub(requestTime) > slackSignatureMaxAge || requestTime.Sub(currentTime) > slackSignatureMaxAge {
		return errors.New("slack timestamp out of range")
	}

	base := "v0:" + timestamp + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(signingSecret))
	_, _ = mac.Write([]byte(base))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return errors.New("signature mismatch")
	}

	return nil
}
