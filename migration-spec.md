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

A **Migration** is a one-time change tied to an activation version. It has these fields:

| Field | Type | Meaning |
|---|---|---|
| `id` | string | Stable, unique, and unchanged across releases. |
| `description` | string | Human-readable summary. |
| `version` | integer | The activation version this migration belongs to. |
| `scope` | string | A **consumer-defined** label. gatekit does not fix the set of scopes. Each scope maps to a run-policy (§2.1) that decides timing. `local` and `shared` are only the keys of the built-in default policy (activation-spec §5), not a closed list. |
| `reversible` | boolean, default `false` | Declares that `rollback()` fully and safely undoes `execute()`. See the note below. |
| `readableFrom` | activation version, default `0` | The oldest version able to read what `execute()` writes. `0` means every version. See the write floor, activation-spec §7. |

And these operations:

- `applies()` returns whether the migration still needs to run. It MUST read **ground
  truth** (on-disk or cluster state), not a ledger of past runs. That makes a migration
  safe to re-run, and `applies()` returns false once the effect is already there.
- `execute()` does the forward change. It SHOULD be safe to re-run.
- `rollback()` undoes `execute()` on a best-effort basis. It is used to compensate when
  a later step in the same run fails, and, for reversible migrations, to lower `active`
  (activation-spec §6). It MUST be idempotent.

**Reversible defaults to false on purpose.** An implementation MUST NOT infer
reversibility from the mere presence of `rollback()`, because a stub would then
silently allow a retreat that loses data.

## 2. Plan computation

A **plan** answers: given where `active` is now (`from`) and where it is going (`to`,
with `from ≤ to`), which migrations must run?

A migration is included when **both** are true:

1. its `version` is above `from` and at or below `to` (that is, `from < version ≤ to`),
2. `applies()` returns true.

Everything else is left out. Leaving out migrations that are already satisfied makes
plans **idempotent**: computing one again after a run gives a shorter plan.

Each included migration becomes a **step**. Its timing comes from its scope's
run-policy (§2.1).

Steps MUST be ordered by `version` ascending, then by `id` ascending, so plans are
deterministic. A plan is a plain value: computed when needed and never stored.

### 2.1 Run-policy and scope classification

A **run-policy** is when a step is allowed to run. There are two:

- `run-now`: eligible at install or immediate activation.
- `run-at-finalize`: held until a deliberate finalize (§3.2).

**Classifying scopes.** An implementation MUST turn each migration's `scope` into a
run-policy through a **classifier** supplied by the consumer (a scope-to-policy map or
equivalent). It MUST NOT hardcode the mapping, and a consumer MUST be able to define
**three or more** scopes.

**The default classifier.** When the consumer supplies none, the implementation MUST use
this built-in one, so existing consumers are unaffected:

| Scope | Policy |
|---|---|
| `local` | `run-now` |
| `shared` | `run-at-finalize` |

**Unmapped scopes.** A scope the classifier does not map MUST resolve to
`run-at-finalize`. That is the safe choice, since it never fires automatically at
install. An implementation MAY let a consumer's classifier extend or override the
built-in default.

**The scope name carries no meaning.** The scope string is opaque to gatekit. The
host-local versus cross-host meaning of `local` and `shared` (activation-spec §5)
belongs to the default policy only. A consumer may classify along any axis it needs,
such as blast radius or how deliberate the step is.

**Previewing a plan.** An implementation MUST be able to show a plan before running it
(a `--plan` or dry-run preview). For each step it shows:

- the `id`, `description` and `scope`,
- whether the step is `run-now` or `run-at-finalize`,
- whether it is `reversible`,
- whether the write floor blocks it (§3.6).

## 3. Execution

### 3.1 Run-now

Run the `run-now` steps in plan order. On the first failure, the implementation MUST
undo the steps it already ran, in reverse order (best-effort), and MUST NOT advance
`active`.

### 3.2 Finalize

Run the `run-at-finalize` steps in plan order. On the first failure, undo the steps
already run, in reverse order, and MUST NOT advance `active`. If every step succeeds,
advance `active` to `to`.

Finalize is the **one-way line** (activation-spec §6): going back is only guaranteed
before any of its migrations has run.

### 3.3 Reversibility

A step is **reversible** when it has no migration, or when every migration in it has
`reversible = true` (§1) and honors the write floor (activation-spec §7). A step with
no migration is a pure gate flip, so it is always reversible.

Lowering `active` (activation-spec §6) is allowed only across reversible steps. An
implementation MUST refuse to lower across an irreversible step (§3.5).

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
