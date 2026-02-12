package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkLord19/foglet/internal/cloudcfg"
)

func TestHandleCloudConfigAndStatus(t *testing.T) {
	srv := newTestServer(t)

	putReq := httptest.NewRequest(http.MethodPut, "/api/cloud", bytes.NewBufferString(`{"cloud_url":"https://cloud.example"}`))
	putReq.Header.Set("Content-Type", "application/json")
	putRec := httptest.NewRecorder()
	srv.handleCloud(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("unexpected PUT status: got=%d body=%q", putRec.Code, putRec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/cloud", nil)
	getRec := httptest.NewRecorder()
	srv.handleCloud(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("unexpected GET status: got=%d", getRec.Code)
	}
	var out cloudStatusResponse
	if err := json.NewDecoder(getRec.Body).Decode(&out); err != nil {
		t.Fatalf("decode status failed: %v", err)
	}
	if out.CloudURL != "https://cloud.example" {
		t.Fatalf("unexpected cloud url: %q", out.CloudURL)
	}
	if out.Paired {
		t.Fatal("did not expect paired status without token")
	}
}

func TestHandleCloudPairPersistsToken(t *testing.T) {
	srv := newTestServer(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/pair/claim", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]string
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatalf("decode pair request failed: %v", err)
		}
		if in["code"] != "PAIR1234" {
			t.Fatalf("unexpected code: %q", in["code"])
		}
		if in["device_id"] == "" {
			t.Fatal("expected generated device_id")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"team_id":       "T1",
			"slack_user_id": "U1",
			"device_id":     in["device_id"],
			"device_token":  "dev-token-1",
		})
	})
	cloudSrv := newHTTPTestServerOrSkipAPI(t, mux)
	defer cloudSrv.Close()

	if err := srv.stateStore.SetSetting(cloudcfg.SettingCloudURL, cloudSrv.URL); err != nil {
		t.Fatalf("set cloud url failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/cloud/pair", bytes.NewBufferString(`{"code":"PAIR1234"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.handleCloudPair(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected pair status: got=%d body=%q", rec.Code, rec.Body.String())
	}

	token, found, err := srv.stateStore.GetSecret(cloudcfg.SecretCloudDeviceTok)
	if err != nil {
		t.Fatalf("get persisted token failed: %v", err)
	}
	if !found || token != "dev-token-1" {
		t.Fatalf("unexpected stored token: found=%v token=%q", found, token)
	}
	deviceID, found, err := srv.stateStore.GetSetting(cloudcfg.SettingCloudDeviceID)
	if err != nil {
		t.Fatalf("get device id failed: %v", err)
	}
	if !found || deviceID == "" {
		t.Fatalf("expected persisted device id: found=%v id=%q", found, deviceID)
	}
}

func newHTTPTestServerOrSkipAPI(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("skipping http server test: %v", r)
		}
	}()
	return httptest.NewServer(handler)
}
