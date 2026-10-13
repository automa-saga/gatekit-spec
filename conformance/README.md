# Conformance fixtures

Language-neutral fixtures every gatekit implementation MUST pass. They are the
source of truth for cross-language agreement. Groups are enumerated in
[manifest.json](manifest.json).

## Groups

- **resolution/** — feature-gate resolution (core-spec §3). Schema:
  `{ name, description, registry, cases: [{ active, enabled: {feature: bool} }] }`.
  A feature named in `enabled` but absent from `registry` tests the unknown-feature
  rule (expected off).
- **invariant/** — the run invariant (protocol-spec §3). Schema:
  `{ name, description, max, window, firstVersion, cases: [{ active, valid }] }`
  where `min = max(firstVersion, max - window)`.
- **planning/** — migration planning (migration-spec §2). Schema:
  `{ name, description, migrations: [{ id, version, scope, applies }], from, to, runNow: [id], held: [id] }`.
  `runNow`/`held` are the expected step ids (ordered by version then id).
- **serialization/** — *(reserved)* protocol-state and message round-trips.

## Running

Each implementation ships a conformance runner that loads these fixtures (e.g. via
a git submodule) and asserts the expected outcomes. Reserved groups are empty until
the corresponding behavior is specified and implemented.
