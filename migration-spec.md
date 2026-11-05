# gatekit migration specification

> Status: **draft**. Normative keywords (MUST, SHOULD, MAY) per RFC 2119.

This document defines the optional migration/activation layer: the migration
contract, how a plan is computed, and how plans are executed. It builds on the
version model in [activation-spec.md](activation-spec.md) (the `{min, max, active}`
triple, activation timing, and finalize). The [conformance/planning](conformance/planning)
fixtures are the source of truth for plan computation.

An implementation of the core ([core-spec.md](core-spec.md)) MAY omit this layer
entirely (a consumer whose active version is set externally needs only
resolution). Implementations that provide it MUST conform to this document.

## 1. Migration contract

A **Migration** is a one-time transformation tied to an activation version:

- `id` (string) — a stable, unique identifier (stable across releases).
- `description` (string) — human-readable summary.
- `version` (integer) — the activation version this migration activates with.
- `scope` (string) — a **consumer-defined** classification of the migration. gatekit
  does not fix the set of scopes; a consumer MAY define as many as it needs. Each
  scope is mapped to a **run-policy** (§2.1) that decides the migration's timing.
  gatekit predefines `local` and `shared` only as the keys of the built-in
  default policy (activation-spec §5), not as a closed enum.
- `applies()` → boolean — whether the migration still needs to run. It MUST read
  **ground truth** (on-disk/cluster state), NOT a ledger of past runs, so that a
  migration is safe to re-run and `applies()` returns false once the effect is
  already present.
- `execute()` — performs the forward transformation. It SHOULD be safe to re-run.
- `rollback()` — reverses `execute()` on a best-effort basis, used to compensate
  when a later step in the same run fails, and (for reversible steps) to lower
  `active` (see activation-spec §6).

## 2. Plan computation

Given a current version `from` and a target version `to` (`from ≤ to`), a **plan**
is computed as follows. A migration is included iff:

1. `from < version ≤ to` (the half-open range), **and**
2. `applies()` returns true.

Migrations failing (1) or (2) MUST be omitted. Omitting already-satisfied
migrations makes the plan **idempotent** across re-runs.

Each included migration becomes a **step** whose timing is its scope's run-policy
(§2.1).

Steps MUST be ordered by `version` ascending, then by `id` ascending, so plans are
deterministic. A plan is a pure value: computed on demand, never persisted.

### 2.1 Run-policy and scope classification

A **run-policy** is the timing a step is eligible for, one of:

- `run-now` — eligible to run at install / immediate activation.
- `run-at-finalize` — deferred until a deliberate `finalize` (§3.2).

An implementation MUST resolve each migration's `scope` to a run-policy through a
**consumer-supplied classifier** (a scope→policy map or equivalent). It MUST NOT
hardcode a fixed scope→policy mapping. A consumer MUST be able to define **three or
more** scopes, each mapped to a run-policy.

The implementation MUST provide a **built-in default classifier** used when the
consumer supplies none, so existing consumers are unaffected:

- `local` → `run-now`,
- `shared` → `run-at-finalize`.

A scope that the effective classifier does not map MUST resolve to `run-at-finalize` — the
conservative default, since `run-at-finalize` never fires automatically at install. An
implementation MAY let a consumer's classifier extend or override the built-in
default.

The scope string itself is opaque to gatekit: the per-host/cross-host meaning of
`local`/`shared` (activation-spec §5) is a property of the *default* policy, not
of the scope axis. A consumer MAY classify by any axis it needs (blast radius,
deliberateness, …) without gatekit ascribing meaning to the names.

An implementation MUST be able to render a plan before executing it (a
`--plan` / dry-run preview): for each step, its `id`, `description`, `scope`, and
whether it is run-now or run-at-finalize.

## 3. Execution

### 3.1 Run-now
Executing the run-now steps MUST run them in plan order. On the first failure the
implementation MUST roll back the steps it already executed, in reverse order
(best-effort), and MUST NOT advance `active`.

### 3.2 Finalize
Finalize MUST run the run-at-finalize steps in plan order; on failure it MUST roll back the
executed run-at-finalize steps (reverse order) and MUST NOT advance `active`. On success it
MUST advance `active` to `to`. Finalize is the **one-way line** (activation-spec §6):
rollback is only guaranteed before any migration in it has run.

### 3.3 Reversibility
Lowering `active` (activation-spec §6) is permitted only across **reversible** steps
(no migration, or every migration provides a working `rollback()` and honors the
write floor, activation-spec §7) and MUST be refused across an irreversible step.

### 3.4 Execution is a saga
The execution model in §3.1–§3.2 is a **saga / compensation** model: steps run in
order and a failure compensates the completed steps in reverse. Execution SHOULD be
**resumable** across a crash. This spec fixes the plan and the observable
run-now/run-at-finalize/one-way semantics, not the runtime; an implementation MAY realize them
with a saga runtime (the Go implementation uses `automa-saga/automa`, which carries
its own durability specification).

## 4. Relationship to the existing frameworks
This model generalizes conventional version-boundary migration frameworks: instead
of "run once when the binary version crosses a boundary," a migration declares the
activation version it belongs to and runs when that version **activates** — at install
for run-now steps, at finalize for run-at-finalize steps.
