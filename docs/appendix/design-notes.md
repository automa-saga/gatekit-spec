# Appendix C: Design notes

> The decisions behind gatekit, and why they went the way they did. Each note is a
> "we chose X over Y because Z." If you are evaluating gatekit or implementing it,
> this is the reasoning you would otherwise have to reconstruct.
>
> Authoritative detail lives in the specs:
> [core](https://github.com/automa-saga/gatekit-spec/blob/main/core-spec.md),
> [activation](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md),
> [migration](https://github.com/automa-saga/gatekit-spec/blob/main/migration-spec.md).
> The overall effort is tracked in the
> [gatekit v1 epic](https://github.com/automa-saga/gatekit/issues/6).

## 1. Spec first, implementation second

**Decision.** The product is the *specification* plus the shared conformance
fixtures. The Go code is the first conformant implementation, not the definition.
Where code and spec disagree, the spec wins and the code is a bug.

**Why.** For a library meant to exist in many languages, two runtimes that resolve a
feature differently are worthless to a user. Pinning behavior in language-neutral
prose + fixtures is the only way N implementations stay in agreement. See
[Appendix B](/appendix/conformance.md).

## 2. In-binary and deterministic, not a flag service

**Decision.** Resolution is a pure function of the registry and the version, compiled
into the binary. No network call, no runtime flag service.

**Why.** The core job is "a fleet must *agree*, deterministically, on which behavior
generation it runs." A remote flag can differ between two machines at the same instant
and has a failure mode when the service is down — both fatal to fleet agreement. The
cost is giving up per-user targeting and dashboard flips, which is the right trade for
this job (and you can run a flag service *alongside* gatekit for the product-experiment
job). See [Lesson 2](/tutorial/2-why-gatekit.md) and
[Lesson 3](/tutorial/3-contrast-with-existing-patterns.md).

## 3. A release-independent activation version

**Decision.** Feature gating keys on a standalone `activation version` — a monotonic
counter that advances by at most one per release and *only* on a gated,
behavior-changing feature — not on the release/semver version.

**Why.** The release version moves constantly for reasons unrelated to behavior (bug
fixes, perf, docs). Keying on it would make "are these two machines compatible?"
impossible to read from the number. A dedicated counter makes compatibility plain
arithmetic. This is the same choice CockroachDB (cluster version), Kafka (protocol
version), and Kubernetes KEP-4330 (emulation version) made. See activation-spec §1.

## 4. A configurable support window `W`, default 1

**Decision.** The number of prior activation versions a binary supports is a
configurable `W`, defaulting to 1 (N/N+1). `min = max(firstVersion, max − W)`.

**Why.** Fleets upgrade at different speeds. A tool you control is fine at `W = 1`; a
platform whose operators upgrade slowly needs a wider window so a lagging machine is
still supported. Kubernetes fixes this at roughly three; making it a knob fits more
situations without forcing everyone to the widest. See activation-spec §2 and
[Lesson 6](/tutorial/6-rolling-upgrade-thinking.md), Rule 2.

## 5. The run invariant refuses, loudly

**Decision.** At startup a binary checks `min ≤ active ≤ max` and *refuses to operate*
with a typed error if it fails.

**Why.** A machine running outside its window can misread data or undermine a flip.
Failing loud at startup converts a subtle, hard-to-trace corruption bug into an
obvious, fixable error. A rolling upgrade needs a hard floor it cannot sink below.
See activation-spec §3.

## 6. Two layers; migrations are opt-in

**Decision.** The gate core (registry + resolution) is always present and
dependency-light. The migration/activation layer (deferred activation, migrations,
finalize) is a separate, opt-in package.

**Why.** Many consumers only need "ship dormant, flip later" and should not inherit
the weight or the concepts of a migration engine. A consumer whose active version is
set externally can reuse the core without the activation model at all. See
[Lesson 2](/tutorial/2-why-gatekit.md) and the gatekit README.

## 7. `applies()` reads ground truth, not a ledger

**Decision.** A migration decides whether it still needs to run by inspecting the
actual data/state, never a record of past runs.

**Why.** A ledger can lie after a crash mid-migration. Reading ground truth makes a
migration idempotent — safe to re-run, a no-op once its effect is already present —
which is what makes the whole saga crash-safe. See migration-spec §1.

## 8. Scope is consumer-defined; unmapped scopes are `run-at-finalize`

**Decision.** A migration's `scope` is an opaque, consumer-defined string mapped to a
run-policy (`run-now` / `run-at-finalize`) by a consumer-supplied classifier. gatekit predefines
`local` and `shared` only as the keys of the *default* classifier, not as a
closed enum. Any scope the classifier does not map resolves to `run-at-finalize`.

**Why.** Consumers classify migrations by different axes (blast radius,
deliberateness, …); baking in a fixed set would not fit them. Defaulting the unknown
case to `run-at-finalize` is the safe choice, because a run-at-finalize step never fires automatically at
install — a forgotten mapping fails *closed*, not open. See migration-spec §2.1.

## 9. Finalize is a one-way line; the runner enforces reversibility

**Decision.** `finalize` runs run-at-finalize migrations and advances `active` as a single,
deliberate step. Lowering `active` is permitted only across reversible steps and is
*refused* with a typed error across an irreversible one.

**Why.** A migration may be irreversible, so rollback can only be *guaranteed* before
finalize. Making finalize the explicit crossing point — and refusing to lower across
an irreversible step — means you can never silently assume a retreat that is not
actually possible. This is CockroachDB's `preserve_downgrade_option` generalized. See
activation-spec §6 and migration-spec §3.

**Lowering and the write floor.** The retreat is a real operation, not a promise
(migration-spec §3.5). It runs the reverse migrations (version descending) and lowers
`active` last, the mirror of finalize, so a failure leaves the recorded version
unchanged and the operation can simply be run again. Two rules keep it honest:

- A migration is **irreversible unless it declares `reversible`**. Presence of a
  `rollback()` is not enough, because a stub would otherwise permit a retreat that
  loses data.
- A migration declares `readableFrom`, the oldest version able to read its output, and
  the runner **refuses to execute it while `min < readableFrom`** (the write floor,
  activation-spec §7). That is what makes lowering to `min` data-safe.

The cost of the floor is that a data-format change cannot finalize in the release that
introduces it. Ship code that reads both formats first, then flip the format once `min`
has caught up: expand/contract, enforced by the runner instead of by habit.

## 10. Execution is a saga; the runtime is swappable

**Decision.** The spec fixes the observable semantics — run in order, compensate
completed steps in reverse on failure, advance the version only on full success — but
not the runtime. The default `StdRunner` is a dependency-free sequential
implementation; a durable, crash-resumable runner
([gatekit-automa](https://github.com/automa-saga/gatekit-automa)) implements the same
interface.

**Why.** Separating the *contract* (what you can observe) from the *engine* (how it
runs) lets a simple consumer stay dependency-free while a demanding one gets
durability — without changing any call sites, and without the automa dependency
leaking into a plain consumer's module graph. See migration-spec §3.4.

## Lifecycle / removal lint

**Status: specified (core-spec §8) with conformance fixtures; implementations are catching up.**
When a version leaves the window the author must delete the now-unreachable specs and
dead code paths (the contract step). The lint reports three kinds of violation, all
derived from the registry and `W`: `stale-spec` (a spec superseded at or below `min`),
`past-remove-by` (a feature whose optional `removeBy` is at or below `min`), and
`deprecated` (a feature whose latest spec is `DEPRECATED`).

**Why.** A gate that is never deleted is a bug (see
[Lesson 4, Step 6](/tutorial/4-build-and-release-a-feature.md)); Kubernetes' long-lived
gates show the failure mode. Keying the check to `W` turns "delete old paths" from a
human judgment into a test failure.

**Why no separate window-violation kind.** Live specs per feature are bounded by
`W + 1`, so any excess is exactly the `stale-spec` set.

**What it does not do.** It does not scan source code. Locating the dead branches a
stale spec guarded stays with the consumer.

## 11. Errors are typed; enrichment is the consumer's job

**Decision.** gatekit carries **no dependencies** and never will for the gate core.
It reports failures as plain stdlib errors plus a small set of **typed, branchable**
errors — `VersionRangeError` (the run invariant), `BelowMinError`,
`IrreversibleStepError`, and `WriteFloorError` (lowering and the write floor). It does
**not** attach reason codes, operator hints, or remediation text of its own.

**Why dependency-free.** The gate core must drop into any project — a CLI, a daemon, a
tiny library — without dragging a dependency graph behind it, and the activation model
must behave identically across Go, Kotlin, Rust, and Python. An error-decoration library
is the opposite of both goals: it would force a transitive dependency on *every* consumer
and bind the error contract to one language's idioms, which the language-neutral
[conformance fixtures](/appendix/conformance.md) could never pin down. The fixtures
express failures as stable kinds (`below-min`, `irreversible-step`, …); the typed Go
errors mirror those kinds, nothing more.

**Why consumers enrich.** A reason code, an operator-facing hint, and a suggested fix are
*application* knowledge, not library knowledge. Only the consumer knows which flag to
suggest, which backup to restore, or how its CLI renders diagnostics. So gatekit draws the
line at a **branchable** error and hands the enrichment to the consumer, which decorates it
**once, at the boundary** where gatekit's error crosses into application code. A Go consumer
does this with its own error stack — see
[the Go page, *Enriching gatekit's errors*](/tutorial/go.md#enriching-gatekits-errors-in-your-consumer).

## See also

- [Appendix A: the resolution algorithm](/appendix/algorithm.md)
- [Appendix D: spot the failure](/appendix/spot-the-failure.md) — these decisions seen through
  the bugs they prevent.
