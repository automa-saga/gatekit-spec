# gatekit activation specification

> Status: **draft**. Normative keywords (MUST, SHOULD, MAY) per RFC 2119.
> Parts marked *(reserved)* are not yet specified and are out of scope until they are.

This document defines the version model that drives resolution
([core-spec.md](core-spec.md)): the `{min, max, active}` triple, the run
invariant, the configurable support window, activation and finalize, reversibility,
and the serialized formats.

**In short.** A binary supports a small range of activation versions. It runs at one
of them (`active`). Moving `active` up is a deliberate step called **finalize**, and
moving it back down is **lowering**. The rules below keep every move inside the range
and keep data readable on the way.

## 1. Two version axes

- **Release version** — the artifact's own version (e.g. semver). Out of scope
  here.
- **activation version** — a non-negative monotonically increasing integer that
  changes **only** when a gated, contract-breaking behavior change ships. Many
  releases MAY share one activation version.

An implementation MUST NOT equate the two: the activation version is independent of
the release version.

## 2. The triple

Every binary has three numbers:

| Name | Meaning | How it is set |
|---|---|---|
| **max** | The newest activation version this binary supports. | MUST equal `MaxActivation(registry)` (core-spec §7). Never edited by hand. |
| **min** | The oldest activation version this binary still supports. | Derived: `max(firstVersion, max − W)`. `firstVersion` is the lowest `activeFrom` in the registry (0 if empty). |
| **active** | The activation version in force right now. Resolution (core-spec §3) uses it. | Supplied by a **version source**. MAY be persisted. |

**W (support window)** is a non-negative integer, default **1**. It is how many earlier
versions the binary also supports.

*Example.* If the registry's highest `activeFrom` is 6, `W` is 1 and the lowest
`activeFrom` is 4, then `max = 6` and `min = max(4, 6 − 1) = 5`. This binary supports
`active` values 5 and 6.

## 3. Run invariant

At startup an implementation MUST verify that `min ≤ active ≤ max`. If not, it MUST
refuse to operate and report a typed error.

| Case | What it means | Fix |
|---|---|---|
| `active > max` | The binary is too old for the stored state. | Run a newer binary. |
| `active < min` | The stored state is too far behind for this binary. | Catch up in steps through the intermediate releases, or widen `W`. |
| `min ≤ active ≤ max` | Supported. | None. |

This guard stops a stale binary from undermining a flip, and it forces catch-up to
proceed one window at a time.

## 4. Increment rules

- activation version MUST increase by at most **1 per release**; multiple gated
  changes in one release share the same next integer.
- A new activation version is introduced by adding a `FeatureSpec` (or a migration,
  §5) at that integer — never by hand-editing `min`/`max`.
- Advancing the floor so a version leaves the window (`version < min`) obliges the
  author to delete that version's now-unreachable specs and dead code paths (the
  contract step). Lints SHOULD enforce this, keyed to `W` (core-spec §8).

## 5. Activation and migrations

An activation step MAY carry a one-time **migration**: a change to stored data or
state, on top of the gated code paths. Each migration declares a `scope`, which decides
when it may run:

| Scope (built-in default) | Meaning | Timing |
|---|---|---|
| `local` | Affects only this participant. | MAY run as soon as its version activates, for example at install. |
| `shared` | Needs agreement across participants. | MUST wait for a coordinated finalize (§6). |

There are two **activation modes**:

- **Immediate.** At install, `active` is set to the migration's version and the
  migration runs then. Valid for `local`.
- **Deferred.** The binary keeps running at the previous `active`, reading older
  formats down to `min`, and **holds** the migration until finalize. Required for
  `shared`. Permitted for `local` when an operator wants to validate in stages with a
  clean way back.

Before running anything, an implementation MUST be able to preview which migrations run
now and which run at finalize (a plan, or dry-run).

## 6. Finalize and reversibility

**Finalize** moves `active` up. It first runs the run-at-finalize migrations for the
target version, then advances `active`.

Finalize is the **one-way line**. A migration MAY be irreversible, so going back is only
guaranteed *before* finalize has run any migration.

An activation step is **reversible** when both of these hold:

- it has no migration, or every migration in it declares itself `reversible`
  (migration-spec §1), and
- it writes nothing that an older supported version cannot read (the write floor, §7).

A step with no migration is a pure gate flip, so it is always reversible.

An implementation that provides the migration layer MUST offer a **lowering**
operation (migration-spec §3.5). It undoes the migrations in descending version order
and then lowers `active`. It MUST refuse, with a typed error, when the target is below
`min` or when any step it crosses is irreversible. Lowering is the only way to lower
`active`; finalize never does.

*(reserved)* The coordinated/negotiated finalize across multiple participants
(readiness = every participant reports `max ≥ target`) and its message format are
not yet specified.

## 7. Min-compatibility write floor

**The rule.** Lowering `active` is only safe if the older version can still read what the
newer one wrote. So an implementation MUST NOT write on-disk or on-wire data in a format
that a version at `min` could not read.

**How a migration says so.** A migration declares `readableFrom`: the oldest activation
version that can read its output (migration-spec §1). The default is `0`, meaning every
version can.

**What is enforced.** A migration is **blocked** when `readableFrom > min`. `RunNow` and
`Finalize` MUST refuse to execute a blocked migration (migration-spec §3.6). With that
rule in place, lowering `active` anywhere inside `[min, max]` is data-safe. Going below
`min` is not supported, and the run invariant forbids running there.

**What it means for authors.** Because `min = max(firstVersion, max − W)`, `min` only
moves when a new release raises `max`. A change that makes data unreadable to older
versions therefore cannot finalize in the release that introduces it. Split it in the
**expand/contract** style:

1. **Release N (expand).** Ship code that can read both formats but still writes the old
   one. This is a reversible step with a low `readableFrom`.
2. **Release N+1 (contract).** Once `min` has reached the version that can read the new
   format, run the migration that converts the data. Its `readableFrom` is no longer
   above `min`, so it is allowed.

## 8. Serialized formats *(draft)*

### 8.1 Activation state
The persisted activation state is an object:

```json
{ "schema": 1, "active": <int>, "min": <int>, "max": <int> }
```

- `schema` (int) — the schema version of this document (currently `1`).
- Field names and the `schema` enumeration are fixed by this spec.
- `min` and `max` MAY be recorded for diagnostics but are authoritatively derived
  from the registry and `W` at load; `active` is authoritative.

### 8.2 Finalize-readiness / negotiation message *(reserved)*
Not yet specified; depends on §6.

## 9. Kubernetes mapping

`active` ↔ `--emulation-version`; `max` ↔ binary version; `min` ↔
`--min-compatibility-version`. gatekit differs by: a release-independent
`activation version`, a configurable window `W` (K8s fixes ~3), and bundling the
optional migration/activation layer (K8s keeps storage migration/conversion
separate).
