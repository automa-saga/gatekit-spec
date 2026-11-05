# kv — the capstone rollout

A tiny, runnable key-value store with real `Put` and `Get` that changes its on-disk
record format from v1 to v2 using gatekit. It is the executable version of
[Appendix E: a worked rollout](../../../docs/appendix/worked-example.md).

The format a write uses is chosen by the gate, which resolves against the store's live
active version — so the **same `Put` call** writes v1 while the feature is dormant and
v2 after finalize, with no change at the call site. `Get` reads records back in
whichever format they were stored in, so v1 and v2 records coexist. v2 adds a CRC32
checksum that `Get` verifies on read.

## Run it

```bash
cd examples/go
go run ./kv
# or build the binary:
go build ./kv && ./kv
```

You will see the upgrade, stage by stage — two writes while dormant (stored v1), the
migration held for finalize, the preserve-downgrade window, then finalize migrating the
records and advancing the active version, a third write that now lands in v2, and every
record read back:

```
kv: record format v1 -> v2 (activation 5 -> 6)
----------------------------------------------
put       alpha, beta while dormant           | active=5 v2=false formats=[v1 v1]
plan      "rewrite-records-v2" is shared -> HELD for finalize
runnow    ran host-local steps (none)          | active=5 v2=false formats=[v1 v1]
wait      preserve-downgrade window; rollback still safe
finalize  migrated records, advanced active    | active=6 v2=true formats=[v2 v2]
put       gamma after finalize                 | active=6 v2=true formats=[v2 v2 v2]
get       alpha = 1   (stored v2)
get       beta  = 2   (stored v2)
get       gamma = 3   (stored v2)
```

What to watch:

- **The migration runs at `finalize`, not at `put`.** `formats` stays `[v1 v1]` through
  deploy, plan, and run-now; only finalize rewrites the records to v2.
- **The write format follows the gate.** `alpha` and `beta` are written while dormant
  (v1); `gamma` is written after finalize (v2) by the *same* `Put` — the call site never
  names a version.
- **Reads don't care about format.** After finalize, `Get` reads `alpha` (migrated to
  v2) and would read a v1 record just as well — that is what makes a staggered rollout
  safe.

## Files

| File | What it holds |
|------|---------------|
| `record.go` | the `Record` type and the v1 / v2 on-disk encoders + decoder (the checksum lives here) |
| `store.go` | the in-memory `Store` (also the `migration.Store`) |
| `kv.go` | the `KV` API: `Put`/`Get`, with the write format chosen by the gate |
| `feature.go` | the `RecordFormatV2` feature, its registry, and the version source |
| `migration.go` | `RewriteToV2` — the Shared (held-for-finalize) data migration |
| `rollout.go` | the deploy → plan → run-now → finalize → read-back sequence |
| `main.go` | wires the store and KV together and runs it |
| `kv_test.go` | verifies the invariants (`go test ./kv`) |

For a crash-safe, resumable finalize, swap `migration.NewStdRunner` in `rollout.go`
for the durable runner in
[gatekit-automa](https://github.com/automa-saga/gatekit-automa) — the call sites do
not change.
