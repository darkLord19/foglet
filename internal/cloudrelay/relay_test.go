package cloudrelay

import (
	"strings"
	"testing"

	"github.com/darkLord19/foglet/internal/cloud"
)

// Branch naming and launch resolution now live behind runner.Launch, and are
// covered in internal/branchname and internal/runner. The relay's own remaining
// job is transport: turning a cloud.Job into a LaunchRequest and a
// CompletePayload.

func TestHandleUnknownJobKind(t *testing.T) {
	r := &Relay{}
	out := r.handleJob(cloud.Job{Kind: "unknown"})
	if out.Success {
		t.Fatal("expected unknown kind to fail")
	}
	if !strings.Contains(out.Error, "unknown job kind") {
		t.Fatalf("unexpected error: %q", out.Error)
	}
}
