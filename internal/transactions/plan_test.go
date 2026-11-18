package transactions

import (
	"testing"
	"time"

	"sysentra/internal/security"
)

func TestPlanCommitRejectsExpiryAndStateDrift(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	plan := Plan{Request: security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Install, Packages: []string{"firefox"}}, StateFingerprint: "state-a", ExpiresAt: now.Add(time.Minute)}
	if err := plan.ValidateCommit(now, "state-a"); err != nil {
		t.Fatal(err)
	}
	if err := plan.ValidateCommit(now.Add(2*time.Minute), "state-a"); err == nil {
		t.Fatal("expired plan was accepted")
	}
	if err := plan.ValidateCommit(now, "state-b"); err == nil {
		t.Fatal("drifted plan was accepted")
	}
}
