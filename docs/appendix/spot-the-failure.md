# Appendix D: Spot the failure

> **How to use this page.** Each item is a short, *plausible* piece of code or a
> rollout plan — the kind that passes review and ships. Read it, decide what goes
> wrong in production, then open the answer. Every failure here is real: a way
> feature gating and rolling upgrades bite teams. Each answer names the gatekit
> control that prevents it.
>
> This is the "failure modes" companion to [Lesson 6](/tutorial/6-rolling-upgrade-thinking.md)
> and the [design notes](/appendix/design-notes.md). It is a teaching tool, not a changelog.

The impact labels: 🟡 *silent wrong behavior* · 🟠 *outage / broken fleet* · 🔴
*data loss*.

---

<a id="f1"></a>

## F.1 — The feature that never turns on 🟡

```go
// registered once:
const SplitCatalog gatekit.Feature = "SplitCatalog"
// registry has SplitCatalog...

// at the call site, months later, a different engineer:
if gate.Enabled("split-catalouge") { // <- look closely
    loadSplitCatalog()
} else {
    loadSingleCatalog()
}
```

The team flips the activation version to turn `SplitCatalog` on. Nothing happens.
Metrics are flat. Why?

<details>
<summary>Answer</summary>

**The identifier is misspelled** (`catalouge`), so the call site asks about a feature
that is not in the registry. An **unknown feature resolves off, silently** — no error,
no warning, just the old path forever.

**The control:** declare the identifier **once as a typed `Feature` constant** and use
that constant everywhere (Lesson 4, Step 1). The compiler then rejects the typo. The
bare-string call site defeats the very protection the constant exists to give.
</details>

---

## F.2 — The rolling upgrade that broke the old nodes 🟠

```go
// v6 ships. To "turn on" the new behavior, the author edits the existing spec
// in place rather than adding a new one:
SplitCatalog: {
    Specs: gatekit.VersionedSpecs{
        {ActiveFrom: 5, Default: true, Maturity: gatekit.GA}, // was Default:false
    },
}
```

During the rollout, nodes still running the *old* binary start misbehaving — before
anyone flipped anything. Why?

<details>
<summary>Answer</summary>

**Editing the spec in place rewrote history.** A machine resolving at activation
version 5 now gets `Default: true`, so the new behavior turns on the instant a node
sees the new registry — with no coordination, mid-rollout. That is the split-brain a
rolling upgrade must avoid.

**The control:** when a contract-breaking change happens, **add a new spec at the next
activation version** (`{ActiveFrom: 6, Default: true}`), never mutate the old one
(Lesson 4, Step 4a; [design note 3](/appendix/design-notes.md)). The old spec must keep
resolving the old behavior so machines still at 5 stay correct.
</details>

---

## F.3 — The compatibility check that lies 🟠

```go
// "Are these two nodes compatible? Just compare their release versions."
func compatible(a, b string) bool {
    return semverMinor(a) == semverMinor(b) // v2.4.x vs v2.4.y
}
```

Two nodes at `v2.4.1` and `v2.4.7` are declared compatible and put in the same
cluster. They corrupt each other's state. But the minors matched!

<details>
<summary>Answer</summary>

**Release version is the wrong axis.** `v2.4.7` may have shipped a gated behavior
change that `v2.4.1` does not understand — the release minor says nothing about which
*behavior generation* a binary runs.

**The control:** compare the **activation version**, not the release version (Lesson 2;
[design note 3](/appendix/design-notes.md)). The activation version moves *only* on a behavior
change, so `min ≤ active ≤ max` is a meaningful compatibility test and the semver
comparison is not.
</details>

---

## F.4 — The dead code deleted too early 🟠

```go
// max is 6, window W=1, so min=5 — version 5 is STILL supported.
// The author cleans up anyway: "the split catalog is on in prod now, drop the old path."
func loadCatalog(gate *gatekit.FeatureGate) {
    // removed loadSingleCatalog() and the whole if/else; now unconditional
    loadSplitCatalog()
}
```

The fleet is mid-upgrade: most nodes are at `active = 6`, but a few slow ones are
still at `active = 5`. After this change ships, those slow nodes start splitting the
catalog too — a behavior they were never meant to run yet, and that their peers at 5
don't expect. What went wrong?

<details>
<summary>Answer</summary>

**The old path was deleted while version 5 was still in the support window.** With
`max = 6` and `W = 1`, `min = 5`, so a node at `active = 5` is still supported and
still *expects* the old single-catalog behavior. Removing the `if`/`else` makes
`loadCatalog` unconditional, so it runs the new path at **every** version — including
5. The collapse happened one generation too early.

A code path is only safe to delete **once its version drops below `min`** — out of the
window (Lesson 6, Rule 2). Here that moment arrives only when `max` reaches 7 (which
pushes `min` to 6); until then, both paths must stay. Note the run invariant does
*not* catch this: `active = 5` is in `[5, 6]`, so the node starts cleanly and then
misbehaves — a within-window logic bug, not an out-of-range refusal.

**The control:** the **support window** defines exactly when a path is safe to delete,
and the **removal lint** (core-spec §8) keys this to `W` so it is not purely a human judgment
call ([design note 5](/appendix/design-notes.md)). Delete on the window's schedule, not on "it's
on in production now."
</details>

---

## F.5 — The stale node that undermined the flip 🟠

```go
func main() {
    gate, _ := gatekit.New(Registry, versionSource)
    // straight to serving — no gate.Check()
    serve(gate)
}
```

The fleet finalizes to activation version 7. One old node keeps running at `active =
5` and quietly serves old-format responses that corrupt a shared index. Why did it
even start?

<details>
<summary>Answer</summary>

**The run invariant was never checked.** With `max = 7` and `W = 1`, `min = 6`, so
`active = 5` violates `min ≤ active ≤ max`. The node should have refused to start — but
nobody called `Check()`.

**The control:** call `gate.Check()` at startup and **refuse to operate on error**
(Lesson 6, Rule 3; activation-spec §3). The invariant exists precisely to stop a stale
node from undermining a flip; skipping the check throws that protection away.
</details>

---

## F.6 — The migration that ran twice and double-charged 🔴

```go
func (m ApplyLateFees) Applies(ctx context.Context) (bool, error) {
    // "have we run this migration before?" — checked against a runs table
    return !m.ranLog.Contains(m.ID()), nil
}
func (m ApplyLateFees) Execute(ctx context.Context) error {
    return addLateFeeToEveryOverdueAccount(ctx) // not idempotent!
}
```

The process crashes *after* `Execute` finishes but *before* the runs-table write. On
restart the migration runs again. Every overdue account is charged a late fee twice.

<details>
<summary>Answer</summary>

**`Applies` consulted a ledger, and the ledger lied after the crash.** The effect
(late fees) was applied, but the "we ran it" record was not yet written, so `Applies`
returned true and `Execute` ran a second time.

**The control:** `Applies` MUST read **ground truth**, not a record of past runs
(Lesson 5; [design note 7](/appendix/design-notes.md)). Here it should ask "does this account
*already have* this period's late fee?" — then a re-run is a safe no-op. Reading the
data, not a log, is what makes the migration idempotent and the saga crash-safe.
</details>

---

## F.7 — The scope that was forgotten 🟠

```go
planner := migration.NewPlanner(migrations...) // no WithScopePolicy
// a migration declares a custom scope the author assumed means "run at install":
func (m ReshardIndex) Scope() migration.Scope { return "reshard" }
```

The author expects `ReshardIndex` to run at install. It does not — it waits for
finalize. Annoying, but notice: this is the *safe* direction. What is the real lesson?

<details>
<summary>Answer</summary>

An **unmapped scope resolves to `run-at-finalize`**, not `run-now`. The author forgot to map
`"reshard"` with `WithScopePolicy`, so gatekit fell back to the conservative default: a
run-at-finalize step never fires automatically at install.

**The control / design intent:** scopes are consumer-defined and unmapped scopes
**fail closed** to `run-at-finalize` ([design note 8](/appendix/design-notes.md); migration-spec §2.1). The
bug — forgetting the mapping — produces a *delayed* migration, never a *surprise*
one. If the opposite default had been chosen, a forgotten mapping would auto-fire a
reshard across the fleet at install. The lesson: map every scope explicitly, and be
glad the default is the safe one.
</details>

---

## F.8 — The finalize before the fleet was ready 🔴

```go
// deploy v7 binaries, then immediately:
runner.Finalize(ctx, plan, store) // advances active to 7 for everyone
```

Finalize runs the shared migration and advances `active` to 7. But three nodes are
still on the v6 binary (`max = 6`). They crash on startup / can't read the new format.

<details>
<summary>Answer</summary>

**Finalize happened before readiness.** The shared migration and the version advance
must wait until *every* participant reports `max ≥ 7` (Lesson 6, Rule 4). Finalizing
first means the nodes still at `max = 6` now face `active = 7`, which violates their run
invariant (`active > max`).

**The controls:** confirm **readiness** before finalize, and the **run invariant** on
the lagging nodes turns the mistake into a clear refusal-to-start rather than silent
corruption. (The coordinated, negotiated readiness check — every participant reporting
in — is reserved for a future spec; today you confirm it with your fleet tooling.)
</details>

---

## F.9 — The write that closed the door early 🔴

```go
func (m RewriteReceipts) Execute(ctx context.Context) error {
    // writes the v7 format, which the v6 binary cannot parse
    return writeReceiptsAsV7(ctx)
}
// scope: local -> runs NOW, at install, while fleet is still mixed v6/v7
```

The team wanted to keep rollback open for a week (the preserve-downgrade window). But
the day they deploy, receipts start appearing in a format the still-present v6 nodes
cannot read — and rollback to v6 is now impossible.

<details>
<summary>Answer</summary>

**A write crossed the write floor during the preserve-downgrade window.** The migration
emitted v7-format data while a supported version (v6) still needed to read it, and it
ran *now* (local) instead of being deferred — collapsing the rollback window the team
thought they had.

**The controls:** honor the **write floor** — never write data a version down to `min`
cannot read (Lesson 5; activation-spec §7) — and give a format-changing migration an
`shared` scope so it is **run at finalize** rather than running at install. Together
these keep "lower the active version" safe and keep the preserve-downgrade window
actually open. gatekit also **refuses to lower `active` across an irreversible step**,
so it will not let you pretend the door is still open once it isn't.
</details>

---

## F.10 — The alpha that stayed dark 🟡

```go
NewIndexer: {
    Specs: gatekit.VersionedSpecs{
        {ActiveFrom: 7, Default: true, Maturity: gatekit.Alpha}, // Default is true!
    },
}
// active = 7, no options
gate.Enabled(NewIndexer) // the author expects true
```

`Default` is `true` and `active` is 7, so the author is sure the feature is on. It is
off. Bug, or correct?

<details>
<summary>Answer</summary>

**Correct behavior.** An `Alpha` feature resolves **off unless explicitly opted in**,
*even when its `Default` is `true`* (Lesson 4; algorithm step 3). This lets you merge
genuinely unfinished work with no risk it turns itself on.

**The control:** `Alpha` maturity + the `WithExperimental(true)` opt-in (or a
per-feature override). The author either wanted `Beta`/`GA`, or needs to opt in. The
"bug" is a misunderstanding of maturity, and the design is protecting them.
</details>

---

## The pattern behind all ten

Nearly every failure above is one of three mistakes:

1. **Fusing what gatekit separates** — identifier from constant (F.1), behavior from
   history (F.2), activation from release (F.3), activate from deploy (F.8, F.9).
2. **Skipping a guard rail** — the support window (F.4), the run invariant (F.5),
   idempotent `applies` (F.6), the write floor (F.9).
3. **Misreading a safe default as a bug** — run-at-finalize-by-default scopes (F.7),
   off-by-default alpha (F.10).

gatekit's job is to make the safe path the default and the dangerous path *refuse*.
When something surprises you, check first whether it is a guard rail doing its job.

## See also

- [Appendix C: design notes](/appendix/design-notes.md) — the decisions these failures justify.
- [Lesson 6](/tutorial/6-rolling-upgrade-thinking.md) — the five rules, stated
  positively.
