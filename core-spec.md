# gatekit core specification

> Status: **draft**. Normative keywords (MUST, SHOULD, MAY) are used per RFC 2119.

This document defines the feature-gate model: how a feature's state is declared
and how it is resolved against an activation version. It is language-neutral; the
[conformance/resolution](conformance/resolution) fixtures are the source of truth.

## 1. Concepts

- **activation version** — a non-negative, monotonically increasing integer that
  identifies a behavior/contract generation. It is distinct from a release
  version (see [activation-spec.md](activation-spec.md)).
- **Feature** — a stable string identifier for a gated behavior. An implementation
  SHOULD represent `Feature` as a distinct type (not a bare string), and consumers
  SHOULD declare each feature as a named constant/enum of that type — one source of
  truth referenced by both registration and call sites, so an identifier cannot
  drift or be mistyped (a mistyped identifier resolves **off**, silently).
- **FeatureSpec** — declares a feature's `default` and `maturity` from a given
  `activeFrom` activation version onward:
  - `activeFrom` (activation version — a non-negative integer) — the activation version at
    which this spec takes effect.
  - `default` (boolean) — whether the feature is on at or after `activeFrom`, unless
    a later spec overrides it or an explicit override is supplied.
  - `maturity` (enum) — one of `ALPHA`, `BETA`, `GA`, `DEPRECATED`.
- **VersionedSpecs** — a feature's ordered lifecycle: one or more `FeatureSpec`s.
- **FeatureDef** — a feature's registry entry: its `VersionedSpecs` plus an optional
  `description`:
  - `description` (string, optional) — a human-readable summary for operator-facing
    listings, generated docs, and the plan / dry-run output. It is **informational
    only**: it MUST NOT affect resolution (§3), the `maturity` policy (§4),
    overrides (§5), or the run invariant (activation-spec §3). It MAY be empty or
    absent.
  - `removeBy` (activation version, optional) — the activation version by which the
    feature MUST be deleted: once `min ≥ removeBy` (activation-spec §2), no supported
    version predates it. It is **lint metadata only** (§8): it MUST NOT affect
    resolution, overrides, or the run invariant. Absent means no removal deadline;
    `0` is not meaningful (`min ≥ 0` always holds) and an implementation MAY treat it
    as absent.
  - `specs` — the feature's `VersionedSpecs`.
- **Registry** — a map from `Feature` to its `FeatureDef`.

## 2. Registry validity

An implementation MUST reject a registry when, for any feature:

- its `VersionedSpecs` is empty, or
- any spec has a negative `activeFrom`, or
- two specs share the same `activeFrom`.

An implementation MUST treat `VersionedSpecs` as order-independent at declaration
time and sort by `activeFrom` ascending before resolution. An implementation MAY make a
negative `activeFrom` unrepresentable at the type level (e.g. an unsigned type), which
satisfies the non-negativity rule by construction.

## 3. Resolution

Given a feature `f` and a current activation version `v`, the **effective spec** is
the spec of `f` with the greatest `activeFrom` that is less than or equal to `v`.

- If `f` is not in the registry, `f` is **off**.
- If `f` has no spec with `activeFrom ≤ v` (its first spec is not yet reached), `f`
  is **off**.
- Otherwise the effective spec's `default` determines the base state.

`Enabled(f)` resolves against the current version supplied by the implementation's
version source (see [activation-spec.md](activation-spec.md) §Active). Resolution MUST
be a pure function of the registry and the version; it MUST NOT depend on wall
clock, environment, or call order.

## 4. maturity policy

- `ALPHA` features are **not guaranteed across versions** and MUST resolve to
  **off** unless explicitly opted in (an override, see §5, or an implementation's
  equivalent of an experimental opt-in). Authors are NOT required to preserve the
  behavior of an alpha feature across activation versions.
- `BETA` and `GA` features follow their declared `default`.
- `DEPRECATED` marks a feature scheduled for removal; it resolves by `default`
  like `BETA`/`GA` but SHOULD be reported by lints as due for removal.

## 5. Overrides

An implementation MUST allow an explicit per-feature override that forces a
feature on or off regardless of the version-resolved default. Precedence, highest
first:

1. explicit override,
2. version-resolved `default` (subject to the `ALPHA` opt-in rule),
3. off.

## 6. Author obligation

Gate a change **only if it breaks the contract with existing users of the
feature.** Bug fixes, performance work, refactors, and unstable-metric changes do
NOT require a gate. When a change does break the contract, record it as a new
`FeatureSpec` at the next activation version rather than mutating behavior in place,
so older behavior remains resolvable within the support window.

## 7. MaxActivation

`MaxActivation(registry)` is the greatest `activeFrom` across all specs, or `0` for an
empty registry. Implementations derive `max` (see activation-spec) from this rather
than hand-maintaining it.

## 8. Lifecycle lint

An implementation SHOULD provide a lint that enforces the contract step
(activation-spec §4). It is a pure function of the registry and the support window
`W`: it derives `max` and `min` as in activation-spec §2, needs no version source,
and so can run in a consumer's test suite or CI. It returns a list of violations,
each a `{feature, kind, activeFrom?}`.

| Kind | Reported when | `activeFrom` |
|---|---|---|
| `stale-spec` | the feature has a later spec whose `activeFrom ≤ min`, so this spec can never be the effective spec for any permitted `active`. Delete it and its dead code path. | the stale spec's `activeFrom` |
| `past-remove-by` | the feature declares `removeBy` and `min ≥ removeBy`. Delete the feature. | absent |
| `deprecated` | the feature's latest spec has `maturity` `DEPRECATED` (§4). It is due for removal. | absent |

Violations are ordered by feature name ascending, then by kind in the order above,
then by `activeFrom` ascending. A clean registry yields an empty list.

A spec window violation (more live specs than the window allows) is not a separate
kind. At most one spec per feature is effective at `min` and each later `activeFrom`
adds at most one more, so live specs are bounded by `W + 1`; any excess is exactly the
`stale-spec` set.

The lint does not look at source code. Finding and deleting the dead code paths that a
reported spec guarded is the consumer's responsibility.

The [conformance/lint](conformance/lint) fixtures are the source of truth.
