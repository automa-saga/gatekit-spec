# Conformance fixtures

Language-neutral fixtures every gatekit implementation MUST pass. They are the
source of truth for cross-language agreement. Groups are enumerated in
[manifest.json](manifest.json).

## Groups

- **resolution/** — feature-gate resolution (core-spec §3–5). Schema:
  `{ name, description, registry: {feature: {description?, specs: [spec]}}, cases: [{ active, allowAlpha?, overrides?: {feature: bool}, enabled: {feature: bool} }] }`.
  Each registry entry carries an optional, informational `description` (core-spec §1)
  that MUST NOT affect resolution, and the feature's `specs`. A feature named in
  `enabled` but absent from `registry` tests the unknown-feature rule (expected off).
  `allowAlpha` (optional, default false) opts the gate in to `ALPHA` features
  gate-wide (core-spec §4); `overrides` (optional) forces specific features on/off
  regardless of the version — the highest-precedence input (core-spec §5).
- **invariant/** — the run invariant (activation-spec §3). Schema:
  `{ name, description, max, window, firstVersion, cases: [{ active, valid }] }`
  where `min = max(firstVersion, max - window)`.
- **planning/** — migration planning (migration-spec §2). Schema:
  `{ name, description, scopePolicy?: {scope: policy}, migrations: [{ id, version, scope, applies }], from, to, runNow: [id], runAtFinalize: [id] }`.
  `runNow`/`run-at-finalize` are the expected step ids (ordered by version then id). `scopePolicy`
  (optional) is the consumer scope→run-policy classifier, each policy one of `run-now`
  or `run-at-finalize`; when omitted the built-in default applies (`local`→run-now,
  `shared`→run-at-finalize), and any scope the classifier does not map resolves to `run-at-finalize`.
- **lint/** — the lifecycle lint (core-spec §8). Schema:
  `{ name, description, window, registry: {feature: {removeBy?, specs: [spec]}}, violations: [{ feature, kind, activeFrom? }] }`
  where `kind` is `stale-spec`, `past-remove-by`, or `deprecated`, and `violations` is in
  the order core-spec §8 defines (feature, then kind, then `activeFrom`). `min` and `max`
  are derived from `registry` and `window`; an empty `violations` is a clean registry.
- **serialization/** — *(reserved)* activation-state and message round-trips.

## Running

Each implementation ships a conformance runner that loads these fixtures (e.g. via
a git submodule) and asserts the expected outcomes. Reserved groups are empty until
the corresponding behavior is specified and implemented.
