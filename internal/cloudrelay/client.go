package cloudrelay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/cloud"
)

type ClientConfig struct {
	BaseURL     string
	DeviceID    string
	DeviceToken string
	HTTPClient  *http.Client
}

type Client struct {
	baseURL     string
	deviceID    string
	deviceToken string
	httpClient  *http.Client
}

type PairClaimResponse struct {
	TeamID      string `json:"team_id"`
	SlackUserID string `json:"slack_user_id"`
	DeviceID    string `json:"device_id"`
	DeviceToken string `json:"device_token,omitempty"`
}

type CompletePayload struct {
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	Branch    string `json:"branch,omitempty"`
	PRURL     string `json:"pr_url,omitempty"`
	CommitSHA string `json:"commit_sha,omitempty"`
	CommitMsg string `json:"commit_msg,omitempty"`
}

func NewClient(cfg ClientConfig) (*Client, error) {
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	if cfg.BaseURL == "" {
		return nil, errors.New("base_url is required")
	}
	cfg.DeviceID = strings.TrimSpace(cfg.DeviceID)
	cfg.DeviceToken = strings.TrimSpace(cfg.DeviceToken)
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:     strings.TrimRight(cfg.BaseURL, "/"),
		deviceID:    cfg.DeviceID,
		deviceToken: cfg.DeviceToken,
		httpClient:  cfg.HTTPClient,
	}, nil
}

func (c *Client) WithDeviceAuth(deviceID, token string) *Client {
	copy := *c
	copy.deviceID = strings.TrimSpace(deviceID)
	copy.deviceToken = strings.TrimSpace(token)
	return &copy
}

func (c *Client) DeviceID() string {
	return c.deviceID
}

func (c *Client) DeviceToken() string {
	return c.deviceToken
}

func (c *Client) ClaimPairing(ctx context.Context, code string) (PairClaimResponse, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return PairClaimResponse{}, errors.New("pair code is required")
	}
	if strings.TrimSpace(c.deviceID) == "" {
		return PairClaimResponse{}, errors.New("device id is required")
	}

	payload := map[string]string{
		"code":      code,
		"device_id": c.deviceID,
	}
	if strings.TrimSpace(c.deviceToken) != "" {
		payload["device_token"] = c.deviceToken
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/pair/claim", bytes.NewReader(body))
	if err != nil {
		return PairClaimResponse{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return PairClaimResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return PairClaimResponse{}, decodeAPIError(resp)
	}
	var out PairClaimResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return PairClaimResponse{}, err
	}
	return out, nil
}

func (c *Client) Unpair(ctx context.Context, teamID, slackUserID string) error {
	if err := c.requireDeviceAuth(); err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]string{
		"team_id":       strings.TrimSpace(teamID),
		"slack_user_id": strings.TrimSpace(slackUserID),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/pair/unpair", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addDeviceAuthHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return decodeAPIError(resp)
	}
	return nil
}

func (c *Client) ClaimJob(ctx context.Context) (cloud.Job, bool, error) {
	if err := c.requireDeviceAuth(); err != nil {
		return cloud.Job{}, false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/device/jobs/claim", nil)
	if err != nil {
		return cloud.Job{}, false, err
	}
	c.addDeviceAuthHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return cloud.Job{}, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return cloud.Job{}, false, nil
	}
	if resp.StatusCode/100 != 2 {
		return cloud.Job{}, false, decodeAPIError(resp)
	}
	var job cloud.Job
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return cloud.Job{}, false, err
	}
	return job, true, nil
}

func (c *Client) CompleteJob(ctx context.Context, jobID string, payload CompletePayload) error {
	if err := c.requireDeviceAuth(); err != nil {
		return err
	}
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return errors.New("job id is required")
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/device/jobs/"+jobID+"/complete", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.addDeviceAuthHeaders(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return decodeAPIError(resp)
	}
	return nil
}

func (c *Client) requireDeviceAuth() error {
	if strings.TrimSpace(c.deviceID) == "" || strings.TrimSpace(c.deviceToken) == "" {
		return errors.New("device auth is required")
	}
	return nil
}

func (c *Client) addDeviceAuthHeaders(req *http.Request) {
	req.Header.Set("X-Fog-Device-ID", c.deviceID)
	req.Header.Set("Authorization", "Bearer "+c.deviceToken)
}

func decodeAPIError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var out struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &out)
	msg := strings.TrimSpace(out.Error)
	if msg == "" {
		msg = strings.TrimSpace(string(body))
	}
	if msg == "" {
		msg = strings.TrimSpace(resp.Status)
	}
	return fmt.Errorf("cloud api error: %s", msg)
}
