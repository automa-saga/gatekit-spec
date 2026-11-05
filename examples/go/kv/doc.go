// Command kv is the runnable capstone for the gatekit course: a tiny key-value store
// that changes its on-disk record format from v1 to v2 using gatekit. It is the
// executable version of Appendix E's worked rollout.
//
// The store supports real Put and Get. The format a Put writes is chosen by the gate,
// which resolves against the store's live active version — so the same Put call writes
// v1 while the feature is dormant and v2 once it is active, with no code change at the
// call site. Get reads records back in whichever format they were stored in, so v1 and
// v2 records coexist. v2 adds a CRC32 integrity checksum that Get verifies on read.
//
// The run ships the change dormant, holds the data migration (the format change is
// Shared, so it is deferred until the upgrade is committed), and runs the migration at
// a coordinated finalize — then, and only then, advances the active version and turns
// the new format on. Once records are v2, an older binary rolled back onto this data
// could not read them; that is why the rewrite waits for finalize.
//
// Build and run it to watch the rollout:
//
//	cd examples/go
//	go run ./kv          # or: go build ./kv && ./kv
//
// It is deliberately in-memory and single-process so it runs anywhere; a real kv would
// read and write disk. For a crash-safe, resumable finalize, swap migration.NewStdRunner
// for the durable runner in github.com/automa-saga/gatekit-automa — the call sites in
// rollout.go do not change.
package main
