# Testing against conformance

The specification ships **conformance fixtures**: plain-JSON inputs and expected
outcomes that every implementation must pass. If you maintain an implementation, or
want to confirm yours agrees with the spec, run them.

## Groups

| Group | Checks | Spec |
|---|---|---|
| `resolution/` | Feature resolution, including alpha and overrides | core §3–5 |
| `invariant/` | The run invariant `min <= active <= max` | activation §3 |
| `planning/` | Run-now vs. held plan computation | migration §2 |
| `serialization/` | Reserved; empty until the formats are fixed | activation §8 |

Every fixture is listed in
[`conformance/manifest.json`](https://github.com/automa-saga/gatekit-spec/blob/main/conformance/manifest.json),
and each group's schema is in
[`conformance/README.md`](https://github.com/automa-saga/gatekit-spec/blob/main/conformance/README.md).

## Run them

Vendor this repository (the Go implementation uses a git submodule), then load and
assert each fixture in a test. The Go implementation's runner skips when the
submodule is absent, so initialize it first:

```bash
git submodule update --init
```

## Validate a fixture you add

New fixtures must be registered in `conformance/manifest.json`, or CI rejects them.
See [CONTRIBUTING](https://github.com/automa-saga/gatekit-spec/blob/main/CONTRIBUTING.md)
for the validation command.

Background: [Appendix B](/appendix/conformance.md).
