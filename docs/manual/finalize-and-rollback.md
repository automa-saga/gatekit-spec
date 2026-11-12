# Finalize & rollback

A **runner** executes a plan. Two methods, two moments:

| Method | When | Does | Advances `active`? |
|---|---|---|---|
| `RunNow(ctx, plan)` | At install / immediate activation | Runs the **run-now** (local) steps in plan order. | **No** |
| `Finalize(ctx, plan, store)` | When the operator is ready | Runs the **run-at-finalize** (shared) steps in order, then calls `store.SetActive(plan.To)`. | **Yes**, only if every run-at-finalize step succeeded |

```go
runner := migration.NewStdRunner()

if err := runner.RunNow(ctx, plan); err != nil {
	return err
}
// ... later, once the whole fleet runs the new binary ...
if err := runner.Finalize(ctx, plan, store); err != nil {
	return err
}
```

## Failure behavior

On the first failing step the runner calls `Rollback` on the steps it already ran,
in reverse order (best-effort), and returns the error. If a run-at-finalize step fails during
`Finalize`, `active` is left unchanged.

## Finalize is the one-way line

A migration may be irreversible, so **binary rollback is only guaranteed before
`Finalize` has run any migration**. Plan for that:

1. Roll the new binary out to every machine with the activation held back.
2. Verify the fleet.
3. Only then run `Finalize`.

A step is **reversible** only if it has no migration, or every migration is declared
`reversible` (the default is irreversible) and writes nothing an older version inside
the window cannot read (the write floor, declared with `readableFrom`). The spec
requires lowering `active` to be its own operation, refused when the target is below
`min` or when it crosses an irreversible step
([migration spec §3.5](https://github.com/automa-saga/gatekit-spec/blob/main/migration-spec.md)).
The Go API exposes this since **gatekit v0.6.0** — `Reversible`, `ReadableFrom`, `WithMin`,
`PlanLower`, and `Lower` — shown next.

## A reversible, floor-aware migration

> **Available since gatekit v0.6.0.** The `Reversible` / `Readable` interfaces and the
> `WithMin`, `PlanLower`, and `Lower` API shown here landed in the activation-rollback /
> write-floor release. The behavior is specified in migration spec §3.5–3.6 and activation
> spec §6–7, and pinned by the `rollback/` and `floor/` conformance fixtures. The durable,
> crash-resumable version is in
> [gatekit-automa](https://github.com/automa-saga/gatekit-automa).

A migration opts in to a safe downgrade by implementing two optional interfaces alongside
`Migration`:

- **`Reversible() bool`** — `Rollback` is a true inverse of `Execute`, so lowering `active`
  across this step is allowed. The default is **false**; gatekit never infers it from the
  mere presence of a `Rollback` method.
- **`ReadableFrom() int`** — the oldest activation version that can read what `Execute`
  writes. The default is **0** (every version). Declaring it turns on the write floor for
  this step.

```go
// RewriteToV2 rewrites records from the v1 format to v2. v2 is readable only from
// activation version 6 onward, and the rewrite can be undone — so it declares both.
type RewriteToV2 struct{ Store *Store }

func (RewriteToV2) ID() string             { return "rewrite-records-v2" }
func (RewriteToV2) Description() string    { return "Rewrite all records from v1 to v2" }
func (RewriteToV2) Version() int           { return 6 }
func (RewriteToV2) Scope() migration.Scope { return migration.Shared }

func (m RewriteToV2) Applies(context.Context) (bool, error) { /* any record still v1? */ }
func (m RewriteToV2) Execute(context.Context) error         { /* v1 -> v2 */ }
func (m RewriteToV2) Rollback(context.Context) error        { /* v2 -> v1 */ }

// Optional — the rollback / write-floor additions:
func (RewriteToV2) Reversible() bool  { return true } // Rollback is a real inverse
func (RewriteToV2) ReadableFrom() int { return 6 }    // v2 is readable only from version 6
```

Wire the floor from the gate's `min`, so one support window drives both resolution and
planning:

```go
min := gate.EffectiveVersion().Min
plan, err := migration.NewPlanner(migs...).WithMin(min).Plan(ctx, store.Current(), target)
// If a migration in range declares ReadableFrom but you forget WithMin, Plan refuses with
// *UnenforcedFloorError rather than silently skip the floor. A negative min is rejected
// with *InvalidMinError.
```

With the floor on, a step whose `ReadableFrom` is above `min` is **blocked**: `RunNow` and
`Finalize` refuse to run anything and return a `*WriteFloorError`. That is what forces the
expand/contract split — you cannot write v2 until `min` has reached 6.

To retreat, lower `active` with its own plan — data first, active last:

```go
lp, err := planner.PlanLower(store.Current(), target, min) // e.g. 6 -> 5
if err != nil {
	var irr *migration.IrreversibleStepError
	if errors.As(err, &irr) {
		// an in-range step is not declared Reversible — a clean retreat is not possible
	}
	return err // also *BelowMinError (target below min) or *InvalidMinError
}
if err := runner.Lower(ctx, lp, store); err != nil { // Rollback each step, then SetActive
	return err
}
```

`PlanLower` refuses a target below `min` or a plan that crosses an irreversible step, and
`Lower` is idempotent: if it fails partway, run it again to finish the retreat.

## Crash-safe finalize

`StdRunner` is sequential and dependency-free. If a process crash mid-finalize is a
real risk, use the durable runner, which journals progress and resumes:

```go
import gkautoma "github.com/automa-saga/gatekit-automa"

runner := gkautoma.NewRunner(gkautoma.WithJournal("/var/lib/app/finalize.journal"))
_ = runner.RunNow(ctx, plan)
_ = runner.Finalize(ctx, plan, store)
```

It implements the same `migration.Runner` interface, so it is a drop-in
replacement. Because an interrupted step is re-run, your `Execute` and `Rollback`
**must** be idempotent.

Design rules for the whole flow: [Lesson 6](/tutorial/6-rolling-upgrade-thinking.md).
End to end: [Appendix E](/appendix/worked-example.md).
