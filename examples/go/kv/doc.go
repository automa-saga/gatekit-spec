// Command kv is the runnable capstone for the gatekit course: a tiny clustered
// key-value store that changes its record format from v1 to v2 using gatekit. It is
// the executable version of Appendix E's worked rollout.
//
// It ships the change dormant, holds the data migration (the format change is an
// shared change, so the fleet must agree), and runs the migration at a coordinated
// finalize — then, and only then, advances the active version and turns the new
// format on.
//
// Build and run it to watch the rollout:
//
//	cd examples/go
//	go run ./kv          # or: go build ./kv && ./kv
//
// It is deliberately in-memory and single-process so it runs anywhere; a real kv
// would read and write disk and coordinate across nodes. For a crash-safe,
// resumable finalize, swap migration.NewStdRunner for the durable runner in
// github.com/automa-saga/gatekit-automa — the call sites in rollout.go do not change.
package main
