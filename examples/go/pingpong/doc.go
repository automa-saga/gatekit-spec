// Command pingpong is a live demo of a rolling, zero-downtime protocol upgrade with
// gatekit. Two peers — ping and pong — take turns exchanging messages that carry a kv
// record. The message wire format is changing from v1 to v2 (v2 adds a checksum).
//
// It shows the whole rolling-upgrade story in one run:
//
//  1. Both peers start on the OLD binary and speak v1.
//  2. Each peer is upgraded to the NEW binary, which can speak and understand BOTH v1
//     and v2 — but while the cluster's activation version is still 5, the feature is
//     dormant, so an upgraded peer keeps SENDING v1. Mixed binaries coexist.
//  3. Once every peer is upgraded (readiness), a single coordinated finalize advances
//     the shared cluster version 5 → 6. Now both peers send v2 — together.
//
// The point to watch: an upgraded, v2-capable peer still sends v1 until the
// coordinated finalize. That is what lets old and new binaries coexist during the
// rollout without a split-brain.
//
// Run it:
//
//	cd examples/go
//	go run ./pingpong    # or: go build ./pingpong && ./pingpong
//
// It is a single-process, deterministic simulation so it runs anywhere and stays
// testable; a real deployment would run separate processes that share the cluster
// version. For a crash-safe finalize that also migrates stored data, see the kv
// capstone and github.com/automa-saga/gatekit-automa.
package main
