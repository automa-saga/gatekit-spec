# gatekit activation specification

> Status: **draft**. Normative keywords (MUST, SHOULD, MAY) per RFC 2119.
> Parts marked *(reserved)* are not yet specified and are out of scope until they are.

This document defines the version model that drives resolution
([core-spec.md](core-spec.md)): the `{min, max, active}` triple, the run
invariant, the configurable support window, activation and finalize, reversibility,
and the serialized formats.

## 1. Two version axes

- **Release version** — the artifact's own version (e.g. semver). Out of scope
  here.
- **activation version** — a non-negative monotonically increasing integer that
  changes **only** when a gated, contract-breaking behavior change ships. Many
  releases MAY share one activation version.

An implementation MUST NOT equate the two: the activation version is independent of
the release version.

## 2. The triple

- **max** — the newest activation version an implementation supports. It MUST equal
  `MaxActivation(registry)` (core-spec §7).
- **W (support window)** — a configurable non-negative integer, default **1**. It
  is the number of prior activation versions the implementation also supports.
- **min** — derived as `max(firstVersion, max − W)`, where `firstVersion` is the
  lowest `activeFrom` in the registry (0 if empty).
- **active** — the activation version currently in force; the value resolution
  (core-spec §3) uses. It is supplied by a **version source** and MAY be persisted.

## 3. Run invariant

At startup an implementation MUST verify `min ≤ active ≤ max`. If `active < min`
(the binary is too new for a far-behind state) or `active > max` (the binary is
too old), it MUST refuse to operate and report a typed error. This guard is what
prevents a stale implementation from undermining a flip, and it forces
catch-up to proceed within the window.

## 4. Increment rules

- activation version MUST increase by at most **1 per release**; multiple gated
  changes in one release share the same next integer.
- A new activation version is introduced by adding a `FeatureSpec` (or a migration,
  §5) at that integer — never by hand-editing `min`/`max`.
- Advancing the floor so a version leaves the window (`version < min`) obliges the
  author to delete that version's now-unreachable specs and dead code paths (the
  contract step). Lints SHOULD enforce this, keyed to `W`.

## 5. Activation and migrations

A activation step MAY carry a one-time **migration** (a data/state transformation)
in addition to gated code paths. Each migration declares a `scope`:

- `local` — affects only the local participant; MAY run when its
  activation version activates (e.g. at install).
- `shared` — requires agreement across participants; MUST be **run-at-finalize** until a
  coordinated finalize (§6).

Activation modes:

- **Immediate** — `active` is set to the migration's version on install; the
  migration runs then. Valid for `local`.
- **Deferred** — the implementation operates at the prior `active` (reading older
  formats down to `min`) and **holds** the migration until finalize. Required for
  `shared`; permitted for `local` when an operator wants rollback-safe
  staged validation.

Implementations MUST be able to preview, before running, which migrations run now
versus run at finalize (a plan / dry-run).

## 6. Finalize and reversibility

**Finalize** advances `active`. Before advancing it runs the run-at-finalize migrations for
the target version. Finalize is the **one-way line**: because a migration MAY be
irreversible, rollback is only guaranteed *before* finalize has run any migration.

A activation step is **reversible** iff it has no migration, or every migration in it
provides a working reverse and writes nothing that an older version in the window
cannot read (the write floor, §7). For a reversible step an implementation MAY
lower `active` (running reverse migrations). Lowering `active` across an
**irreversible** step MUST be refused with a typed error.

*(reserved)* The coordinated/negotiated finalize across multiple participants
(readiness = every participant reports `max ≥ target`) and its message format are
not yet specified.

## 7. Min-compatibility write floor

To keep lowering `active` data-safe, an implementation MUST NOT emit on-disk or
on-wire data in a format that a version below `min` could not read. A
writer/migration MAY declare the minimum activation version required to read its
output; emitting below the floor MUST be rejected. Within `[min, max]`, lowering
`active` is data-safe; crossing below `min` is not supported (and the run invariant
forbids running there).

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
