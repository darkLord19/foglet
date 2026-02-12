package state

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := bytes.Repeat([]byte{0x01}, masterKeySize)
	plain := []byte("ghp_example_token")

	ciphertext, err := encrypt(githubPATKey, plain, key)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	got, err := decrypt(githubPATKey, ciphertext, key)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}

	if !bytes.Equal(got, plain) {
		t.Fatalf("decrypted value mismatch: got=%q want=%q", got, plain)
	}
}

func TestDecryptFailsOnAADMismatch(t *testing.T) {
	key := bytes.Repeat([]byte{0x02}, masterKeySize)
	plain := []byte("ghp_example_token")

	ciphertext, err := encrypt(githubPATKey, plain, key)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}

	if _, err := decrypt("another_secret", ciphertext, key); err == nil {
		t.Fatal("expected decrypt error for mismatched secret name")
	}
}

func TestLoadOrCreateMasterKey(t *testing.T) {
	tmp := t.TempDir()
	keyPath := filepath.Join(tmp, "master.key")

	key1, err := loadOrCreateMasterKey(keyPath)
	if err != nil {
		t.Fatalf("first load/create failed: %v", err)
	}
	if len(key1) != masterKeySize {
		t.Fatalf("unexpected key size: got %d want %d", len(key1), masterKeySize)
	}

	key2, err := loadOrCreateMasterKey(keyPath)
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}
	if !bytes.Equal(key1, key2) {
		t.Fatal("expected same key on second load")
	}

	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("stat key failed: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("unexpected key permissions: got %o want 600", perm)
	}
}
