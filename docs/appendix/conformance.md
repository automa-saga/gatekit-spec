# Appendix B: Conformance fixtures

> **The point of this appendix:** gatekit is meant to exist in several languages. For
> that to be worth anything, a feature must resolve *identically* everywhere, and an
> activation state written by one implementation must be read *identically* by
> another. The **conformance fixtures** are how that promise is kept and checked.

## What a fixture is

A **conformance fixture** is a plain-JSON file holding an *input* and the *expected
outcome*. It is language-neutral: there is no Go, Kotlin, or Rust in it — just data.
Every implementation ships a small runner that loads these files and asserts it
produces the expected outcomes.

The fixtures are the **source of truth** for cross-language agreement. If the Go code
and a fixture disagree, the code is wrong. If a new implementation passes every
fixture, it agrees with every other implementation *by construction*.

They live in [`conformance/`](https://github.com/automa-saga/gatekit-spec/tree/main/conformance)
and are enumerated in
[`conformance/manifest.json`](https://github.com/automa-saga/gatekit-spec/blob/main/conformance/manifest.json).

## The groups

| Group | What it pins down | Spec |
|-------|-------------------|------|
| `resolution/` | feature-gate resolution: specs, defaults, maturity, overrides, alpha opt-in | core §3–5 |
| `invariant/` | the run invariant `min ≤ active ≤ max` and window arithmetic | activation §3 |
| `planning/` | migration plan computation: range, `applies`, scope→policy, ordering | migration §2 |
| `serialization/` | *(reserved)* activation-state and message round-trips | activation §8 |

## A resolution fixture, read line by line

Here is a real fixture — a Beta feature promoted to GA, and an Alpha feature that
stays off by default:

```json
{
  "name": "basic_lifecycle",
  "registry": {
    "featureA": {
      "description": "Split the catalog into per-minor files",
      "specs": [
        { "activeFrom": 4, "default": false, "maturity": "BETA" },
        { "activeFrom": 6, "default": true,  "maturity": "GA" }
      ]
    },
    "featureB": {
      "specs": [ { "activeFrom": 5, "default": false, "maturity": "ALPHA" } ]
    }
  },
  "cases": [
    { "active": 3, "enabled": { "featureA": false, "featureB": false } },
    { "active": 4, "enabled": { "featureA": false, "featureB": false } },
    { "active": 6, "enabled": { "featureA": true,  "featureB": false } },
    { "active": 99, "enabled": { "featureA": true, "featureB": false } }
  ]
}
```

- `registry` is the input feature table. `description` is present precisely to prove
  it is ignored — resolution must not change because of it.
- Each `case` sets `active` and lists the expected `enabled` result per feature.
- `featureA` flips on at `active = 6` (its GA spec). `featureB` is Alpha with its
  default off, so it stays off at every version shown here. (The separate
  [`overrides_and_alpha.json`](https://github.com/automa-saga/gatekit-spec/blob/main/conformance/resolution/overrides_and_alpha.json)
  fixture isolates the Alpha rule: it gives an Alpha feature a `true` default and
  shows it still resolving **off** until the gate is opted in.)

## The fixture schemas

Summarized from
[`conformance/README.md`](https://github.com/automa-saga/gatekit-spec/blob/main/conformance/README.md):

**`resolution/`**
```
{ name, description,
  registry: { feature: { description?, specs: [ {activeFrom, default, maturity} ] } },
  cases: [ { active, allowAlpha?, overrides?: {feature: bool}, enabled: {feature: bool} } ] }
```
`allowAlpha` (default false) opts into ALPHA gate-wide; `overrides` forces features
on/off at highest precedence; a feature named in `enabled` but absent from `registry`
tests the unknown-feature rule (expected off).

**`invariant/`**
```
{ name, description, max, window, firstVersion, cases: [ { active, valid } ] }
```
where `min = max(firstVersion, max − window)` and `valid` is whether `active` passes
the run invariant.

**`planning/`**
```
{ name, description, scopePolicy?: {scope: policy},
  migrations: [ {id, version, scope, applies} ],
  from, to, runNow: [id], held: [id] }
```
`runNow`/`held` are the expected step ids (ordered by version then id). When
`scopePolicy` is omitted the built-in default applies (`host-local`→run-now,
`interop`→held); any scope the classifier does not map resolves to `held`.

## How to run them

Each implementation ships a conformance runner that loads these fixtures and asserts
the outcomes. The Go implementation vendors this repo as a **git submodule** and runs
the fixtures as ordinary tests:

```bash
# in the Go implementation (github.com/automa-saga/gatekit)
git submodule update --init --recursive   # pull in the gatekit-spec fixtures
go test ./...                             # the conformance_test.go files load and assert them
```

A new implementation becomes "conformant" by doing the same: load every file named in
`manifest.json`, run the cases, and assert the expected outcomes. Reserved groups
(currently `serialization/`) are empty until the behavior they cover is specified.

## Why this matters for *you*

If you are **choosing** gatekit for a [polyglot](glossary.md#polyglot) system (one
built in more than one language), the fixtures are your guarantee that the Go service
and (later) the Kotlin service will not silently disagree about whether a feature is
on. If you are **implementing** gatekit in a new
language, the fixtures are your to-do list and your definition of done.

## See also

- [Appendix A: the resolution algorithm](algorithm.md) — the rules the `resolution/`
  and `invariant/` fixtures encode.
- [Lesson 5](/tutorial/5-activation-use-cases.md) — the plan computation the
  `planning/` fixtures encode.
