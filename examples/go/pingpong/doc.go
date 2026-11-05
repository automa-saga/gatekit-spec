// Command pingpong is a live demo of a rolling, zero-downtime protocol upgrade with
// gatekit. ping and pong are two real processes that take turns exchanging messages over
// TCP. The message wire format is changing from v1 to v2 (v2 adds a checksum).
//
// Each message advertises the sender's gatekit activation triple (gate.EffectiveVersion)
// as ev=min/max/active. max is the capability signal: max=5 is the old binary, max=6 the
// new one, which can read v2. The story runs in three phases:
//
//  1. Old binary (ev=5/5/5): both peers send v1 and can read only v1.
//  2. Rolling deploy (ev=5/6/5): each peer is upgraded at a different time, so max rises
//     to 6 (it can now read BOTH v1 and v2) but active is still 5, so it keeps SENDING v1.
//     Mixed binaries coexist.
//  3. Negotiated flip (ev=5/6/6): once both advertise max=6, one proposes a flip message
//     number on the wire (a `flip=` field in a v1 message); both adopt it and advance
//     active to 6 at that message, so the gate turns v2 on for sending.
//
// The point to watch: an upgraded, v2-capable peer still sends v1 until the agreed flip,
// and the two peers do not flip at the same instant — a new binary always reads both
// formats, so the one-message skew between them is safe (decode both, send one). That is
// what lets old and new binaries coexist during the rollout without a split-brain.
//
// Run it in two terminals (pong first, it listens; then ping, it connects):
//
//	cd examples/go
//	go run ./pingpong pong
//	go run ./pingpong ping
//
// Or run both in one process, which is also what the test drives:
//
//	go run ./pingpong demo
//
// The two peers are real processes over a real socket; demo runs the same code over an
// in-memory pipe so it is deterministic and testable. The "old binary" is modeled by a
// flag on the peer rather than a second build, so the whole rollout fits in one example.
// For a crash-safe finalize that also migrates stored data, see the kv capstone and
// github.com/automa-saga/gatekit-automa.
package main
