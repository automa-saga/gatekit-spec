# gatekit Specifications

> Status: **draft** — the core and protocol specs are still settling alongside the first (Go) implementation.

gatekit is intended to exist in more than one language (Go first, then others such
as Kotlin, Java, Python, and Rust) for the same audience and use cases: a
version-keyed feature-gate and rolling-upgrade mechanism.

For a multi-language project the **product is the specification**, not any single
implementation. Two runtimes that both "implement gatekit" are worthless to a user
if a feature resolves differently, or a persisted protocol state is read
differently, depending on the language. The specifications here define the
behavior every conformant implementation MUST exhibit, so that:

- a protocol state written by one implementation is understood identically by
  another,
- feature resolution is identical everywhere given the same registry and version,
  and
- the obligations placed on feature authors (gate at the correct increment,
  reversibility, the N/N+W window) are one promise, not a per-language footnote.

## Principles

1. **Spec first, implementation second.** Observable behavior is defined here in
   language-neutral terms. The Go implementation
   ([automa-saga/gatekit](https://github.com/automa-saga/gatekit)) is the **first
   conformant implementation**, not the definition. Where the Go code and a spec
   disagree, the spec is authoritative and the code is a bug.
2. **On-disk / on-wire formats are contracts.** Any artifact persisted or
   exchanged between processes (the `{min, max, active}` protocol state, the
   finalize-readiness/negotiation message) has a versioned, language-neutral
   schema. Field names and value enumerations are fixed by the spec.
3. **Conformance is testable.** Each spec is accompanied by language-neutral
   **conformance fixtures** (inputs + expected outcomes as plain JSON). Every
   implementation MUST pass the shared fixtures; they are the source of truth for
   cross-language agreement.
4. **The core stays small.** The smaller the specified core, the cheaper it is to
   keep N implementations in agreement. Anything outside the core is out of scope
   until it is specified here.

## Documents

- [core-spec.md](core-spec.md) — the feature-gate model: features, versioned
  specs, and deterministic resolution.
- [protocol-spec.md](protocol-spec.md) — the version model: the `{min, max,
  active}` triple, the run invariant, the configurable support window, activation
  (immediate/deferred), finalize, reversibility, and the serialized formats.
- [migration-spec.md](migration-spec.md) — the optional migration/activation
  layer: the migration contract, plan computation (run-now vs held), and execution
  (run-now, finalize, rollback).

## Conformance

See [conformance/README.md](conformance/README.md). Fixtures are grouped by
`resolution/`, `invariant/`, `planning/`, and `serialization/`, and enumerated in
[conformance/manifest.json](conformance/manifest.json).

## Relationship to Kubernetes

gatekit is the standalone, general-purpose analogue of Kubernetes'
compatibility-version / feature-gate model (KEP-4330). Terminology maps as:
`--emulation-version` ↔ `active`, binary version ↔ `max`,
`--min-compatibility-version` ↔ `min`. gatekit decouples the version from the
release (a separate monotonic `protocolVersion`) and makes the support window
configurable rather than fixed.
