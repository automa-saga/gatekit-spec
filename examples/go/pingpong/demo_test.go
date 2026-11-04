package main

import (
	"fmt"
	"strings"
	"testing"
)

// TestRun verifies the rolling-upgrade invariants: nobody ever fails to decode a
// message (safe coexistence), an upgraded peer stays on v1 until finalize (dormant),
// and after finalize both peers speak v2.
func TestRun(t *testing.T) {
	var log []string
	err := Run(func(format string, args ...any) {
		log = append(log, fmt.Sprintf(format, args...))
	})
	if err != nil {
		t.Fatalf("rollout failed (a peer could not read a message): %v", err)
	}

	joined := strings.Join(log, "\n")

	// Coexistence: at least one round shows an upgraded, v2-capable peer still sending
	// v1 before finalize — the dormant window that keeps mixed binaries safe.
	if !strings.Contains(joined, "still dormant → v1") {
		t.Errorf("expected an upgraded-but-dormant peer sending v1 before finalize:\n%s", joined)
	}

	// Find the finalize line; before it every message is v1, after it every message v2.
	finalizeAt := -1
	for i, l := range log {
		if strings.HasPrefix(l, "FINALIZE") {
			finalizeAt = i
		}
	}
	if finalizeAt < 0 {
		t.Fatalf("no finalize step was logged:\n%s", joined)
	}

	for i, l := range log {
		if !strings.HasPrefix(l, "round") {
			continue
		}
		wantV2 := i > finalizeAt
		isV2 := strings.Contains(l, "reads v2")
		if wantV2 && !isV2 {
			t.Errorf("after finalize, expected a v2 message:\n%s", l)
		}
		if !wantV2 && isV2 {
			t.Errorf("before finalize, expected a v1 message (dormant):\n%s", l)
		}
	}
}
