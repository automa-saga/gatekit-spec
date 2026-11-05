# pingpong — a live rolling protocol upgrade

Two peers, **ping** and **pong**, are two **real processes** that take turns exchanging
messages over TCP. The message wire format is changing from **v1** to **v2** (v2 adds a
checksum). This demo rolls that change across the two peers with gatekit — with **no
downtime** and **no split-brain** — and does the switch **mid-conversation**. It is the
protocol-coexistence companion to the [kv capstone](../kv/) (the on-disk data migration)
and the live form of the course's
[worked rollout](../../../docs/appendix/worked-example.md).

## Run it (two terminals)

Start pong first; it listens. Then start ping in a second terminal; it connects.

```bash
cd examples/go
go run ./pingpong pong      # terminal 1 — listens, plays "pong"
go run ./pingpong ping      # terminal 2 — connects, plays "ping"
```

Or run both in one process (also what the test drives):

```bash
go run ./pingpong demo
```

## The activation triple on the wire

Each message advertises the sender's gatekit **activation triple**,
`ev=min/max/active` — exactly what `gate.EffectiveVersion()` returns, which the spec
calls *"the triple a binary advertises for rolling upgrades."* It is the whole handshake:

- **max** — the newest activation the sender's binary supports. `max=5` is the old
  binary; `max=6` is the new binary, which can read v2. This is the capability signal.
- **active** — the activation version in force, i.e. what the sender currently *sends*:
  v1 at `active=5`, v2 at `active=6`.
- **min** — the oldest activation still supported (the support window).

A peer sends v2 only once it has seen `max=6` from its partner, and refuses to read a v2
message unless its own `max` is 6. So the triple alone carries both safety decisions.

## The three phases

A real rolling upgrade does not happen all at once. gatekit separates **deploying** the
new code from **activating** the new behavior, which lets the upgrade move through three
phases:

1. **Old binary — `ev=5/5/5`.** Both peers start on the old binary: `max=5`, so they send
   v1 and can *only read* v1. Nobody may send v2 yet — the other side could not parse it.
2. **Rolling deploy — `ev=5/6/5`.** Each peer is upgraded to the new binary **at a
   different time**: `max` rises to 6, so it can now **read both** v1 and v2, but `active`
   is still 5, so it keeps **sending v1**. This mixed-version window is safe precisely
   because the upgraded peer still speaks the old format.
3. **The negotiated flip — `ev=5/6/6`.** Once both peers advertise `max=6`, one proposes a
   **flip message number** in a `flip=` field, in an ordinary **v1** message. Both adopt
   it, and when the conversation reaches that number they advance `active` to 6 — the gate
   turns v2 on, and they **send v2**.

```mermaid
sequenceDiagram
    participant P as ping
    participant Q as pong
    Note over P,Q: Phase 1 - OLD binary, ev=5/5/5 (send v1, read only v1)
    P->>Q: seq1 v1 ev=5/5/5 flip=0
    Q->>P: seq2 v1 ev=5/5/5 flip=0
    Note over Q: pong upgraded, ev=5/6/5 (max=6, reads v1+v2, still sends v1)
    Q->>P: seq4 v1 ev=5/6/5 flip=0
    Note over P: ping upgraded, ev=5/6/5, both now advertise max=6
    Note over P: proposes flip at message 7
    P->>Q: seq5 v1 ev=5/6/5 flip=7
    Note over Q: pong agrees to flip=7
    Q->>P: seq6 v1 ev=5/6/5 flip=7
    Note over P,Q: Phase 3 - message 7 reached, active to 6, send v2
    P->>Q: seq7 v2 ev=5/6/6 flip=7 (crc)
    Note over Q: reads the v2 fine, then sends v2 too
    Q->>P: seq8 v2 ev=5/6/6 flip=7 (crc)
    Note over P,Q: from here on, every message is v2
```

## Why they don't flip at the same instant — and why that's fine

Two independent processes share no clock, so they can't switch at the same *instant*.
They don't need to. The rule is **decode both, send one**: a new binary (`max=6`) always
*understands* v1 and v2, but only *sends* v2 after `active` reaches 6. So when ping sends
the first v2 message, pong — already at `max=6` — reads it even though pong itself sent v1
a moment ago, then sends v2 from its next turn. The one-message skew is harmless because
both can already read either format. What they agree on is not an instant but a **point in
the conversation** (the `flip=` message number), negotiated in-band.

This is the Kafka `inter.broker.protocol.version` pattern: deploy new binaries that
understand both protocols, keep speaking the old one, then raise the agreed version once
everyone is ready.

## Sample output (`demo`)

```
>> seq= 1  v1|from=ping|seq=1|ev=5/5/5|flip=0
...
-- upgraded to the new binary at message 4: supports 5/6/5 (reads v1 AND v2; still sending v1)
-- upgraded to the new binary at message 5: supports 5/6/5 (reads v1 AND v2; still sending v1)
-- both peers now support v2 (Max=6): proposing flip at message 7
-- agreed to flip to v2 at message 7
>> seq= 6  v1|from=pong|seq=6|ev=5/6/5|flip=7
-- reached the agreed message 7: flipping to v2 (sending v2 now)
>> seq= 7  v2|from=ping|seq=7|ev=5/6/6|flip=7|crc=23e508bd
<< seq= 7  v2|from=ping|seq=7|ev=5/6/6|flip=7|crc=23e508bd  (we read v2 though we still send v1 — decode both, send one)
...
-- done after 12 messages; no message ever failed to decode
```

`-upgrade-ping`, `-upgrade-pong` set when each peer is upgraded; `-lead` sets how far
ahead the flip is proposed; `-rounds` the message count. Set `-upgrade-pong` beyond
`-rounds` and watch the flip never happen — the peers hold on v1, because it is never safe
to send v2 to a peer whose `max` is still 5.

## What to watch

- **Dormant after upgrade (`ev=5/6/5`).** Once a peer is upgraded its `max` is 6, but
  `active` stays 5, so it keeps sending v1 — staying compatible with a peer that has not
  upgraded yet. That is the mixed-binary **coexistence** window.
- **The flip point is negotiated on the wire.** The `flip=` number rides in v1 messages,
  so both peers agree on *which message* raises `active` to 6 — no shared config, no shared
  clock.
- **Decode both, send one.** A `max=6` binary always *understands* v1 and v2, but only
  *sends* v2 once `active=6`. An old binary (`max=5`) only understands v1 — which is why an
  upgraded peer must not send v2 until it has seen its partner advertise `max=6`.

## Files

| File | What it holds |
|------|---------------|
| `feature.go` | the `MessageFormatV2` feature, the old/new binary spec sets, and each peer's activation version |
| `message.go` | the `Msg` payload and the v1 / v2 wire encoders + decoder (the triple + checksum) |
| `peer.go` | a peer: its gate (binary version), the rollout phases, and encode/decode |
| `transport.go` | the TCP listen (pong) and dial (ping) sides |
| `demo.go` | runs both peers in one process over an in-memory pipe |
| `main.go` | the `ping` / `pong` / `demo` subcommands and flags |
| `peer_test.go` | drives the demo and verifies the invariants (`go test ./pingpong`) |

The two peers are real processes over a real socket; `demo` runs the same `runPeer` code
over an in-memory pipe so it is deterministic and testable. The "old binary" is modeled by
giving the peer a registry that stops at activation 5, rather than a second build, so the
whole rollout fits in one example.
