package cloud

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	return store
}
