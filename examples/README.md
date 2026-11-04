# Examples

Runnable examples cited by the gatekit course (`../docs`).

- [`go/`](go/) — **real, runnable** examples against the released gatekit Go API,
  verified by `go test`. Includes two runnable **capstone binaries** (the executable
  form of the course's [worked rollout](../docs/appendix/worked-example.md)):
  [`go/kv/`](go/kv/) (`cd go && go run ./kv`) rolls an on-disk record-format change at
  finalize, and [`go/pingpong/`](go/pingpong/) (`cd go && go run ./pingpong`) rolls a
  message wire format across two coexisting peers.
- `kotlin/`, `rust/`, `python/` — **placeholders.** These are added as the
  corresponding implementations mature and pass the shared
  [conformance fixtures](../conformance). Until then their lessons are
  clearly-marked placeholders so the site never implies a library that does not exist
  yet.

Go is the reference implementation; the non-Go examples are spec-illustrative until
their implementations land.
