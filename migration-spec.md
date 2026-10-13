# gatekit migration specification

> Status: **draft**. Normative keywords (MUST, SHOULD, MAY) per RFC 2119.

This document defines the optional migration/activation layer: the migration
contract, how a plan is computed, and how plans are executed. It builds on the
version model in [protocol-spec.md](protocol-spec.md) (the `{min, max, active}`
triple, activation timing, and finalize). The [conformance/planning](conformance/planning)
fixtures are the source of truth for plan computation.

An implementation of the core ([core-spec.md](core-spec.md)) MAY omit this layer
entirely (a consumer whose active version is set externally needs only
resolution). Implementations that provide it MUST conform to this document.

## 1. Migration contract

A **Migration** is a one-time transformation tied to a protocol version:

- `id` (string) — a stable, unique identifier (stable across releases).
- `description` (string) — human-readable summary.
- `version` (integer) — the protocolVersion this migration activates with.
- `scope` (enum) — `host-local` or `interop` (see protocol-spec §5).
- `applies()` → boolean — whether the migration still needs to run. It MUST read
  **ground truth** (on-disk/cluster state), NOT a ledger of past runs, so that a
  migration is safe to re-run and `applies()` returns false once the effect is
  already present.
- `execute()` — performs the forward transformation. It SHOULD be safe to re-run.
- `rollback()` — reverses `execute()` on a best-effort basis, used to compensate
  when a later step in the same run fails, and (for reversible steps) to lower
  `active` (see protocol-spec §6).

## 2. Plan computation

Given a current version `from` and a target version `to` (`from ≤ to`), a **plan**
is computed as follows. A migration is included iff:

1. `from < version ≤ to` (the half-open range), **and**
2. `applies()` returns true.

Migrations failing (1) or (2) MUST be omitted. Omitting already-satisfied
migrations makes the plan **idempotent** across re-runs.

Each included migration becomes a **step**, classified by scope:

- `host-local` → **run-now** (eligible to run at install / immediate activation).
- `interop` → **held** (deferred until finalize).

Steps MUST be ordered by `version` ascending, then by `id` ascending, so plans are
deterministic. A plan is a pure value: computed on demand, never persisted.

An implementation MUST be able to render a plan before executing it (a
`--plan` / dry-run preview): for each step, its `id`, `description`, `scope`, and
whether it is run-now or held.

## 3. Execution

### 3.1 Run-now
Executing the run-now steps MUST run them in plan order. On the first failure the
implementation MUST roll back the steps it already executed, in reverse order
(best-effort), and MUST NOT advance `active`.

### 3.2 Finalize
Finalize MUST run the held steps in plan order; on failure it MUST roll back the
executed held steps (reverse order) and MUST NOT advance `active`. On success it
MUST advance `active` to `to`. Finalize is the **one-way line** (protocol-spec §6):
rollback is only guaranteed before any migration in it has run.

### 3.3 Reversibility
Lowering `active` (protocol-spec §6) is permitted only across **reversible** steps
(no migration, or every migration provides a working `rollback()` and honors the
write floor, protocol-spec §7) and MUST be refused across an irreversible step.

## 4. Relationship to the existing frameworks
This model generalizes conventional version-boundary migration frameworks: instead
of "run once when the binary version crosses a boundary," a migration declares the
protocolVersion it belongs to and runs when that version **activates** — at install
for run-now steps, at finalize for held steps.
