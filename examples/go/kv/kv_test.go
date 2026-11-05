package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// TestRollout checks the real Put/Get behavior across the upgrade: Put writes v1 while
// dormant, the migration is held until finalize, finalize rewrites every record to v2
// and advances active, and after finalize Put writes v2 while Get still reads every
// record.
func TestRollout(t *testing.T) {
	store := NewStore()
	kv, err := NewKV(store)
	if err != nil {
		t.Fatal(err)
	}

	var log []string
	if err := Rollout(context.Background(), store, kv, func(format string, args ...any) {
		log = append(log, fmt.Sprintf(format, args...))
	}); err != nil {
		t.Fatal(err)
	}

	// End state: finalize ran, active advanced, every record is v2.
	if got := store.Current(); got != 6 {
		t.Errorf("active after finalize = %d, want 6", got)
	}
	for _, r := range store.Records() {
		if r.Format() != "v2" {
			t.Errorf("record %q format = %q, want v2", r.Key, r.Format())
		}
	}

	joined := strings.Join(log, "\n")
	// The migration must be held at plan time (Shared), not run at deploy.
	if !strings.Contains(joined, "HELD for finalize") {
		t.Errorf("expected the migration to be reported HELD at plan time:\n%s", joined)
	}
	// The first write, while dormant, must land in v1.
	if !strings.Contains(log[0], "formats=[v1 v1]") {
		t.Errorf("put while dormant should store v1, got: %s", log[0])
	}
}

// TestPutFormatFollowsGate is the unit-level version of the same point: the format Put
// chooses is driven by the gate, which resolves against the store's live active
// version. Advancing active (without rebuilding the gate or the KV) flips Put from v1
// to v2.
func TestPutFormatFollowsGate(t *testing.T) {
	store := NewStore()
	kv, err := NewKV(store)
	if err != nil {
		t.Fatal(err)
	}

	if r := kv.Put("k", "1"); r.Format() != "v1" {
		t.Fatalf("at active=5 Put format = %q, want v1", r.Format())
	}

	store.active = 6 // what finalize does
	if r := kv.Put("k", "2"); r.Format() != "v2" {
		t.Fatalf("at active=6 Put format = %q, want v2", r.Format())
	}

	// Get reads the (now v2) record back and verifies its checksum.
	value, ok, err := kv.Get("k")
	if err != nil || !ok || value != "2" {
		t.Fatalf("Get after v2 write = (%q, %v, %v), want (\"2\", true, nil)", value, ok, err)
	}
}

// TestGetDetectsCorruption confirms the v2 checksum is actually checked: a record whose
// stored bytes are tampered with fails to decode instead of returning bad data.
func TestGetDetectsCorruption(t *testing.T) {
	store := NewStore()
	store.active = 6
	kv, err := NewKV(store)
	if err != nil {
		t.Fatal(err)
	}
	kv.Put("k", "hello")

	// Corrupt the stored value without fixing the checksum.
	store.records[0].Stored = strings.Replace(store.records[0].Stored, "hello", "hallo", 1)

	if _, _, err := kv.Get("k"); err == nil {
		t.Fatal("Get should have reported a checksum mismatch, got nil error")
	}
}
