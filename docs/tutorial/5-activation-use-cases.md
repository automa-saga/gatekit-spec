# 5. Activation use cases

> **The question:** Lesson 4 turned a feature on by moving the activation version.
> But *who* moves it, *when*, and what if turning it on also means rewriting stored
> data that you cannot un-rewrite? This is where one version axis sometimes becomes
> two.

> **Prerequisites:** [Lesson 4](4-build-and-release-a-feature.md). You need the
> activation version, specs, and the support window. This lesson uses the optional
> **migration/activation** layer.

**You will learn:** when one version axis is enough and when you need two; the
difference between a *host-local* and an *interop* migration; and the two ideas that
make rollback safe — the **one-way line** and the **preserve-downgrade window**.

## Two questions decide everything

Every activation decision comes down to two questions:

1. **Does flipping this feature also change stored or on-the-wire data?** If no, you
   are in the easy world of Lesson 4. If yes, you need a **migration**.
2. **Does the change require other machines to agree before it is safe?** If no, you
   can activate on deploy. If yes, you must *defer* activation until the fleet is
   ready.

The answers sort you into a small number of cases. Let us walk them from simplest to
hardest.

## Case A: Single axis — reuse your release version

The simplest setup: you have no separate protocol, and activating on deploy is fine.
Then you do not need a dedicated counter at all. **Map the activation version onto
your release version** (or a monotonic build number) and activate as you deploy.

```go
// A version source that reports whatever your release/build number maps to.
gate, _ := gatekit.New(Registry, gatekit.StaticVersion(currentBuildActivation()))
if gate.Enabled(catalog.SplitCatalog) { /* new */ } else { /* old */ }
```

This is the right default for a tool or a service you deploy and fully control. The
feature is dormant in the releases before its spec, and live from the release that
crosses it. No migration layer, no finalize — just Lesson 4.

**Use single-axis when:** there is no cross-machine protocol to coordinate, and it is
fine for the behavior to turn on the moment a machine runs the new release.

## Case B: Two axes — a dedicated, deferred counter

Now the harder world. The change affects how machines talk to each other, or it
rewrites data, and you must not flip until the whole fleet is ready. Tying that to
the release version would be wrong: the moment the new release lands on *one*
machine, that machine would start speaking the new protocol while its neighbors
still speak the old one. Split-brain.

So you run a **second axis** — a dedicated activation counter, advanced *deliberately*
and *separately* from the release. This is the **protocol version pattern** from
Lesson 2, and it is exactly what Kafka and CockroachDB do.

The two axes do different jobs:

```
Release version   ──►  "deploy the binaries"   (happens per machine, as they roll)
Activation version──►  "raise the protocol"     (happens ONCE, for the whole fleet,
                                                  after every machine is new enough)
```

**Deploy** moves the release version, machine by machine, new code dormant.
**Finalize** moves the activation version, once, for everyone — and *that* is when
the new behavior wakes up.

**Use two axes when:** the change needs fleet-wide agreement (a wire protocol, a
shared storage format), or you want a rollback-safe staging period between deploy and
activate.

## Migrations, and the one word that changes everything

When activation also transforms data, you attach a **migration** to the activation
version. A migration is a small object with a forward step, a reverse step, and a way
to check whether it still needs to run:

```go
type RewriteReceipts struct{}

func (RewriteReceipts) ID() string          { return "rewrite-receipts-v6" }
func (RewriteReceipts) Description() string  { return "Rewrite receipts into the v6 format" }
func (RewriteReceipts) Version() int         { return 6 }      // activates with activation version 6
func (RewriteReceipts) Scope() migration.Scope { return migration.Interop } // see below

// Applies reads GROUND TRUTH — the actual data — not a log of past runs. So it is
// safe to re-run: once the data is already v6, Applies returns false.
func (RewriteReceipts) Applies(ctx context.Context) (bool, error) {
    return anyReceiptStillInOldFormat(ctx)
}

func (RewriteReceipts) Execute(ctx context.Context) error  { return rewriteAllReceipts(ctx) }
func (RewriteReceipts) Rollback(ctx context.Context) error { return restoreReceiptBackup(ctx) }
```

> **Why `Applies` must read ground truth.** If you tracked "did I run this?" in a
> ledger, a crash halfway through would leave the ledger lying. By asking the data
> itself — "is any receipt still old-format?" — the migration is **idempotent**: safe
> to run again and again, doing nothing once the effect is already present.

## Scope: host-local vs interop

A migration's **scope** decides *when* it is allowed to run. gatekit does not hardwire
the meaning — you classify your own scopes — but it ships two well-understood ones:

> **host-local.** The migration affects only the machine it runs on (converting a
> local cache, say). It is safe to run **as soon as** its activation version activates
> — nobody else is affected.
>
> **interop.** The migration affects agreement *between* machines (a shared format, a
> wire protocol). It must be **held** until a coordinated finalize, or you get
> split-brain.

These map to a **run-policy**:

| Scope | Default run-policy | Runs when |
|-------|--------------------|-----------|
| `host-local` | `run-now` | immediately, at install/activation |
| `interop` | `held` | only at a deliberate `finalize` |
| *(anything unmapped)* | `held` | the safe default — never auto-fires |

You can define your own scopes (by [blast radius](../appendix/glossary.md#blast-radius)
— how much a change can affect — by how deliberate the change is, whatever you need)
and map each to `run-now` or `held`. Any scope you forget to map
falls back to `held` — the conservative choice, because a held migration never fires
on its own.

## Preview before you leap: the plan

Before running anything, gatekit computes a **plan**: exactly which migrations will
run now and which are held, for the move from one version to another. A plan is a
pure preview — it changes nothing — so it is perfect for a `--dry-run`:

```go
planner := migration.NewPlanner(RewriteReceipts{}, WarmLocalCache{})

plan, err := planner.Plan(ctx, from /* 5 */, to /* 6 */)
if err != nil {
    return err
}

for _, step := range plan.RunNow() { /* will run at activation */ }
for _, step := range plan.Held()   { /* waits for finalize */ }
```

The plan includes a migration only if its version is in the
[half-open range](../appendix/glossary.md#half-open-range) `(from, to]` — greater than
`from`, up to and including `to` — **and** its `Applies` returns true.
Already-satisfied migrations are left out, which is why re-running a plan is safe.

## The two execution steps

Running a plan happens in up to two steps, with a runner:

```go
runner := migration.NewStdRunner() // dependency-free; a durable runner exists separately

// 1. Run the host-local (run-now) steps immediately. On any failure, it rolls back
//    the steps already done, in reverse, and does NOT advance the version.
if err := runner.RunNow(ctx, plan); err != nil {
    return err
}

// 2. Later, once the fleet is ready: finalize. This runs the held (interop) steps,
//    then advances the active version via the store. This is the one-way line.
if err := runner.Finalize(ctx, plan, store); err != nil {
    return err // version did NOT advance; held steps were rolled back
}
```

- **`RunNow`** does the safe, local work at install time. It never advances the
  active version.
- **`Finalize`** does the coordinated work and *then* advances the active version to
  the target. The version moves **only if every held step succeeds.**

## The one-way line

Here is the sharpest idea in the whole course.

> **Term — the one-way line (finalize).** Finalize is the point of no easy return.
> Before finalize runs any migration, rolling back to the old binary is clean. Once
> finalize has run a migration that cannot be perfectly reversed, **rollback is no
> longer guaranteed** — the old binary may not be able to read what the new one
> wrote.

```
      DEPLOY (new code, dormant)                 FINALIZE
  ─────────────────────────────────────┬──────────────────────────►
            rollback is SAFE here       │   rollback NOT guaranteed
       (preserve-downgrade window)      │        (one-way line)
                                        ▲
                               you cross this deliberately,
                                 once the fleet is ready
```

The span to the **left** of the line is the **preserve-downgrade window** — the
deliberate pause between deploying the new code and finalizing. You stay there as
long as you want: watch metrics, bake the release, keep the door open to roll back to
the old binary. CockroachDB calls this `preserve_downgrade_option`; Kafka gets it by
pinning the protocol version. In gatekit it is simply "you have not called finalize
yet."

## Reversibility: when *can* you go back?

Crossing the line is only dangerous if the migration is irreversible. gatekit is
precise about this:

> An activation step is **reversible** if it has **no migration**, or if **every**
> migration in it provides a working `Rollback` *and* writes nothing that an older
> supported version could not read (the **write floor**).

- For a **reversible** step, you may lower the active version again — the runner runs
  the reverse migrations.
- For an **irreversible** step, lowering the active version across it is **refused**
  with a typed error. gatekit will not let you pretend you can go back when you
  cannot.

> **Term — write floor.** A migration must never write data in a format too new for
> the oldest supported version (`min`) to read. Honoring the floor is what keeps
> "lower the active version" safe *within* the support window. Crossing *below* `min`
> is never supported — and the run invariant forbids running there anyway.

## Putting the cases together

| Situation | Axes | Migration scope | Activation |
|-----------|------|-----------------|------------|
| Tool you deploy & control; no shared protocol | single (reuse release) | none | on deploy |
| Local format change, no coordination needed | single or two | `host-local` → run-now | at install |
| Shared wire/storage format; fleet must agree | two | `interop` → held | at `finalize` |
| You want a rollback-safe staging pause | two | either, deferred | at `finalize`, after the preserve-downgrade window |

## A saga underneath

The execution model — run steps in order, and on failure compensate the completed
ones in reverse — is a **saga**: a sequence of steps each with an undo, so a partial
failure can be unwound. gatekit's default `StdRunner` does this sequentially; a
separate, durable, crash-resumable runner
([gatekit-automa](https://github.com/automa-saga/gatekit-automa)) implements the same
interface for systems that need to survive a crash mid-migration. Your call sites do
not change when you swap runners.

## Check yourself

- Your change rewrites a shared storage format every node reads. One axis or two?
  Which scope? *(Two axes; `interop` (held) — the fleet must agree before the format
  changes.)*
- Why must `Applies` read the data instead of a "have I run this?" record? *(So the
  migration is idempotent and a crash can't leave a lying ledger.)*
- You deployed the new binaries an hour ago but have not finalized. Can you still
  roll back to the old binary? *(Yes — you are in the preserve-downgrade window,
  before the one-way line.)*
- When will gatekit refuse to lower the active version? *(Across an irreversible step
  — one whose migration has no working rollback or violates the write floor.)*

## Further reading

- [gatekit activation spec §5–7](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md)
  — activation modes, finalize, reversibility, and the write floor.
- [gatekit migration spec](https://github.com/automa-saga/gatekit-spec/blob/main/migration-spec.md)
  — the migration contract, plan computation, and the saga execution model.
- [Appendix B: conformance fixtures](../appendix/conformance.md) — the `planning/`
  fixtures that pin down plan computation.

**Next:** [6. Rolling-upgrade thinking](6-rolling-upgrade-thinking.md)
