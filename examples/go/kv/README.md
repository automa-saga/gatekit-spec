# kv — the capstone rollout

A tiny, runnable clustered key-value store that changes its record format from v1 to
v2 using gatekit. It is the executable version of
[Appendix E: a worked rollout](../../../docs/appendix/worked-example.md).

## Run it

```bash
cd examples/go
go run ./kv
# or build the binary:
go build ./kv && ./kv
```

You will see the rollout, stage by stage — deploy dormant, the migration held, the
preserve-downgrade window, then finalize running the migration and advancing the
active version:

```
kv rollout: record format v1 -> v2 (activation 5 -> 6)
----------------------------------------------------
deploy    rolled out new binaries, dormant        | active=5 v2=false records=[v1 v1]
plan      "rewrite-records-v2" is interop -> HELD for finalize
runnow    ran host-local steps (none)             | active=5 v2=false records=[v1 v1]
wait      preserve-downgrade window; rollback safe, waiting for readiness
finalize  ran held migration, advanced active     | active=6 v2=true records=[v2 v2]
```

The point to watch: the data migration runs at **finalize**, not at **deploy** —
`records` stays `[v1 v1]` until the final line.

## Files

| File | What it holds |
|------|---------------|
| `record.go` | the record type and an in-memory store (also the `migration.Store`) |
| `feature.go` | the `RecordFormatV2` feature, its registry, and the version source |
| `migration.go` | `RewriteToV2` — the interop (held) data migration |
| `rollout.go` | the deploy → plan → run-now → finalize sequence |
| `main.go` | prints the rollout |
| `rollout_test.go` | verifies the invariants (`go test ./kv`) |

For a crash-safe, resumable finalize, swap `migration.NewStdRunner` in `rollout.go`
for the durable runner in
[gatekit-automa](https://github.com/automa-saga/gatekit-automa) — the call sites do
not change.
