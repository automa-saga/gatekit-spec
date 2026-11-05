# Appendix A: The resolution algorithm

> This appendix states the rules precisely, for when you need the exact behavior
> rather than the tutorial version. It restates
> [core-spec §3–5](https://github.com/automa-saga/gatekit-spec/blob/main/core-spec.md)
> and [activation-spec §2–3](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md)
> in one place. Where this page and a spec disagree, **the spec is authoritative.**

## Inputs

Resolution is a deterministic function of the gate's own inputs, with no dependence
on external state:

- the **registry** — a map from each `Feature` to its `FeatureDef` (an optional
  `description` plus `VersionedSpecs`), compiled into the binary,
- the **active** activation version `v` — a non-negative integer supplied by the
  version source, and
- the gate's **construction-time configuration** — any per-feature overrides and
  whether `ALPHA` is opted in (`allowAlpha`), both fixed when the gate is built.

It depends on **nothing else**: not the wall clock, not the environment, not call
order. core-spec §3 states resolution as a pure function of the registry and the
version; the overrides (§5) and the `ALPHA` opt-in (§4) are the gate's own fixed
configuration, applied in the precedence below — they are inputs, not external state.

## Registry validity

A registry MUST be rejected if, for any feature:

- its `VersionedSpecs` is empty, or
- any spec has a negative `activeFrom` (an implementation MAY make this
  unrepresentable with an unsigned type), or
- two specs share the same `activeFrom`.

Before resolution, each feature's specs are sorted by `activeFrom` ascending.
Declaration order does not matter.

## The effective spec

Given a feature `f` and the active version `v`, the **effective spec** is:

> the spec of `f` with the **greatest `activeFrom` that is ≤ `v`**.

In pseudocode:

```
effectiveSpec(f, v):
    specs = registry[f].specs        # sorted ascending by activeFrom
    result = none
    for s in specs:
        if s.activeFrom > v:
            break                    # all later specs are also > v
        result = s                   # last one with activeFrom <= v wins
    return result
```

## Resolution

```
Enabled(f, v):
    # 1. explicit override wins, regardless of version
    if f in overrides:
        return overrides[f]

    # 2. find the effective spec
    s = effectiveSpec(f, v)
    if s is none:                    # f unknown, or first spec not yet reached
        return false

    # 3. ALPHA is off unless opted in (allowAlpha is set by WithExperimental)
    if s.maturity == ALPHA and not allowAlpha:
        return false

    # 4. otherwise the effective spec's default
    return s.default
```

### Precedence, highest first

1. **Explicit override** (`WithOverrides`) — forces on/off regardless of `v`,
   including before a feature's first spec is reached. An override MAY name only a
   registered feature.
2. **Version-resolved default** — the effective spec's `default`, *except* an `ALPHA`
   spec resolves **off** unless the gate is opted in (`WithExperimental`).
3. **Off** — the feature is unknown, or has no spec with `activeFrom ≤ v`.

### Maturity rules

- `ALPHA` — not guaranteed across versions; resolves **off** unless explicitly opted
  in. Authors need not preserve alpha behavior across versions.
- `BETA`, `GA` — resolve by their declared `default`.
- `DEPRECATED` — resolves by `default` like BETA/GA, but SHOULD be reported by lints
  as due for removal.

`description` is informational only and MUST NOT affect any step above.

## The version triple

The binary advertises three derived numbers (activation-spec §2):

```
max   = MaxActivation(registry)              # greatest activeFrom across all specs, or 0
W     = support window                        # configured; default 1, non-negative
min   = max(firstVersion, max − W)            # firstVersion = lowest activeFrom, 0 if empty
active = versionSource.Current()              # the v used by Enabled
```

`max − W` is floored at 0 (the activation version is unsigned, so it never
underflows).

## The run invariant

At startup an implementation MUST verify:

```
min ≤ active ≤ max
```

If it fails, the implementation MUST refuse to operate and return a typed error
(`VersionRangeError` in Go). `active < min` means the binary is too new for a
far-behind state; `active > max` means the binary is too old.

## Worked example

Registry:

```
SplitCatalog:
    { activeFrom: 5, default: false, maturity: BETA }
    { activeFrom: 6, default: true,  maturity: GA   }
NewIndexer:
    { activeFrom: 7, default: true,  maturity: ALPHA }
```

So `max = 7`. With `W = 1`, `firstVersion = 5`, `min = max(5, 7−1) = 6`.

| `v` | `SplitCatalog` | `NewIndexer` | notes |
|-----|-----------------|---------------|-------|
| 4 | off | off | no spec with `activeFrom ≤ 4` for either |
| 5 | off | off | SplitCatalog's 5-spec: default off |
| 6 | **on** | off | SplitCatalog's 6-spec: default on, GA |
| 7 | on | off | NewIndexer reached but ALPHA → off unless opted in |
| 7 + `allowAlpha` | on | **on** | ALPHA opt-in lets its default apply |
| 7 + override `NewIndexer=false` | on | off | override beats the default |

Run invariant at `max = 7`, `min = 6`: `active` must be 6 or 7. An `active` of 5
would make the binary refuse to start.

## See also

- [Appendix B: conformance fixtures](/appendix/conformance.md) — the JSON cases that pin this
  algorithm down across languages.
- [Glossary](/appendix/glossary.md) — every term above, defined.
