# Go examples

Runnable programs that use the real gatekit API, cited by the gatekit course (see
`../../docs`). Each is a `main` package you can build and run — and each ships a `go
test` that drives the same code, so the behavior shown in the lessons stays correct
against the released gatekit API.

## Run them

These examples depend on the [gatekit](https://github.com/automa-saga/gatekit) module.
They compile and run once that module is resolvable (public and/or tagged — the
examples pin `v0.4.0`):

```bash
cd examples/go
go run ./kv          # the on-disk data migration at finalize
go run ./pingpong    # two peers coexisting, then flipping the wire format together
go test ./...        # drive the same code under test
```

Working on gatekit and this repo side by side? Point the module at your local
checkout without committing the change:

```bash
go mod edit -replace github.com/automa-saga/gatekit=../../../gatekit
go test ./...
git checkout go.mod   # drop the replace before committing
```

## What's here

| Example | Lessons | What it is |
|---------|---------|------------|
| [`kv/`](kv/) | Appendix E | A **key-value store** with real `Put`/`Get` whose on-disk record format rolls from v1 to v2. The format a write uses is chosen by the gate, so the same `Put` writes v1 while dormant and v2 after a coordinated finalize — the data-migration story. |
| [`pingpong/`](pingpong/) | Appendix E | **Two peers** rolling a message wire format from v1 to v2 with no downtime — the protocol-coexistence story. |

Both are the executable form of the course's
[worked rollout](../../docs/appendix/worked-example.md): kv is the on-disk data
migration, pingpong is the wire-protocol coexistence.
