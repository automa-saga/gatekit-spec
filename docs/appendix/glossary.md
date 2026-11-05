# Glossary

Plain-language definitions of every term used across the gatekit course. Terms are
grouped by topic and cross-referenced where helpful. Where a term has an exact
normative definition, the spec is authoritative.

## The core idea

<a id="deploy"></a>**Deploy.** Putting new code onto the machines that run it. In
gatekit, deploying is deliberately *behavior-neutral*: the new code can be present but
asleep. Contrast [activate](#activate).

<a id="activate"></a>**Activate.** Making a new behavior actually happen. In gatekit
this is a separate event from [deploy](#deploy) — you turn a feature on by advancing a
version, with no new deployment.

<a id="feature-gate"></a>**Feature gate (gate).** A controlled `if` statement that
decides, at runtime, whether a feature is on. Ships in the *off* branch so code can be
merged and deployed before it is turned on.

<a id="feature"></a>**Feature.** A stable string identifier for a gated behavior.
Declared once as a typed constant and referenced by both registration and every call
site, so a typo cannot silently resolve a different feature off.

<a id="dormant"></a>**Dormant.** Shipped but not active: the code is on the machine,
but the gate resolves it off, so the old path runs. The safe state a feature lives in
between deploy and activate.

## Versions

<a id="release-version"></a>**Release version.** The artifact's own version (for
example a semver like `v2.4.1`). It moves constantly, for reasons unrelated to
behavior. Out of scope for resolution; deliberately *not* what gatekit keys on.

<a id="activation-version"></a>**Activation version.** A non-negative, monotonically
increasing integer that identifies a *behavior generation*. It advances by at most one
per release and only when a gated, contract-breaking change becomes reachable. It is
independent of the [release version](#release-version); many releases may share one.
This is the axis gatekit keys on.

<a id="protocol-version"></a>**Protocol version.** The name of the *pattern* where the
decoupled [activation version](#activation-version) is a dedicated, deliberately
advanced counter that participants use to agree on behavior (as in Kafka's inter-broker
protocol version or CockroachDB's cluster version). In gatekit's vocabulary it is a
*use* of the activation version, not a separate thing.

<a id="single-axis"></a>**Single axis.** Reusing your [release version](#release-version)
(or a build number) as the activation version, activating on deploy. The simple case:
no separate counter, no finalize. See [Lesson 5](/tutorial/5-activation-use-cases.md).

<a id="two-axis"></a>**Two axes.** Running a dedicated [activation version](#activation-version)
counter separate from the release version, advanced deliberately at
[finalize](#finalize). Needed when a change requires fleet-wide agreement or a
rollback-safe staging period.

## The triple and the window

<a id="triple"></a>**The triple `{min, max, active}`.** The three numbers a binary
advertises for rolling upgrades. See each below.

<a id="max"></a>**max.** The newest [activation version](#activation-version) a binary
supports. Equal to `MaxActivation(registry)` — the greatest `activeFrom` across all
specs — and never hand-edited.

<a id="min"></a>**min.** The oldest activation version a binary still supports. Derived
as `max(firstVersion, max − W)`.

<a id="active"></a>**active.** The activation version currently in force — the value
[resolution](#resolution) uses. Supplied by a [version source](#version-source) and may
be persisted.

<a id="support-window"></a>**Support window (`W`).** How many activation versions
behind [max](#max) a binary still supports. Configurable, default 1 (N/N+1). It is your
"coexistence budget" during a rolling upgrade; it must be at least as wide as the
version gap your fleet spans mid-upgrade.

<a id="run-invariant"></a>**Run invariant.** The startup check `min ≤ active ≤ max`. If
it fails, the binary must refuse to operate and return a typed error. `active < min`
means the binary is too new for a far-behind state; `active > max` means it is too old.
The guard that stops a stale binary from undermining a flip.

## Resolution

<a id="resolution"></a>**Resolution.** Computing whether a feature is on. A pure
function of the [registry](#registry) and the [active](#active) version — no clock,
environment, or network. See [Appendix A](algorithm.md).

<a id="registry"></a>**Registry.** The in-binary table mapping each [feature](#feature)
to its definition (an optional description plus its versioned specs).

<a id="featurespec"></a>**FeatureSpec.** One entry in a feature's lifecycle: an
`activeFrom` (the activation version it takes effect), a `default` (on/off from then
on), and a [maturity](#maturity).

<a id="versionedspecs"></a>**VersionedSpecs.** A feature's ordered lifecycle — one or
more [FeatureSpec](#featurespec)s. Order-independent at declaration; sorted by
`activeFrom` before resolution.

<a id="effective-spec"></a>**Effective spec.** For a feature at version `v`, the spec
with the greatest `activeFrom` that is ≤ `v`. Its `default` sets the base state.

<a id="maturity"></a>**Maturity.** A feature's lifecycle stage: `ALPHA` (off unless
opted in), `BETA` and `GA` (follow their `default`), `DEPRECATED` (follows `default`
but flagged for removal).

<a id="override"></a>**Override.** An explicit per-feature force on/off that beats the
version-resolved default — the highest-precedence input to resolution. May name only a
registered feature.

<a id="description"></a>**Description.** An optional, human-readable summary of a
feature for listings and dry-run output. Informational only: it never affects
resolution.

## Migrations and activation

<a id="migration"></a>**Migration.** A one-time transformation of stored or on-the-wire
data, tied to an [activation version](#activation-version). Has a forward step
(`execute`), a reverse step (`rollback`), and an [applies](#applies) check.

<a id="applies"></a>**Applies.** A migration's check for whether it still needs to run.
Must read **ground truth** (the actual data), not a ledger of past runs, so the
migration is idempotent and crash-safe.

<a id="scope"></a>**Scope.** A consumer-defined classification of a migration, mapped
to a [run-policy](#run-policy) by a classifier. gatekit predefines `host-local` and
`interop` only as the keys of the default classifier, not a closed set.

<a id="host-local"></a>**host-local.** A migration affecting only the machine it runs
on. In the default policy it runs immediately when its version activates.

<a id="interop"></a>**interop.** A migration affecting agreement between machines. In
the default policy it is [held](#held) until a coordinated [finalize](#finalize).

<a id="run-policy"></a>**Run-policy.** The timing a migration is eligible for:
`run-now` (at install/activation) or [held](#held) (until finalize). An unmapped scope
resolves to `held` — the safe, fail-closed default.

<a id="held"></a>**Held.** A migration step deferred until a deliberate
[finalize](#finalize); it never fires automatically at install.

<a id="plan"></a>**Plan.** The ordered set of migration steps to move from one version
to another, each marked run-now or held. A pure value computed on demand and never
stored; the basis of a `--dry-run` preview.

<a id="finalize"></a>**Finalize.** The deliberate step that runs the [held](#held)
migrations and advances [active](#active) to the target. The
[one-way line](#one-way-line): the version advances only if every held step succeeds.

<a id="one-way-line"></a>**One-way line.** The point at which rollback stops being
guaranteed, because [finalize](#finalize) may run an irreversible migration. Before it,
retreat is clean; after it, it is not.

<a id="preserve-downgrade-window"></a>**Preserve-downgrade window.** The deliberate
pause between deploying new code and calling [finalize](#finalize), during which
rolling back to the old binary is still safe. Named after CockroachDB's
`preserve_downgrade_option`.

<a id="reversible"></a>**Reversible.** An activation step is reversible if it has no
migration, or every migration provides a working `rollback` and honors the
[write floor](#write-floor). Only reversible steps may be undone by lowering
[active](#active).

<a id="write-floor"></a>**Write floor.** The rule that a migration must never write
data in a format too new for the oldest supported version ([min](#min)) to read.
Honoring it is what keeps lowering `active` data-safe within the window.

<a id="saga"></a>**Saga.** An execution model where steps run in order and a failure
compensates the completed steps in reverse (each step has an undo). gatekit's migration
runner is a saga; a durable, crash-resumable runner exists behind the same interface.

## Fleet and upgrades

<a id="rolling-upgrade"></a>**Rolling upgrade.** Upgrading a fleet by replacing or
restarting machines a few at a time, so the service never goes fully down and never
switches all at once. Requires old and new code to [coexist](#coexistence) for a while.

<a id="coexistence"></a>**Coexistence.** The stretch of a rolling upgrade where old and
new code run side by side, serving the same users and data. Every gatekit guard rail
exists to make coexistence safe.

<a id="readiness"></a>**Readiness.** The condition that a fleet is safe to advance:
every participant reports [max](#max) ≥ the target version. [Finalize](#finalize) waits
for readiness. The coordinated, negotiated form of this check is reserved for a future
spec.

<a id="split-brain"></a>**Split-brain.** Two machines disagreeing about a behavior or
protocol at the same instant, so users see inconsistent results. The failure gatekit's
deterministic, in-binary [resolution](#resolution) is designed to prevent.

<a id="version-source"></a>**Version source.** The thing that tells gatekit the current
[active](#active) version. Pluggable: gatekit does not decide who owns the value, so it
can be a static value, config, or the migration layer's store.

<a id="conformance-fixture"></a>**Conformance fixture.** A language-neutral JSON file
of inputs and expected outcomes that every implementation must pass. The source of
truth for cross-language agreement. See [Appendix B](conformance.md).

## General terms

A few words from software and math that the lessons use. We keep the real term and
define it here rather than talk around it.

<a id="monotonic"></a>**Monotonic.** Only ever moves in one direction. For a counter,
it means the value only goes up, never down. The
[activation version](#activation-version) is monotonic.

<a id="half-open-range"></a>**Half-open range `(from, to]`.** A span that *excludes*
its lower end and *includes* its upper end: every value greater than `from`, up to and
including `to`. gatekit's [plan](#plan) includes migrations whose version falls in
`(from, to]`.

<a id="idempotent"></a>**Idempotent.** Safe to run more than once: running it again
when its effect is already present changes nothing. gatekit migrations are idempotent
because [applies](#applies) reads ground truth instead of a log of past runs.

<a id="blast-radius"></a>**Blast radius.** How much a change can affect if it goes
wrong — one machine, or the whole fleet. One axis a consumer might use to classify a
migration's [scope](#scope).

<a id="polyglot"></a>**Polyglot.** Using more than one programming language. gatekit is
built for polyglot systems, where (say) a Go service and a Kotlin service must agree
on the same behavior.
