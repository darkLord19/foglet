package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFogHomeFromEnv(t *testing.T) {
	t.Setenv(fogHomeEnv, "/tmp/fog-home")
	got, err := FogHome()
	if err != nil {
		t.Fatalf("FogHome returned error: %v", err)
	}
	if got != "/tmp/fog-home" {
		t.Fatalf("FogHome mismatch: got %q want %q", got, "/tmp/fog-home")
	}
}

func TestManagedReposDir(t *testing.T) {
	got := ManagedReposDir("/tmp/fog-home")
	want := filepath.Join("/tmp/fog-home", "repos")
	if got != want {
		t.Fatalf("managed repos dir mismatch: got %q want %q", got, want)
	}
}

func TestFogHomeDefault(t *testing.T) {
	t.Setenv(fogHomeEnv, "")
	userHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("user home error: %v", err)
	}

	got, err := FogHome()
	if err != nil {
		t.Fatalf("FogHome returned error: %v", err)
	}
	want := filepath.Join(userHome, ".fog")
	if got != want {
		t.Fatalf("FogHome mismatch: got %q want %q", got, want)
	}
}
