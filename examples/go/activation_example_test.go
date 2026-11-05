package examples_test

import (
	"context"
	"fmt"

	"github.com/automa-saga/gatekit"
	"github.com/automa-saga/gatekit/migration"
)

// Example_runInvariant shows the triple {min, max, active} and the startup run
// invariant min <= active <= max (Lesson 2; Lesson 6, Rule 3).
func Example_runInvariant() {
	// max = 6; with the default window W = 1 and firstVersion = 5, min = 5.
	gate, _ := gatekit.New(catalogRegistry, gatekit.StaticVersion(6))
	ev := gate.EffectiveVersion()
	fmt.Printf("min=%d max=%d active=%d\n", ev.Min, ev.Max, ev.Active)
	fmt.Println(gate.Check() == nil) // 6 is in [5,6]: ok

	// A stale binary stuck at active = 4 is below min and must refuse to run.
	stale, _ := gatekit.New(catalogRegistry, gatekit.StaticVersion(4))
	fmt.Println(stale.Check() != nil) // refuses: out of range
	// Output:
	// min=5 max=6 active=6
	// true
	// true
}

// memStore is a trivial in-memory migration.Store (the persisted active version).
type memStore struct{ v int }

func (s *memStore) Current() int                { return s.v }
func (s *memStore) SetActive(version int) error { s.v = version; return nil }

// demoMigration is a no-op Migration used to illustrate planning and finalize. A
// real migration's Applies would read ground truth and Execute/Rollback would
// transform/restore data (Lesson 5).
type demoMigration struct {
	id, desc string
	version  int
	scope    migration.Scope
	applies  bool
}

func (m demoMigration) ID() string                            { return m.id }
func (m demoMigration) Description() string                   { return m.desc }
func (m demoMigration) Version() int                          { return m.version }
func (m demoMigration) Scope() migration.Scope                { return m.scope }
func (m demoMigration) Applies(context.Context) (bool, error) { return m.applies, nil }
func (m demoMigration) Execute(context.Context) error         { return nil }
func (m demoMigration) Rollback(context.Context) error        { return nil }

// Example_plan previews which migrations run now (local) and which are run-at-finalize for
// finalize (shared) when moving from activation 5 to 6 (Lesson 5).
func Example_plan() {
	warm := demoMigration{id: "warm-cache", desc: "warm local cache", version: 6, scope: migration.Local, applies: true}
	reshard := demoMigration{id: "reshard", desc: "reshard shared index", version: 6, scope: migration.Shared, applies: true}

	planner := migration.NewPlanner(warm, reshard)
	plan, err := planner.Plan(context.Background(), 5, 6)
	if err != nil {
		panic(err)
	}

	for _, s := range plan.RunNow() {
		fmt.Printf("run-now: %s\n", s.Migration.ID())
	}
	for _, s := range plan.RunAtFinalize() {
		fmt.Printf("run-at-finalize:    %s\n", s.Migration.ID())
	}
	// Output:
	// run-now: warm-cache
	// run-at-finalize:    reshard
}

// Example_finalize runs the run-at-finalize (shared) steps and advances the active version —
// the one-way line (Lesson 5).
func Example_finalize() {
	store := &memStore{v: 5}
	reshard := demoMigration{id: "reshard", desc: "reshard shared index", version: 6, scope: migration.Shared, applies: true}

	plan, _ := migration.NewPlanner(reshard).Plan(context.Background(), 5, 6)
	runner := migration.NewStdRunner()

	// RunNow does the local steps immediately (none here) and never advances.
	if err := runner.RunNow(context.Background(), plan); err != nil {
		panic(err)
	}
	fmt.Println("active before finalize:", store.Current())

	// Finalize runs the run-at-finalize steps, then advances active to plan.To.
	if err := runner.Finalize(context.Background(), plan, store); err != nil {
		panic(err)
	}
	fmt.Println("active after finalize:", store.Current())
	// Output:
	// active before finalize: 5
	// active after finalize: 6
}
