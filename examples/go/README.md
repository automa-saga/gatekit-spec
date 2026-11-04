# Go examples

Runnable, real-API examples cited by the gatekit course (see `../../docs`). Each
`Example` function is executed by `go test` and its printed output is checked against
the `// Output:` block, so the snippets shown in the lessons stay correct against the
released gatekit API.

## Run them

These examples depend on the [gatekit](https://github.com/automa-saga/gatekit) module.
They compile and run once that module is resolvable (public and/or tagged — the
examples pin `v0.3.0`):

```bash
cd examples/go
go test ./...
```

Working on gatekit and this repo side by side? Point the module at your local
checkout without committing the change:

```bash
go mod edit -replace github.com/automa-saga/gatekit=../../../gatekit
go test ./...
git checkout go.mod   # drop the replace before committing
```

## What's here

| File | Lessons | Shows |
|------|---------|-------|
| `gate_example_test.go` | 4 | declare a feature, ship it dormant, flip it on, override, alpha opt-in, the dry-run report |
| `activation_example_test.go` | 2, 5, 6 | the `{min,max,active}` triple + run invariant, migration planning (run-now vs held), and `finalize` |
| [`kv/`](kv/) | Appendix E | a **runnable capstone binary** — a clustered key-value store rolling a record format from v1 to v2: `go run ./kv` |

## Run the capstone

```bash
go run ./kv          # watch the kv rollout stage by stage
```
