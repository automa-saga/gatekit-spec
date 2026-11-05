# Troubleshooting

## A feature is off and I expected it on

Work down the precedence list in [Resolving gates](/manual/resolving-gates.md):

1. **Typo or unregistered name.** An unknown feature is always off. Use typed
   constants. (An override for an unknown feature is rejected by `New`, so that
   case fails loudly.)
2. **First spec not reached.** The active version is below the feature's lowest
   `ActiveFrom`.
3. **`Default: false`.** The spec in force is dormant. Check the spec with the
   highest `ActiveFrom` at or below the active version.
4. **`Alpha` maturity.** Alpha is off unless `WithExperimental(true)` or an override.
5. **An override.** `WithOverrides` beats everything, including the version.

Print the state to see which spec is in force:

```go
fmt.Print(gate.Report())
```

## `Check` returns `*VersionRangeError`

The active version is outside `[Min, Max]`.

- **`Active > Max`:** the binary is older than the state. Upgrade the binary.
- **`Active < Min`:** the binary is too new for a state that fell behind the
  support window. Catch the state up in steps, or widen the window with
  `WithSupportWindow`.

## A migration did not run

- Is its `Version()` in the half-open range `(from, to]`? A migration at `from` is
  excluded.
- Does `Applies` return true? It reads ground truth; a satisfied migration is omitted.
- Is its scope **held**? Interop migrations run at `Finalize`, not `RunNow`. A scope
  your policy does not map also resolves to held.

## `Finalize` failed

`active` is unchanged and executed steps were rolled back. Fix the cause and re-run.
With the durable runner, completed steps are skipped on re-run, so `Execute` must be
idempotent.

## More

[Appendix D: spot the failure](/appendix/spot-the-failure.md) · [FAQ](/faq.md) ·
[Glossary](/appendix/glossary.md)
