# Implementing activation

**The goal.** gatekit tells you *whether a feature is on* at a given activation
version, but it deliberately leaves two things to you: **when** `active` advances,
and **where** it is stored. This chapter wires gatekit's pieces — a gate, a planner,
and a runner — together with a little of your own code, so the "ship dormant, then
activate later" lifecycle runs end to end in a real, restartable program — whether it
runs on one node or a fleet.

**How — three pieces you add around gatekit,** built one per section below:

1. **A state store** (§1) owns the `active` value: persistent, forward-only, written
   atomically.
2. **A startup routine** (§2) runs on every boot — enforce the run invariant, catch
   drift, run the migrations that are safe now, and decide whether to activate.
3. **A finalize command** (§3–4) is the deliberate step — driven by an operator or a
   trigger — that runs the deferred migrations and advances `active` for the fleet;
   §5 adds an optional auto-finalize daemon.

The code below compiles and runs against the Go API. It is a starting point, not a
library: adapt the state location, locking, and logging to your product.

> **Want to see it running first?** The capstone —
> [Appendix E: a worked rollout](/appendix/worked-example.md) and the runnable
> [`examples/go/kv`](https://github.com/automa-saga/gatekit-spec/tree/main/examples/go/kv)
> — is a complete rollout of these pieces on one concrete tool. This chapter is the
> how-to for building the same wiring into *your own* program.

## What you are building

Those pieces exist to keep three guarantees — they are what §1–4 protect:

- **Idempotent.** Running activation twice has the same effect as running it once.
- **Safe.** `active` advances only after every run-at-finalize migration has succeeded, and the
  write is atomic.
- **One-way.** Once activated, `active` never goes back down. An older binary then
  refuses to start, because `active` is above its `Max`.

## 1. A state store

`migration.Store` is two methods. Make the implementation strict: fail closed on a
bad file, move forward only, and write atomically.

```go
package activation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type state struct {
	Schema int `json:"schema"`
	Active int `json:"active"`
}

// FileStore is a migration.Store backed by a JSON state file. It fails closed on a
// bad file and only ever moves the active version forward.
type FileStore struct {
	mu     sync.Mutex
	path   string
	max    int // the binary's Max; the store never goes above it
	active int
}

// OpenFileStore loads the state file and validates it.
func OpenFileStore(path string, max int) (*FileStore, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read activation state: %w", err)
	}
	var s state
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse activation state: %w", err)
	}
	if s.Schema != 1 {
		return nil, fmt.Errorf("activation state: unsupported schema %d", s.Schema)
	}
	if s.Active < 0 || s.Active > max {
		return nil, fmt.Errorf("activation state: active %d is outside 0..%d", s.Active, max)
	}
	return &FileStore{path: path, max: max, active: s.Active}, nil
}

// Current implements migration.Store.
func (s *FileStore) Current() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// SetActive implements migration.Store: idempotent, forward-only, atomic.
func (s *FileStore) SetActive(v int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case v == s.active:
		return nil // already there: finalize is safe to re-run
	case v < s.active:
		return fmt.Errorf("refusing to lower activation from %d to %d", s.active, v)
	case v > s.max:
		return fmt.Errorf("activation %d is above this binary's max %d", v, s.max)
	}
	if err := writeAtomic(s.path, state{Schema: 1, Active: v}); err != nil {
		return err
	}
	s.active = v
	return nil
}

// writeAtomic writes to a temp file in the same directory, syncs it, then renames
// it over the target, so a crash never leaves a half-written state file.
func writeAtomic(path string, st state) error {
	b, err := json.Marshal(st)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".activation-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
```

Points to copy:

- `SetActive` to the current value is a **no-op**, so a second finalize is safe.
- Lowering is **refused**, and so is going above the binary's `Max`.
- The write goes to a temp file, is synced, then renamed, so a crash never leaves a
  half-written file.
- `OpenFileStore` fails closed. A missing, unparseable, or out-of-range file stops
  the process instead of silently defaulting to `0` or `Max`.

Keep the file root-owned. Hand-editing it is outside the safety guarantees; step 2
detects the common mistakes.

## 2. What to do at startup

Run this once the new binary is up (and again on every start). It does four things:

1. **Run invariant.** `gate.Check()` refuses to run when `active` is outside
   `[min, max]`. This is what stops an old binary from running after activation.
2. **Drift check.** No migration at or below `active` should still apply. If one
   does, `active` was edited or raised without the work being done, or a finalize
   was cut short. Refuse to run and name the migration.
3. **Run-now steps.** Plan from `active` to this binary's `Max` and run the
   local steps. `active` does not move.
4. **Decide whether to activate now** (see the next section).

**What "drift" means.** The stored `active` is a *claim* — that every migration at
or below it has already run. The drift check tests that claim against ground truth:
`Plan(ctx, -1, active)` returns any migration in the range `(-1, active]` whose
`Applies()` is still true, i.e. whose work is *not* actually done. An empty result
means the recorded version matches reality; a non-empty one means the version and
the data have **diverged**. The classic trigger is restoring the database from an
older backup without resetting `active`: the records are old again, but `active`
still says they were migrated — so the binary would run as if the new format were in
place over old data. Failing fast, and naming the migration, is safer than operating
on that lie. (This is the data-level complement to the run invariant in step 1:
step 1 checks `active` is in a supportable *range*; the drift check verifies it is
actually *true*.)

```go
package activation

import (
	"context"
	"fmt"

	"github.com/automa-saga/gatekit"
	"github.com/automa-saga/gatekit/migration"
)

// Outcome says where an upgrade ended up.
type Outcome int

const (
	Activated Outcome = iota // active advanced to the target
	Pending                  // run-at-finalize steps remain; waiting for finalize
)

// OnStartup is called once the new binary is running. synchronized reports whether
// the whole fleet is upgrading together (so there is no mixed-version window).
func OnStartup(
	ctx context.Context,
	gate *gatekit.FeatureGate,
	store *FileStore,
	migs []migration.Migration,
	runner migration.Runner,
	synchronized bool,
) (Outcome, error) {
	// 1. Run invariant: refuse to run outside [min, max].
	if err := gate.Check(); err != nil {
		return Pending, err
	}

	planner := migration.NewPlanner(migs...)
	active := store.Current()

	// 2. Drift check: nothing at or below `active` should still need to run. If one
	// does, the state was edited, restored from a backup, or a finalize was cut short.
	drift, err := planner.Plan(ctx, -1, active)
	if err != nil {
		return Pending, err
	}
	if len(drift.Steps) > 0 {
		return Pending, fmt.Errorf("activation %d recorded but %q still applies; refusing to run",
			active, drift.Steps[0].Migration.ID())
	}

	// 3. Plan the move to this binary's max and run the local steps.
	target := int(gate.MaxActivation())
	plan, err := planner.Plan(ctx, active, target)
	if err != nil {
		return Pending, err
	}
	if err := runner.RunNow(ctx, plan); err != nil {
		return Pending, err
	}

	// 4. Activate now if nothing needs fleet agreement, or the whole fleet moves together.
	if synchronized || len(plan.RunAtFinalize()) == 0 {
		if err := runner.Finalize(ctx, plan, store); err != nil {
			return Pending, err
		}
		return Activated, nil
	}
	return Pending, nil // run-at-finalize steps wait for an explicit finalize
}
```

After `Activated`, rebuild the gate (or use a `VersionSource` that reads the store),
because the gate you passed was built from the old value.

## 3. Synchronized vs. rolling upgrades

The rule in `OnStartup` is:

> Activate now if the whole fleet upgrades together, **or** if nothing is deferred.
> Otherwise wait for an explicit finalize.

| Upgrade | Run-at-finalize steps? | Result |
|---|---|---|
| Synchronized (all nodes together) | any | Activate immediately. There is no mixed-version window. |
| Rolling (one node at a time) | none | Activate immediately. Nothing needs fleet agreement. |
| Rolling | some | **Pending.** The node runs the new binary at the old `active` until finalize. |

The empty-plan test comes from the migrations themselves (`Scope` and `Applies`), so
nobody has to remember a per-release flag. While a rolling upgrade is pending,
nodes at N and N+1 coexist inside the support window `W`; a node that restarts just
resumes at the same pending state.

## 4. The finalize command

When the operator is ready (every node is on the new binary), a command computes
the plan and finalizes it. Always offer a preview first.

```go
func Finalize(ctx context.Context, store *FileStore, migs []migration.Migration,
	runner migration.Runner, target int, dryRun bool) error {
	plan, err := migration.NewPlanner(migs...).Plan(ctx, store.Current(), target)
	if err != nil {
		return err
	}
	fmt.Print(plan.Report()) // what runs now, what is deferred
	if dryRun {
		return nil
	}
	return runner.Finalize(ctx, plan, store) // run-at-finalize steps, then SetActive(target)
}
```

Running it again after success finds an empty plan and a no-op `SetActive`.
Take a lock around it so a daemon and an operator can't race, and log the old and new
`active`.

## 5. Automatic finalize (optional)

A daemon can finalize on its own once the upgrade has proven itself, for example
after a healthy soak period. Keep it conservative:

- Auto-finalize only local or explicitly "safe" migrations; leave irreversible
  ones to the operator.
- Wait out the soak window first. Deferring is what gives you the rollback window,
  so don't finalize at download time.
- Use the durable runner (`gatekit-automa`) if the daemon may restart mid-finalize.
  Your `Execute` and `Rollback` must then be idempotent.
- For a multi-node fleet, per-node auto-finalize risks a mixed-activation fleet. The
  coordinated "every node is ready" check is reserved in the
  [activation spec](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md),
  so build that coordination yourself or keep finalize operator-driven.

## What this does not cover

- **Lowering `active`.** The spec permits it across reversible steps, but the Go API
  has no operation for it, and this design deliberately omits it.
- **Fleet readiness.** Not yet specified; see above.

Related: [Activation](/manual/activation.md) ·
[Migrations](/manual/migrations.md) ·
[Finalize & rollback](/manual/finalize-and-rollback.md) ·
[Appendix E: a worked rollout](/appendix/worked-example.md) (the runnable capstone)
