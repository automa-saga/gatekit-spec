# Migrations

A **migration** is a one-time, idempotent transformation tied to an activation
version. It lives in the `migration` package:

```go
import "github.com/automa-saga/gatekit/migration"
```

## Implement the interface

```go
type Migration interface {
	ID() string                              // stable across releases
	Description() string
	Version() int                            // activation version it activates with
	Scope() migration.Scope
	Applies(ctx context.Context) (bool, error)
	Execute(ctx context.Context) error
	Rollback(ctx context.Context) error
}
```

Guidance for each method:

- **`Applies`** reads ground truth (the actual data), not a ledger of what ran. That
  makes the migration safe to re-run and the plan idempotent.
- **`Execute`** and **`Rollback`** must be idempotent, especially with the durable
  runner, which re-runs an interrupted step after a crash.
- **`Rollback`** is best-effort and is used when a later step fails before finalize.

## Scope decides timing

| Scope | Default policy | Meaning |
|---|---|---|
| `migration.HostLocal` | **run now** | Affects only the local host; runs when its activation version activates. |
| `migration.Interop` | **held** | Affects cross-host agreement; held for a coordinated `finalize`. |

Scopes are not a closed set. Map your own scopes to a policy
(`migration.PolicyRunNow` or `migration.PolicyHeld`):

```go
planner := migration.NewPlanner(migs...).WithScopePolicy(migration.ScopePolicy{
	"cache": migration.PolicyRunNow,
})
```

Your entries merge over the default. A scope that nothing maps resolves to
**held**, the conservative choice, because a held step never fires automatically.

## Compute a plan

```go
plan, err := migration.NewPlanner(migs...).Plan(ctx, store.Current(), target)
```

`Plan` selects migrations whose version is in the half-open range `(from, to]` that
still `Applies`, ordered by version then ID. It is a pure value, computed on demand
and never stored.

```go
for _, s := range plan.RunNow() { fmt.Println("run-now:", s.Migration.ID()) }
for _, s := range plan.Held()   { fmt.Println("held:   ", s.Migration.ID()) }
```

## Preview before running

`plan.Report()` returns a `PlanReport` that marshals to JSON and renders to text,
for a `--plan` or `--dry-run` flag:

```go
fmt.Print(plan.Report())
// Plan: activation 5 -> 6
//   Run now (host-local):
//     - warm-cache [host-local] warm local cache
//         reason: host-local: runs now on activation
//   Held for finalize (interop):
//     - reshard [interop] reshard shared index
//         reason: interop: held for finalize
```

## The Store

Finalize persists the new active version through a `Store` you provide:

```go
type Store interface {
	Current() int
	SetActive(version int) error
}
```

Wire the same value into the gate so resolution and planning agree:
`gatekit.StaticVersion(store.Current())`.

Normative rules: [migration spec](https://github.com/automa-saga/gatekit-spec/blob/main/migration-spec.md).
Next: [Finalize & rollback](/manual/finalize-and-rollback.md).
