package main

import (
	"context"
	"fmt"

	"github.com/automa-saga/gatekit/migration"
)

// Rollout drives the whole record-format upgrade as a real sequence of Put and Get
// calls, reporting each stage via logf. It is a plain function so both main (printing
// to stdout) and the test (capturing lines) can drive it.
//
// The shape to watch:
//
//  1. While active = 5, RecordFormatV2 is dormant, so Put writes v1.
//  2. The rewrite migration is Shared, so the planner holds it for finalize — nothing
//     changes at plan time.
//  3. finalize runs the migration (every record v1 -> v2) and advances active to 6.
//  4. With active = 6 the same Put now writes v2, and Get still reads every record —
//     old and new format alike.
func Rollout(ctx context.Context, store *Store, kv *KV, logf func(string, ...any)) error {
	state := func() string {
		return fmt.Sprintf("active=%d v2=%v formats=%v",
			store.Current(), kv.gate.Enabled(RecordFormatV2), store.Formats())
	}

	// 1. Deploy dormant. The new binary is running, but active is still 5, so writes
	// land in v1 — a rollback to the old binary could still read them.
	kv.Put("alpha", "1")
	kv.Put("beta", "2")
	logf("put       alpha, beta while dormant           | %s", state())

	// 2. Plan 5 -> 6. The rewrite is Shared, so it is HELD for finalize.
	plan, err := migration.NewPlanner(RewriteToV2{store}).Plan(ctx, store.Current(), 6)
	if err != nil {
		return err
	}
	for _, s := range plan.RunAtFinalize() {
		logf("plan      %q is shared -> HELD for finalize", s.Migration.ID())
	}

	// 3. RunNow does the host-local steps immediately (none here); never advances active.
	runner := migration.NewStdRunner() // swap gatekit-automa here for crash-safe finalize
	if err := runner.RunNow(ctx, plan); err != nil {
		return err
	}
	logf("runnow    ran host-local steps (none)          | %s", state())

	// The preserve-downgrade window: rollback to the old binary is still safe here,
	// because every record is still v1.
	logf("wait      preserve-downgrade window; rollback still safe")

	// 4. Finalize. Runs the held migration (rewrites alpha, beta to v2), then advances
	// active to 6. From here, the old binary can no longer read the data.
	if err := runner.Finalize(ctx, plan, store); err != nil {
		return err
	}
	logf("finalize  migrated records, advanced active    | %s", state())

	// 5. active is now 6, so the same Put writes v2.
	kv.Put("gamma", "3")
	logf("put       gamma after finalize                 | %s", state())

	// 6. Read every record back. Get decodes whichever format it was stored in and
	// verifies the v2 checksum — so v1 and v2 records are served alike.
	for _, key := range []string{"alpha", "beta", "gamma"} {
		value, _, err := kv.Get(key)
		if err != nil {
			return fmt.Errorf("get %q: %w", key, err)
		}
		stored, _ := store.get(key)
		logf("get       %-5s = %-3s (stored %s)", key, value, formatOf(stored))
	}
	return nil
}
