package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestRollout verifies the capstone's key invariants: the migration is held at plan
// time, nothing changes until finalize, and finalize advances the active version and
// rewrites every record to v2.
func TestRollout(t *testing.T) {
	store := NewStore()
	var log []string
	err := Rollout(context.Background(), store, func(format string, args ...any) {
		log = append(log, fmt.Sprintf(format, args...))
	})
	if err != nil {
		t.Fatal(err)
	}

	// End state: finalize ran, active advanced, every record is v2.
	if got := store.Current(); got != 6 {
		t.Errorf("active after finalize = %d, want 6", got)
	}
	for _, r := range store.Records {
		if r.Format != "v2" {
			t.Errorf("record %q format = %q, want v2", r.Key, r.Format)
		}
	}

	joined := strings.Join(log, "\n")
	// The migration must be held at plan time (interop), not run at deploy.
	if !strings.Contains(joined, "HELD for finalize") {
		t.Errorf("expected the migration to be reported HELD at plan time:\n%s", joined)
	}
	// At deploy, before finalize, records must still be v1 (dormant).
	if !strings.Contains(log[0], "records=[v1 v1]") {
		t.Errorf("deploy stage should show records v1, got: %s", log[0])
	}
}
