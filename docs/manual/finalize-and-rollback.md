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
The Go API does not yet expose `reversible`, `readableFrom` or a lowering operation;
until it does, treat rollback as "before finalize".

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
