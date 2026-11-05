// Command pingpong is a live demo of a rolling, zero-downtime protocol upgrade with
// gatekit. ping and pong are two real processes that take turns exchanging messages over
// TCP. The message wire format is changing from v1 to v2 (v2 adds a checksum).
//
// It shows the whole rolling-upgrade story in one conversation, in three phases:
//
//  1. Both peers start on the OLD binary: they send v1 and can read only v1.
//  2. Each peer is upgraded to the NEW binary at a different time (a rolling deploy). The
//     new binary reads BOTH v1 and v2, but while its activation version is still 5 the
//     feature is dormant, so it keeps SENDING v1. Mixed binaries coexist.
//  3. Once both are upgraded, one proposes a flip message number on the wire (a `flip=`
//     field in a v1 message); both adopt it and switch to SENDING v2 at that message.
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
