package main

import (
	"context"
	"fmt"

	"github.com/automa-saga/gatekit"
	"github.com/automa-saga/gatekit/migration"
)

// Rollout performs the Appendix E sequence against store, reporting each stage via
// logf. It is a plain function so both main (printing to stdout) and the test
// (capturing lines) can drive it.
//
// The sequence: deploy dormant, compute the plan (the rewrite is run-at-finalize), run the
// local steps (none), wait in the preserve-downgrade window, then finalize —
// which runs the run-at-finalize migration and advances the active version.
func Rollout(ctx context.Context, store *Store, logf func(string, ...any)) error {
	gate, err := gatekit.New(Registry, version{store})
	if err != nil {
		return err
	}
	state := func() string {
		return fmt.Sprintf("active=%d v2=%v records=%v",
			store.Current(), gate.Enabled(RecordFormatV2), store.Formats())
	}

	// Monday — deploy the new binaries, dormant. active stays 5, records stay v1.
	logf("deploy    rolled out new binaries, dormant        | %s", state())

	// Compute the plan 5 -> 6. The rewrite is shared, so it is run at finalize.
	plan, err := migration.NewPlanner(RewriteToV2{store}).Plan(ctx, 5, 6)
	if err != nil {
		return err
	}
	for _, s := range plan.RunAtFinalize() {
		logf("plan      %q is shared -> run at finalize", s.Migration.ID())
	}

	// RunNow does the local steps immediately (none here); never advances active.
	runner := migration.NewStdRunner() // swap gatekit-automa here for crash-safe finalize
	if err := runner.RunNow(ctx, plan); err != nil {
		return err
	}
	logf("runnow    ran local steps (none)             | %s", state())

	// The preserve-downgrade window: rollback to the old binary is still safe here.
	logf("wait      preserve-downgrade window; rollback safe, waiting for readiness")

	// Friday — finalize. Runs the run-at-finalize migration, then advances active to 6.
	if err := runner.Finalize(ctx, plan, store); err != nil {
		return err
	}
	logf("finalize  ran run-at-finalize migration, advanced active     | %s", state())
	return nil
}
