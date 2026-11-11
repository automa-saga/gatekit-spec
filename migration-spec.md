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
  when a later step in the same run fails, and (for reversible migrations) to lower
  `active` (see activation-spec §6). It MUST be idempotent.
- `reversible` (boolean, default `false`) — declares that `rollback()` fully and
  safely reverses `execute()`. The default is **irreversible**: an implementation
  MUST NOT infer reversibility from the mere presence of `rollback()`, because a
  stub would then silently allow a retreat that loses data.
- `readableFrom` (activation version, optional, default `0`) — the oldest
  activation version able to read what `execute()` produces. `0` means every
  version can. See the write floor, activation-spec §7.

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
whether it is run-now or run-at-finalize, whether it is `reversible`, and whether
the write floor blocks it (§3.6).

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
A step is **reversible** iff it has no migration, or every migration in it declares
`reversible = true` (§1) and honors the write floor (activation-spec §7). A step with
no migration is a pure gate flip and is always reversible. Lowering `active`
(activation-spec §6) is permitted only across reversible steps and MUST be refused
across an irreversible step (§3.5).

### 3.4 Execution is a saga
The execution model in §3.1–§3.2 is a **saga / compensation** model: steps run in
order and a failure compensates the completed steps in reverse. Execution SHOULD be
**resumable** across a crash. This spec fixes the plan and the observable
run-now/run-at-finalize/one-way semantics, not the runtime; an implementation MAY realize them
with a saga runtime (the Go implementation uses `automa-saga/automa`, which carries
its own durability specification).

### 3.5 Lowering `active`

Lowering is the way back after a finalize: it moves `active` from a higher version
(`from`) to a lower one (`to`) without changing the binary. It MUST be its own
operation. Finalize only ever raises `active`, and lowering only ever lowers it.

For example, `active` is 6 and you want 4. The migrations that belong to versions 5
and 6 must be undone, newest first, and then `active` becomes 4.

**Steps.** An implementation MUST do these in order, and stop at the first refusal:

1. **Check the target.** If `to < min`, refuse with a **below-min** error. No supported
   binary can run below `min`, so there is nothing safe to go back to.
2. **Build the reverse plan.** Take every migration whose `version` is above `to` and at
   or below `from`. Include a migration even when `applies()` is true. Reversal cannot
   tell a migration that ran from one whose effect was already there, so it plays safe
   and relies on `rollback()` being idempotent. Order the plan by `version`, highest
   first, then by `id`, highest first.
3. **Check every step can be undone.** If any migration in the plan is not `reversible`,
   refuse with an **irreversible-step** error that lists all of them. Nothing has run
   yet, so nothing needs cleaning up.
4. **Undo the steps.** Run `rollback()` for each migration, in plan order. If one fails,
   stop and report the error. MUST NOT lower `active`. Running the lowering again is the
   recovery, because `rollback()` is safe to repeat.
5. **Lower `active`** to `to`, only after every step has succeeded.

**Why this order.** It is the mirror of finalize (§3.2): data first, `active` last. A
failure at any point leaves the recorded version unchanged, so the operation can simply
be retried. Raising `active` again later is an ordinary finalize.

**One more rule.** A persisted `active` MUST NOT be lowered by anything other than this
operation.

**Which error, and when**

| Situation | Result |
|---|---|
| `to < min` | refused: **below-min** (checked first, even if steps are also irreversible) |
| any step in the plan is not `reversible` | refused: **irreversible-step**, naming each one |
| a `rollback()` fails | stopped: error reported, `active` unchanged, safe to re-run |
| every step succeeded | `active` lowered to `to` |

### 3.6 Write-floor preflight

The write floor (activation-spec §7) stops a migration from writing data that the oldest
supported version cannot read. This section says when that check happens.

A step is **blocked** if any of its migrations has `readableFrom` above `min`.

Before `RunNow` or `Finalize` runs anything, it MUST check the steps it is about to run.
If any step is blocked, it MUST refuse with a **write-floor** error that names every
blocked step. Nothing is executed and `active` does not change. A blocked step can run
later, once a new release moves `max` and `min` up to `readableFrom` (activation-spec §2).

## 4. Relationship to the existing frameworks
This model generalizes conventional version-boundary migration frameworks: instead
of "run once when the binary version crosses a boundary," a migration declares the
activation version it belongs to and runs when that version **activates** — at install
for run-now steps, at finalize for run-at-finalize steps.
