package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/cloudcfg"
	"github.com/darkLord19/foglet/internal/cloudrelay"
	"github.com/google/uuid"
)

type cloudStatusResponse struct {
	CloudURL       string `json:"cloud_url,omitempty"`
	DeviceID       string `json:"device_id,omitempty"`
	HasDeviceToken bool   `json:"has_device_token"`
	Paired         bool   `json:"paired"`
}

func (s *Server) handleCloud(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getCloudStatus(w)
	case http.MethodPut:
		s.updateCloudConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleCloudPair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		http.Error(w, "code is required", http.StatusBadRequest)
		return
	}

	cloudURL, _, _, err := s.loadCloudConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	deviceID, err := s.ensureDeviceID()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	token, _, err := s.stateStore.GetSecret(cloudcfg.SecretCloudDeviceTok)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	client, err := cloudrelay.NewClient(cloudrelay.ClientConfig{
		BaseURL:     cloudURL,
		DeviceID:    deviceID,
		DeviceToken: token,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	resp, err := client.ClaimPairing(ctx, req.Code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(resp.DeviceToken) != "" {
		if err := s.stateStore.SaveSecret(cloudcfg.SecretCloudDeviceTok, strings.TrimSpace(resp.DeviceToken)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	if strings.TrimSpace(resp.DeviceID) != "" && strings.TrimSpace(resp.DeviceID) != deviceID {
		if err := s.stateStore.SetSetting(cloudcfg.SettingCloudDeviceID, strings.TrimSpace(resp.DeviceID)); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	s.getCloudStatus(w)
}

func (s *Server) handleCloudUnpair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TeamID      string `json:"team_id"`
		SlackUserID string `json:"slack_user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	cloudURL, deviceID, deviceToken, err := s.loadCloudConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	client, err := cloudrelay.NewClient(cloudrelay.ClientConfig{
		BaseURL:     cloudURL,
		DeviceID:    deviceID,
		DeviceToken: deviceToken,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	if err := client.Unpair(ctx, strings.TrimSpace(req.TeamID), strings.TrimSpace(req.SlackUserID)); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.getCloudStatus(w)
}

func (s *Server) updateCloudConfig(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CloudURL string `json:"cloud_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	cloudURL := strings.TrimSpace(req.CloudURL)
	if cloudURL == "" {
		http.Error(w, "cloud_url is required", http.StatusBadRequest)
		return
	}
	if err := s.stateStore.SetSetting(cloudcfg.SettingCloudURL, cloudURL); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.getCloudStatus(w)
}

func (s *Server) getCloudStatus(w http.ResponseWriter) {
	cloudURL, _, _ := s.stateStore.GetSetting(cloudcfg.SettingCloudURL)
	deviceID, _, _ := s.stateStore.GetSetting(cloudcfg.SettingCloudDeviceID)
	hasToken, _ := s.stateStore.HasSecret(cloudcfg.SecretCloudDeviceTok)
	resp := cloudStatusResponse{
		CloudURL:       strings.TrimSpace(cloudURL),
		DeviceID:       strings.TrimSpace(deviceID),
		HasDeviceToken: hasToken,
		Paired:         strings.TrimSpace(deviceID) != "" && hasToken,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) loadCloudConfig() (cloudURL, deviceID, deviceToken string, err error) {
	cloudURL, found, err := s.stateStore.GetSetting(cloudcfg.SettingCloudURL)
	if err != nil {
		return "", "", "", err
	}
	if !found || strings.TrimSpace(cloudURL) == "" {
		return "", "", "", errors.New("cloud_url is not configured")
	}
	deviceID, _, err = s.stateStore.GetSetting(cloudcfg.SettingCloudDeviceID)
	if err != nil {
		return "", "", "", err
	}
	deviceToken, _, err = s.stateStore.GetSecret(cloudcfg.SecretCloudDeviceTok)
	if err != nil {
		return "", "", "", err
	}
	return strings.TrimSpace(cloudURL), strings.TrimSpace(deviceID), strings.TrimSpace(deviceToken), nil
}

func (s *Server) ensureDeviceID() (string, error) {
	deviceID, found, err := s.stateStore.GetSetting(cloudcfg.SettingCloudDeviceID)
	if err != nil {
		return "", err
	}
	deviceID = strings.TrimSpace(deviceID)
	if found && deviceID != "" {
		return deviceID, nil
	}
	deviceID = uuid.NewString()
	if err := s.stateStore.SetSetting(cloudcfg.SettingCloudDeviceID, deviceID); err != nil {
		return "", err
	}
	return deviceID, nil
}
