package cloud

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveAndGetInstallationEncrypted(t *testing.T) {
	tmp := t.TempDir()
	store, err := NewStore(tmp)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	token := "xoxb-secret-token"
	if err := store.SaveInstallation("T123", "U999", token); err != nil {
		t.Fatalf("save installation failed: %v", err)
	}

	inst, found, err := store.GetInstallation("T123")
	if err != nil {
		t.Fatalf("get installation failed: %v", err)
	}
	if !found {
		t.Fatal("expected installation")
	}
	if inst.BotToken != token {
		t.Fatalf("token mismatch: got %q want %q", inst.BotToken, token)
	}

	dbBytes, err := os.ReadFile(filepath.Join(tmp, defaultDBName))
	if err != nil {
		t.Fatalf("read sqlite file failed: %v", err)
	}
	if strings.Contains(string(dbBytes), token) {
		t.Fatal("raw token should not appear in sqlite file")
	}
}

func TestPairDeviceRejectsDifferentDeviceUntilUnpaired(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	if err := store.PairDevice("T1", "U1", "device-a"); err != nil {
		t.Fatalf("pair device failed: %v", err)
	}
	if err := store.PairDevice("T1", "U1", "device-b"); err == nil {
		t.Fatal("expected pair rejection for different device")
	}

	if err := store.UnpairDevice("T1", "U1"); err != nil {
		t.Fatalf("unpair failed: %v", err)
	}
	if err := store.PairDevice("T1", "U1", "device-b"); err != nil {
		t.Fatalf("pair after unpair failed: %v", err)
	}
}

func TestRecordEventIDDedup(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	isNew, err := store.RecordEventID("T1", "Ev1")
	if err != nil {
		t.Fatalf("record event failed: %v", err)
	}
	if !isNew {
		t.Fatal("expected first event to be new")
	}

	isNew, err = store.RecordEventID("T1", "Ev1")
	if err != nil {
		t.Fatalf("record duplicate event failed: %v", err)
	}
	if isNew {
		t.Fatal("expected duplicate event to return not-new")
	}
}

func TestThreadSessionRoundTrip(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	if err := store.UpsertThreadSession("T1", "C1", "123.456", "session-1"); err != nil {
		t.Fatalf("upsert thread session failed: %v", err)
	}
	id, found, err := store.GetThreadSession("T1", "C1", "123.456")
	if err != nil {
		t.Fatalf("get thread session failed: %v", err)
	}
	if !found || id != "session-1" {
		t.Fatalf("unexpected thread session result: found=%v id=%q", found, id)
	}
}

func TestClaimPairingRequestIssuesTokenAndSupportsSameDeviceMultiWorkspace(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	req, err := store.CreatePairingRequest("T1", "U1", "C1", "111.222", 5*time.Minute)
	if err != nil {
		t.Fatalf("create pairing request failed: %v", err)
	}

	claim, err := store.ClaimPairingRequest(req.Code, "device-a", "")
	if err != nil {
		t.Fatalf("claim pairing request failed: %v", err)
	}
	if claim.DeviceToken == "" {
		t.Fatal("expected newly issued device token")
	}

	// New workspace pairing for same device must reuse existing device token auth.
	req2, err := store.CreatePairingRequest("T2", "U1", "C9", "999.888", 5*time.Minute)
	if err != nil {
		t.Fatalf("create second pairing request failed: %v", err)
	}
	claim2, err := store.ClaimPairingRequest(req2.Code, "device-a", claim.DeviceToken)
	if err != nil {
		t.Fatalf("claim second pairing request failed: %v", err)
	}
	if claim2.DeviceToken != "" {
		t.Fatal("expected no new token for existing authenticated device")
	}
}

func TestClaimPairingRequestRejectsDifferentDeviceUntilUnpair(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	req, err := store.CreatePairingRequest("T1", "U1", "C1", "111.222", 5*time.Minute)
	if err != nil {
		t.Fatalf("create pairing request failed: %v", err)
	}
	_, err = store.ClaimPairingRequest(req.Code, "device-a", "")
	if err != nil {
		t.Fatalf("initial claim failed: %v", err)
	}

	req2, err := store.CreatePairingRequest("T1", "U1", "C1", "111.222", 5*time.Minute)
	if err != nil {
		t.Fatalf("create second request failed: %v", err)
	}
	if _, err := store.ClaimPairingRequest(req2.Code, "device-b", ""); err == nil {
		t.Fatal("expected claim rejection when user already paired with another device")
	}

	if err := store.UnpairStrict("T1", "U1", "device-a"); err != nil {
		t.Fatalf("strict unpair failed: %v", err)
	}
	req3, err := store.CreatePairingRequest("T1", "U1", "C1", "111.222", 5*time.Minute)
	if err != nil {
		t.Fatalf("create third request failed: %v", err)
	}
	if _, err := store.ClaimPairingRequest(req3.Code, "device-b", ""); err != nil {
		t.Fatalf("claim after strict unpair failed: %v", err)
	}
}

func TestAuthenticateDevice(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	req, err := store.CreatePairingRequest("T1", "U1", "C1", "111.222", 5*time.Minute)
	if err != nil {
		t.Fatalf("create pairing request failed: %v", err)
	}
	claim, err := store.ClaimPairingRequest(req.Code, "device-a", "")
	if err != nil {
		t.Fatalf("claim pairing failed: %v", err)
	}
	if err := store.AuthenticateDevice("device-a", claim.DeviceToken); err != nil {
		t.Fatalf("authenticate device failed: %v", err)
	}
	if err := store.AuthenticateDevice("device-a", "wrong-token"); err == nil {
		t.Fatal("expected auth failure for wrong token")
	}
}

func TestEnqueueClaimAndCompleteJob(t *testing.T) {
	store := newTestStore(t)
	defer func() { _ = store.Close() }()

	req, err := store.CreatePairingRequest("T1", "U1", "C1", "111.222", 5*time.Minute)
	if err != nil {
		t.Fatalf("create pairing request failed: %v", err)
	}
	_, err = store.ClaimPairingRequest(req.Code, "device-a", "")
	if err != nil {
		t.Fatalf("claim pairing failed: %v", err)
	}

	job, err := store.EnqueueJob(Job{
		DeviceID:    "device-a",
		TeamID:      "T1",
		ChannelID:   "C1",
		RootTS:      "111.222",
		SlackUserID: "U1",
		Kind:        jobKindStartSession,
		Repo:        "owner/repo",
		Tool:        "claude",
		AutoPR:      true,
		Prompt:      "implement auth",
	})
	if err != nil {
		t.Fatalf("enqueue job failed: %v", err)
	}
	if job.State != jobStateQueued {
		t.Fatalf("unexpected initial job state: %q", job.State)
	}

	claimed, found, err := store.ClaimNextJob("device-a")
	if err != nil {
		t.Fatalf("claim next job failed: %v", err)
	}
	if !found {
		t.Fatal("expected claimed job")
	}
	if claimed.ID != job.ID || claimed.State != jobStateClaimed {
		t.Fatalf("unexpected claimed job: %+v", claimed)
	}

	completed, err := store.CompleteJob(JobCompletion{
		JobID:     job.ID,
		DeviceID:  "device-a",
		Success:   true,
		SessionID: "session-1",
		RunID:     "run-1",
		Branch:    "fog/auth",
		PRURL:     "https://github.com/acme/repo/pull/1",
		CommitSHA: "abc123",
		CommitMsg: "feat: add auth",
	})
	if err != nil {
		t.Fatalf("complete job failed: %v", err)
	}
	if completed.State != jobStateCompleted {
		t.Fatalf("unexpected completed state: %q", completed.State)
	}
	if completed.SessionID != "session-1" || completed.RunID != "run-1" {
		t.Fatalf("unexpected completion payload persisted: %+v", completed)
	}
	if completed.CompletedAt == nil {
		t.Fatal("expected completed_at")
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	return store
}
