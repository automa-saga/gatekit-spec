# Activation

This chapter covers the version model behind a rolling upgrade: the
`{min, max, active}` triple, the run invariant, and the two activation modes.

## Two version axes

- The **release version** is your artifact's version (for example semver).
- The **activation version** is a non-negative, monotonically increasing integer
  that moves only when a gated, contract-breaking behavior ships.

gatekit never equates them. Many releases can share one activation version.

## The triple

```go
ev := gate.EffectiveVersion()
fmt.Printf("min=%d max=%d active=%d\n", ev.Min, ev.Max, ev.Active)
```

| Field | How it is set |
|---|---|
| `Max` | The highest `ActiveFrom` in the registry (`gate.MaxActivation()`). Derived, never hand-maintained. |
| `Min` | `max(firstActivation, Max - W)`, where `W` is the support window. |
| `Active` | Read from the `VersionSource`; the value resolution uses. |

### Support window

`W` defaults to 1 (strict N/N+1: the binary supports the current and one prior
activation version). Widen it with `WithSupportWindow`:

```go
gate, err := gatekit.New(registry, source, gatekit.WithSupportWindow(3))
```

`W` must be non-negative.

## Run invariant

At startup, refuse to run if `active` is outside `[min, max]`:

```go
if err := gate.Check(); err != nil {
	var rangeErr *gatekit.VersionRangeError
	if errors.As(err, &rangeErr) {
		log.Fatalf("unsupported activation %d (supported %d..%d)",
			rangeErr.Active, rangeErr.Min, rangeErr.Max)
	}
	return err
}
```

Below `min` means this binary is too new for a far-behind state; above `max`
means it is too old. This guard stops a stale binary from undermining a flip and
forces catch-up to proceed within the window.

## Immediate vs. deferred activation

| Mode | What happens | Valid for |
|---|---|---|
| **Immediate** | `active` is set to the migration's version on install and the migration runs then. | `host-local` migrations |
| **Deferred** | The binary keeps operating at the prior `active`, reads older formats down to `min`, and holds the migration until `finalize`. | Required for `interop`; optional for `host-local` |

Use deferred activation whenever mixed versions can coexist in a fleet. Details
and the migration side: [Migrations](/manual/migrations.md) and
[Finalize & rollback](/manual/finalize-and-rollback.md).

Normative rules: [activation spec](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md).
Use cases: [Lesson 5](/tutorial/5-activation-use-cases.md).
