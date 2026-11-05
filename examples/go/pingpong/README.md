# pingpong — a live rolling protocol upgrade

Two peers, **ping** and **pong**, take turns exchanging messages that carry a kv
record. The message wire format is changing from **v1** to **v2** (v2 adds a
checksum). This demo rolls that change across the two peers with gatekit, with **no
downtime** and **no split-brain** — the companion to the [kv capstone](../kv/) (which
focuses on the on-disk data migration) and the live form of the course's
[worked rollout](../../../docs/appendix/worked-example.md).

## Run it

```bash
cd examples/go
go run ./pingpong     # or: go build ./pingpong && ./pingpong
```

```
pingpong: rolling a message format v1 → v2 across two peers
----------------------------------------------------------
start   both on the OLD binary; cluster active=5 → both speak v1
round  1 ping → pong  "v1 ping seq=1 k=1"                 pong reads v1
round  2 pong → ping  "v1 pong seq=2 k=2"                 ping reads v1
rollout pong upgraded to the NEW binary (v2-capable); active still 5, so it stays dormant and keeps sending v1
round  3 ping → pong  "v1 ping seq=3 k=3"                 pong reads v1
round  4 pong → ping  "v1 pong seq=4 k=4"                 ping reads v1  (sender is v2-capable but still dormant → v1)
rollout ping upgraded too → every peer supports v2 (readiness); still active=5, still sending v1
round  5 ping → pong  "v1 ping seq=5 k=5"                 pong reads v1  (sender is v2-capable but still dormant → v1)
round  6 pong → ping  "v1 pong seq=6 k=6"                 ping reads v1  (sender is v2-capable but still dormant → v1)
FINALIZE cluster active 5 → 6; MessageFormatV2 is ON for the whole fleet
round  7 ping → pong  "v2 ping seq=7 k=7 crc=c359b9b2"    pong reads v2
round  8 pong → ping  "v2 pong seq=8 k=8 crc=7153ec2f"    ping reads v2
round  9 ping → pong  "v2 ping seq=9 k=9 crc=9bd12ad4"    pong reads v2
round 10 pong → ping  "v2 pong seq=10 k=10 crc=4b3d7e09"  ping reads v2
done    no peer ever failed to read a message — the rollout was safe
```

## What to watch

- **Dormant after upgrade (rounds 3–6).** Once a peer is upgraded it *can* speak v2,
  but while the shared cluster version is still 5 the gate keeps it on v1 — so it
  stays compatible with the peer that has not upgraded yet. That is the
  mixed-binary **coexistence** window.
- **One coordinated flip (finalize).** Both peers resolve their gate against the
  *same* cluster version, so a single `finalize` (5 → 6) turns v2 on for the whole
  fleet at once. No peer can diverge on its own — no split-brain.
- **Decode both, send one.** A new binary always *understands* v1 and v2, but only
  *sends* v2 after the flip. An old binary (rounds 1–2) only understands v1 — which is
  why an upgraded peer must not send v2 early. Try editing `peer.go` to send v2 before
  finalize and watch the old peer fail to decode.

## Files

| File | What it holds |
|------|---------------|
| `cluster.go` | the `MessageFormatV2` feature, the registry, and the shared cluster version source |
| `message.go` | the message payload and the v1 / v2 wire encoders |
| `peer.go` | a peer: its binary capability, its gate, and encode/decode |
| `demo.go` | the ping-pong rounds + the rollout/finalize schedule |
| `main.go` | prints the run |
| `demo_test.go` | verifies the invariants (`go test ./pingpong`) |

This is a single-process, deterministic simulation so it runs anywhere and stays
testable; a real deployment would run separate processes sharing the cluster version.
