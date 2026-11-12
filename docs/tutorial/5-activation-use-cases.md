# 5. Activation use cases

> **The question:** Lesson 4 turned a feature on by moving the activation version.
> But *who* moves it, *when*, and what if turning it on also means rewriting stored
> data that you cannot un-rewrite? This is where one version axis sometimes becomes
> two.

> **Prerequisites:** [Lesson 4](/tutorial/4-build-and-release-a-feature.md). You need the
> activation version, specs, and the support window. This lesson uses the optional
> **migration/activation** layer.

**You will learn:** when one version axis is enough and when you need two; the
difference between a *local* and an *shared* migration; and the two ideas that
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
                                                  after every machine is new enough;
                                                  the run-at-finalize migration runs here)
```

**Deploy** moves the release version, machine by machine, new code dormant.
**Finalize** moves the activation version, once, for everyone — and *that* is when
the new behavior wakes up.

**Use two axes when:** the change needs fleet-wide agreement (a wire protocol, a
shared storage format), or you want a rollback-safe staging period between deploy and
activate.

## Migrations, and the one word that changes everything

> **The piece almost every other gate misses.** A migration is tied to the
> **activation version**, not to deploy. So when you defer activation, the migration
> runs at the **moment of activation** (`finalize`) — never silently at deploy, and
> never left behind. A gate that is flipped on *later* must carry its data change to
> *that* later moment. Plain flag services (LaunchDarkly, Unleash) and most
> feature-gate libraries have no idea your gate implies a migration at all: they flip a
> boolean and leave the data change to you, as a separate, manual, easy-to-forget step.
> Coupling the two is the whole reason this layer exists.

When activation also transforms data, you attach a **migration** to the activation
version. A migration is a small object with a forward step, a reverse step, and a way
to check whether it still needs to run:

```go
type RewriteReceipts struct{}

func (RewriteReceipts) ID() string          { return "rewrite-receipts-v6" }
func (RewriteReceipts) Description() string  { return "Rewrite receipts into the v6 format" }
func (RewriteReceipts) Version() int         { return 6 }      // activates with activation version 6
func (RewriteReceipts) Scope() migration.Scope { return migration.Shared } // see below

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

## Features and migrations: two lists, one activation axis

A common question: *where does the list of migrations come from — isn't it derived from my
features?* No. You keep **two separate declarations**, linked only by the activation-version
number they share:

- the **`Registry`** — your features, for the gate (what resolves on/off);
- a **`[]migration.Migration`** slice — your migrations, for the planner and runner (what
  data gets transformed).

gatekit never derives one from the other, and — the key rule — **not every feature needs a
migration.** A feature earns one *only* when turning it on reshapes stored or wire data
(a compatibility gate). A development gate (an `Alpha` feature you flip on in place) or a
pure behavior flip carries **none**: there is nothing to transform.

Here are both lists for one app. `ReceiptsV6` changes stored data, so it pairs with a
migration at the **same activation version (6)**; `FastScan` is a dev gate, so it has none:

```go
// features.go — the gate side
var Registry = gatekit.Registry{
    ReceiptsV6: { // compatibility gate: new on-disk format, on fleet-wide at 6
        Description: "Store receipts in the v6 format",
        Specs: gatekit.VersionedSpecs{{ActiveFrom: 6, Default: true, Maturity: gatekit.GA}},
    },
    FastScan: { // development gate: experimental, off unless opted in — no data change
        Description: "Experimental faster scan path",
        Specs: gatekit.VersionedSpecs{{ActiveFrom: 6, Default: false, Maturity: gatekit.Alpha}},
    },
}

// migrations.go — the migration side: a hand-maintained slice, ONE entry per
// data-changing activation. ReceiptsV6's change rides RewriteReceipts (Version() == 6).
// FastScan appears nowhere here — a dev gate has no migration.
var Migrations = []migration.Migration{
    RewriteReceipts{}, // the companion to the ReceiptsV6 feature, at version 6
}
```

That slice *is* the `migs...` you pass to the planner; the same active version drives both
sides, so "which features are on" and "which migrations run" stay in lock-step:

```go
active := store.Current()
gate, _    := gatekit.New(Registry, gatekit.StaticVersion(active), gatekit.WithSupportWindow(1))
planner    := migration.NewPlanner(Migrations...).WithMin(gate.EffectiveVersion().Min)
plan, _    := planner.Plan(ctx, active, target)
```

When you add a feature later, you append to the registry — and to `Migrations` **only if it
reshapes data**:

| Feature kind | In `Registry`? | In `Migrations`? | Why |
|---|---|---|---|
| Reshapes stored/wire data (compat gate) | yes | **yes** — `Version()` = its `ActiveFrom` | the data must be transformed when it activates |
| Additive / node-local dev gate (`Alpha`) | yes | **no** | nothing to transform; flip in place |
| Pure behavior flip, no data | yes | **no** | lowering it is just resolving the old version again |

So `migs` is not pulled from your feature list — it is the short, explicit set of migrations,
one per data-changing activation, matched to its feature by the activation version they share.

## Scope: local vs shared

A migration's **scope** decides *when* it is allowed to run. gatekit does not hardwire
the meaning — you classify your own scopes — but it ships two well-understood ones:

> **local.** The migration affects only the machine it runs on (converting a
> local cache, say). It is safe to run **as soon as** its activation version activates
> — nobody else is affected.
>
> **shared.** The migration affects agreement *between* machines (a shared format, a
> wire protocol). It must be **run-at-finalize** until a coordinated finalize, or you get
> split-brain.

These map to a **run-policy**:

| Scope | Default run-policy | Runs when |
|-------|--------------------|-----------|
| `local` | `run-now` | immediately, at install/activation |
| `shared` | `run-at-finalize` | only at a deliberate `finalize` |
| *(anything unmapped)* | `run-at-finalize` | the safe default — never auto-fires |

You can define your own scopes (by [blast radius](/appendix/glossary.md#blast-radius)
— how much a change can affect — by how deliberate the change is, whatever you need)
and map each to `run-now` or `run-at-finalize`. Any scope you forget to map
falls back to `run-at-finalize` — the conservative choice, because a run-at-finalize migration never fires
on its own.

## Preview before you leap: the plan

Before running anything, gatekit computes a **plan**: exactly which migrations will
run now and which run at finalize, for the move from one version to another. A plan is a
pure preview — it changes nothing — so it is perfect for a `--dry-run`:

```go
planner := migration.NewPlanner(RewriteReceipts{}, WarmLocalCache{})

plan, err := planner.Plan(ctx, from /* 5 */, to /* 6 */)
if err != nil {
    return err
}

for _, step := range plan.RunNow() { /* will run at activation */ }
for _, step := range plan.RunAtFinalize()   { /* waits for finalize */ }
```

The plan includes a migration only if its version is in the
[half-open range](/appendix/glossary.md#half-open-range) `(from, to]` — greater than
`from`, up to and including `to` — **and** its `Applies` returns true.
Already-satisfied migrations are left out, which is why re-running a plan is safe.

## The two execution steps

Running a plan happens in up to two steps, with a runner:

```go
runner := migration.NewStdRunner() // dependency-free; a durable runner exists separately

// 1. Run the local (run-now) steps immediately. On any failure, it rolls back
//    the steps already done, in reverse, and does NOT advance the version.
if err := runner.RunNow(ctx, plan); err != nil {
    return err
}

// 2. Later, once the fleet is ready: finalize. This runs the run-at-finalize (shared) steps,
//    then advances the active version via the store. This is the one-way line.
if err := runner.Finalize(ctx, plan, store); err != nil {
    return err // version did NOT advance; run-at-finalize steps were rolled back
}
```

- **`RunNow`** does the safe, local work at install time. It never advances the
  active version.
- **`Finalize`** does the coordinated work and *then* advances the active version to
  the target. The version moves **only if every run-at-finalize step succeeds.**

### A deferred migration, start to finish

Put the whole sequence on one timeline, using the `RewriteReceipts` migration above
(a `shared` step tied to activation version 6). Watch *when* the migration actually
runs — not at deploy, but at finalize:

```
         Monday: DEPLOY                    (days pass…)              Friday: FINALIZE
  ───────────────────────────────────────────────────────────────────────────────►
  new binaries rolled out, dormant      preserve-downgrade        run-at-finalize migration RUNS,
  active = 5                             window: rollback safe     then active → 6
                                                                   behavior now live

  plan = Plan(from:5, to:6)             (nothing has run yet)      runner.Finalize(...)
  RewriteReceipts is HELD (shared)                                 → RewriteReceipts.Execute
  runner.RunNow(...) → no-op here                                   → store.SetActive(6)
```

Read it left to right:

1. **Monday — deploy.** You install the new binaries across the fleet. `active` is
   still 5, so every node behaves exactly as before. You compute the plan for `5 → 6`;
   `RewriteReceipts` is a `shared` migration, so it is **run-at-finalize** — `RunNow` has
   nothing to do. **The data has not been touched.**
2. **The days in between — the preserve-downgrade window.** The new code sits dormant.
   Nothing irreversible has happened, so rolling back to the old binary is clean. You
   watch metrics and wait until every node reports `max ≥ 6` (readiness).
3. **Friday — finalize.** You call `runner.Finalize`. *Now* `RewriteReceipts.Execute`
   runs — the receipts are rewritten — and only if it succeeds does `active` advance to
   6 and the new behavior wake up, fleet-wide, together.

This is the whole point from the top of the section, made concrete: **the migration
runs at the moment of activation, Friday — not when the code was deployed on Monday.**
A plain flag service would have flipped a boolean on Monday and left you to remember
the receipt rewrite yourself.

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
precise about this, and it makes you say so explicitly:

> An activation step is **reversible** if it has **no migration**, or if **every**
> migration in it is **declared `reversible`** *and* writes nothing that an older
> supported version could not read (the **write floor**).

Two details matter:

- **Irreversible is the default.** A migration is only reversible if its author says so.
  gatekit does not guess from the fact that a `Rollback` method exists, because a stub
  `Rollback` would then let you "go back" and quietly lose data.
- **A step with no migration is always reversible.** It is a pure gate flip, so going
  back just means resolving against the lower version again.

### Going back: lowering the active version

When a reversible step has gone wrong, you **lower** the active version. This is its own
operation, separate from finalize: finalize only raises `active`, and lowering only
lowers it. Say `active` is 6 and you want 4:

1. **Check the target.** It must be at or above `min`. Below `min`, no supported binary
   could run, so gatekit refuses (**below-min**).
2. **Collect what to undo.** Every migration for versions 5 and 6, newest first.
3. **Check each one is reversible.** If any is not, gatekit refuses and names them all
   (**irreversible-step**). Nothing has run yet, so there is nothing to clean up.
4. **Undo them, one by one.** If an undo fails, gatekit stops and leaves `active`
   alone. Because `Rollback` must be safe to repeat, you just run the lowering again.
5. **Lower `active`**, last of all. It is the same data-first order as finalize.

Raising `active` again later is an ordinary finalize. That is how you "roll forward".

### The write floor, in practice

> **Term — write floor.** A migration must never write data in a format too new for the
> oldest supported version (`min`) to read. A migration declares this with
> **`readableFrom`**: the oldest version that can read what it writes (the default, `0`,
> means every version). If `readableFrom` is **above** `min`, the migration is
> **blocked**, and `RunNow` and `Finalize` refuse to run it before they touch anything.

Why block it? Because someone could lower `active` to `min`. If the data were already in
a format that version cannot read, going back would corrupt or lose it. Honoring the
floor is what keeps "lower the active version" safe *within* the support window.
Crossing *below* `min` is never supported, and the run invariant forbids running there.

**What it means for `RewriteReceipts`.** The migration above writes the v6 receipt
format. With `W = 1` and `max = 6`, `min` is 5. For the rewrite to be allowed at
finalize, version 5 must already be able to *read* v6 receipts, so the migration
declares `readableFrom = 5`. If version 5 could not read them, the floor would block
the migration. You would split the change across two releases (expand, then contract):

| Release | What ships |
|---|---|
| **N (expand)** | Code that reads **both** receipt formats but still writes the old one. No data change, so this step is reversible. |
| **N+1 (contract)** | Once `min` has reached the version that reads both, run the rewrite. Its `readableFrom` is no longer above `min`, so it is allowed. |

A format change that cannot finalize in the same release is the price of being able to
go back safely. gatekit just makes the runner enforce what good teams do by habit.

Two points sharpen this:

- **It is the *write*, not the version, that is blocked.** Activating the new version is
  fine — you can finalize the behavior flip normally. Only the migration that *writes* the
  newer-than-`min` format is deferred.
- **"`min` has reached" does not mean the rollout finished.** `min` is `max − W`, computed
  from the registry (Lesson 6), so it rises only when the **next release raises `max`** —
  not when every machine has upgraded. That is why the contract step lands in release N+1,
  the one that introduces the next activation version: with `W = 1`, `max` becomes 6 there,
  so `min` becomes 5, and the `readableFrom = 5` rewrite is finally allowed.

The deferral is really just `W` showing its cost: with `W = 0`, `min = max`, so a format
newer than `min` is never blocked — you could write it immediately, but you would have
given up any rollback to the prior version. The one-release wait is the price of a
one-generation rollback window.

> **Where this stands.** The rules above are specified in the
> [migration spec §3.5–3.6](https://github.com/automa-saga/gatekit-spec/blob/main/migration-spec.md)
> and pinned by the `rollback/` and `floor/` conformance fixtures. The Go API for
> `reversible`, `readableFrom` and the lowering operation is still being built, so the
> snippets in this lesson do not show them yet.

## Putting the cases together

| Situation | Axes | Migration scope | Activation |
|-----------|------|-----------------|------------|
| Tool you deploy & control; no shared protocol | single (reuse release) | none | on deploy |
| Local format change, no coordination needed | single or two | `local` → run-now | at install |
| Shared wire/storage format; fleet must agree | two | `shared` → run-at-finalize | at `finalize` |
| You want a rollback-safe staging pause | two | either, deferred | at `finalize`, after the preserve-downgrade window |

## A saga underneath — and surviving a crash

The execution model — run steps in order, and on failure compensate the completed
ones in reverse — is a **saga**: a sequence of steps each with an undo, so a partial
failure can be unwound.

But notice the question the Friday timeline above quietly raises: **what if the
process crashes in the *middle* of finalize?** Say `RewriteReceipts` is halfway
through rewriting records when the machine reboots. You restart — and now what? Some
records are v2, some are still v1, and `active` never advanced. You must be able to
pick up exactly where you left off, not start over blindly or lose track.

gatekit answers this by making the **runner a pluggable interface**. You pick the
execution engine that matches how much safety you need, and your call sites
(`RunNow`, `Finalize`) never change:

| Runner | Dependencies | Guarantees | Use it when |
|--------|-------------|------------|-------------|
| `migration.StdRunner` (built in) | none | sequential; compensates on failure | the default — short migrations, or a tool you can just re-run |
| [gatekit-automa](https://github.com/automa-saga/gatekit-automa) | automa | all of the above **plus a journal**: survives a crash and **resumes** | long or multi-step migrations that must survive a crash mid-finalize |

`gatekit-automa` implements the same `migration.Runner` interface, so it is a
**drop-in swap**:

```go
import gkautoma "github.com/automa-saga/gatekit-automa"

// Was: runner := migration.NewStdRunner()
runner := gkautoma.NewRunner(gkautoma.WithJournal("/var/lib/app/finalize.journal"))

runner.RunNow(ctx, plan)          // same calls…
runner.Finalize(ctx, plan, store) // …now crash-safe and resumable
```

With a journal, automa write-aheads each step. If finalize crashes, re-running it
**skips the steps that already completed and re-runs the one that was interrupted** —
then advances `active` as the final step, exactly as before. This is why
[`Applies` must read ground truth](#migrations-and-the-one-word-that-changes-everything)
and migrations must be idempotent: resuming means a step may run again, and that must
be safe.

Two design notes worth holding onto:

- **Zero-dependency core.** `automa` is imported only inside `gatekit-automa`, never in
  gatekit itself — so a consumer that uses `StdRunner` never inherits the dependency.
  This is the "take only the layer you need" principle ([Lesson 2](/tutorial/2-why-gatekit.md))
  carried all the way down to the execution engine.
- **The spec fixes behavior, not the engine.** The migration spec pins the observable
  semantics (order, compensation, the one-way line); it does not mandate a runtime. Any
  conformant runner is interchangeable.

For a tool-sized worked example that puts deploy, deferred activation, the run-at-finalize
migration, finalize, and a crash-safe runner together, see
[Appendix E: a worked rollout](/appendix/worked-example.md).

## Check yourself

- Your change rewrites a shared storage format every node reads. One axis or two?
  Which scope? *(Two axes; `shared` (run-at-finalize) — the fleet must agree before the format
  changes.)*
- Why must `Applies` read the data instead of a "have I run this?" record? *(So the
  migration is idempotent and a crash can't leave a lying ledger.)*
- You deployed the new binaries an hour ago but have not finalized. Can you still
  roll back to the old binary? *(Yes — you are in the preserve-downgrade window,
  before the one-way line.)*
- When will gatekit refuse to lower the active version? *(When the target is below `min`,
  or when a step it crosses is not declared reversible.)*
- Why does a migration that is "reversible" still get blocked sometimes? *(Its output is
  not readable by the oldest supported version, so `readableFrom > min`. Expand first,
  then contract once `min` catches up.)*

## Further reading

- [gatekit activation spec §5–7](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md)
  — activation modes, finalize, reversibility, and the write floor.
- [gatekit migration spec](https://github.com/automa-saga/gatekit-spec/blob/main/migration-spec.md)
  — the migration contract, plan computation, and the saga execution model.
- [Appendix B: conformance fixtures](/appendix/conformance.md) — the `planning/`
  fixtures that pin down plan computation.

**Next:** [6. Rolling-upgrade thinking](/tutorial/6-rolling-upgrade-thinking.md)
