# Declaring features

A **feature** is a named, gated behavior. You declare each one once, in a
`Registry`, with a list of **versioned specs** describing how it changes over the
activation version.

## Name the feature with a typed constant

Declare each identifier once as a constant and use it everywhere, so a typo is a
compile error rather than a feature that silently resolves off.

```go
const SplitCatalog gatekit.Feature = "split-catalog"
```

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

## Choosing the activation version

The activation version grows by **at most 1 per release**, and only when you ship a
contract-breaking change. Many releases can share one version. Never hand-edit the
supported range; it is derived from the registry. See
[Activation](/manual/activation.md).

Normative rules: [core spec](https://github.com/automa-saga/gatekit-spec/blob/main/core-spec.md).
Worked walkthrough: [Lesson 4](/tutorial/4-build-and-release-a-feature.md).
