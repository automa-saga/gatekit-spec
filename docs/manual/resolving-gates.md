# Resolving gates

Build a `FeatureGate` from the registry and a **version source**, then ask it
whether a feature is on.

```go
gate, err := gatekit.New(registry, gatekit.StaticVersion(6))
if err != nil {
	return err
}

if gate.Enabled(SplitCatalog) {
	// new path
} else {
	// old path
}
```

The gate is safe for concurrent use. The answer depends only on the code and the
version, never on a network call.

## Version sources

`VersionSource` is the interface `Current() gatekit.Activation`. gatekit does not
decide who owns the value:

- `gatekit.StaticVersion(n)` is a fixed value, good for tests and simple consumers.
- With the migration layer, wire the persisted active version in, for example
  `gatekit.StaticVersion(store.Current())`. See [Activation](/manual/activation.md).

## Precedence

For each feature, highest first:

1. An explicit **override** (`WithOverrides`) wins regardless of version.
2. Otherwise the version-resolved `Default` of the spec in force, except that an
   `Alpha` spec resolves off unless the gate has `WithExperimental(true)`.
3. Otherwise off: an unknown feature, or a feature whose first spec is not yet reached.

```go
gate, err := gatekit.New(
	registry,
	gatekit.StaticVersion(5),
	gatekit.WithOverrides(map[gatekit.Feature]bool{SplitCatalog: true}),
	gatekit.WithExperimental(false),
)
```

## Asking about other versions

`EnabledAt(feature, version)` resolves against an explicit version instead of the
source. It is useful in tests and when planning an upgrade.

## Listing features

`Report()` and `ReportAt(version)` give an operator-facing dry-run view:

```go
fmt.Print(gate.ReportAt(6))
// Features at activation 6:
//   - split-catalog [GA] on  Load the per-minor split catalog
```

`FeatureReport` also exposes the data (`Version`, and per feature its description,
maturity, and state) for your own formatting.

Exact algorithm: [Appendix A](/appendix/algorithm.md).
