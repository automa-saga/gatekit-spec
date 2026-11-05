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

`VersionSource` is the interface `Current() gatekit.Activation`, called on every
`Enabled`. gatekit does not decide who owns the value:

- `gatekit.StaticVersion(n)` is a fixed value, good for tests and simple consumers.
- To **activate later** — ship dormant, then flip on a trigger or an operator
  command — read the version from a persistent store, so advancing it (normally via
  the migration layer's `finalize`) changes resolution with no new build.

Mind one sharp edge: `gatekit.StaticVersion(store.Current())` takes a **snapshot** at
construction, so a later `SetActive` does not reach that gate until it is rebuilt.
For a live flip, give the gate a source that reads the store on each call:

```go
type storeSource struct{ s migration.Store }

func (ss storeSource) Current() gatekit.Activation {
	return gatekit.Activation(ss.s.Current())
}

gate, _ := gatekit.New(reg, storeSource{store}) // Enabled now tracks the store live
```

You can also drive the version from an environment variable (a few lines over
`os.Getenv`), which suits a CLI or a dev setup. But a bare env var bypasses
`finalize`: it moves `active` **without running that version's held migrations**, and
it is per-process rather than coordinated fleet state. Keep the activation version in
a store for any fleet with interop migrations. See [Activation](/manual/activation.md)
and [Implementing activation](/manual/implementing-activation.md).

## Precedence

For each feature, highest first:

1. An explicit **override** wins regardless of version — set in code with
   `WithOverrides`, or from the environment with `WithEnvOverrides("PREFIX_")`, which
   maps each `PREFIX_<FEATURE>` variable to a feature via `EnvVarName` (the feature
   identifier in `UPPER_SNAKE_CASE`) and errors on an unknown or non-boolean one.
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
//   - SplitCatalog [GA] on  Load the per-minor split catalog
```

`FeatureReport` also exposes the data (`Version`, and per feature its description,
maturity, and state) for your own formatting.

Exact algorithm: [Appendix A](/appendix/algorithm.md).
