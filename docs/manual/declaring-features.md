# Declaring features

A **feature** is a named, gated behavior. You declare each one once, in a
`Registry`, with a list of **versioned specs** describing how it changes over the
activation version.

## Name the feature with a typed constant

Declare each identifier once as a constant and use it everywhere, so a typo is a
compile error rather than a feature that silently resolves off.

```go
const SplitCatalog gatekit.Feature = "SplitCatalog"
```

Spell the identifier in **CamelCase** (`SplitCatalog`, not `split-catalog`).
`WithEnvOverrides` derives an environment-variable name from it, and only a
CamelCase identifier maps to a valid `UPPER_SNAKE_CASE` variable — see
[Resolving gates](/manual/resolving-gates.md).

## Declare its lifecycle

```go
var registry = gatekit.Registry{
	SplitCatalog: {
		Description: "Load the per-minor split catalog",
		Specs: gatekit.VersionedSpecs{
			{ActiveFrom: 5, Default: false, Maturity: gatekit.Beta},
			{ActiveFrom: 6, Default: true, Maturity: gatekit.GA},
		},
	},
}
```

Each `FeatureSpec` has three fields:

| Field | Meaning |
|---|---|
| `ActiveFrom` | The activation version at which this spec takes effect. |
| `Default` | Whether the feature is on from `ActiveFrom` onward, until a later spec overrides it. |
| `Maturity` | The lifecycle stage: `Alpha`, `Beta`, `GA`, or `Deprecated`. |

The spec in force is the one with the highest `ActiveFrom` that is at or below the
current activation version. Order at declaration time does not matter; gatekit sorts
when you build the gate.

`Description` is optional and informational. It appears in reports but never
affects resolution.

`RemoveBy` is an optional activation version by which the feature must be deleted.
It is lint metadata only: it never affects resolution. See
[Removing features](#removing-features-the-lint).

## Maturity

| Maturity | Behavior |
|---|---|
| `Alpha` | Off unless the gate is opted in with `WithExperimental(true)` (or the feature is overridden), even if `Default` is true. |
| `Beta`, `GA` | Follow `Default`. |
| `Deprecated` | Follows `Default`; marks a feature on its way out. |

## Rules `gatekit.New` enforces

`New` returns an error if:

- the `VersionSource` is `nil`,
- a feature has no specs,
- a feature has two specs with the same `ActiveFrom`,
- an override names a feature that is not in the registry,
- the support window is negative.

## Removing features: the lint

A gate that is never deleted is a bug, and moving `min` forward breaks nothing, so
nobody notices. `gatekit.Lint` makes it a test failure. Call it from your own tests; gatekit never
runs it for you.

```go
func TestRegistryLifecycle(t *testing.T) {
	violations, err := gatekit.Lint(registry, gatekit.WithSupportWindow(1))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range violations {
		t.Errorf("%s: %s (spec %d)", v.Feature, v.Kind, v.ActiveFrom)
	}
}
```

| Kind | Reported when |
|---|---|
| `StaleSpec` | a later spec has `ActiveFrom <= Min`, so this one can never be selected |
| `PastRemoveBy` | `RemoveBy` is set and `Min >= RemoveBy` |
| `DeprecatedDue` | the latest spec is `Deprecated` |

- Pass the **same** support window you give `New`; `Min` depends on it.
- Keep it in tests or CI. Use `gate.Check()` at startup instead; a hygiene problem must
  not stop an operator's binary.
- `DeprecatedDue` fires on every run until the feature is deleted. Skip that kind in
  your loop if you only want to enforce the window.
- Lint reports the spec, not your code. Search for the feature constant to find the
  branches to delete.

Normative rules: [core spec §8](https://github.com/automa-saga/gatekit-spec/blob/main/core-spec.md).

## Choosing the activation version

The activation version grows by **at most 1 per release**, and only when you ship a
contract-breaking change. Many releases can share one version. Never hand-edit the
supported range; it is derived from the registry. See
[Activation](/manual/activation.md).

Normative rules: [core spec](https://github.com/automa-saga/gatekit-spec/blob/main/core-spec.md).
Worked walkthrough: [Lesson 4](/tutorial/4-build-and-release-a-feature.md).
