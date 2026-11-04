# Examples

Runnable examples cited by the gatekit course (`../docs`).

- [`go/`](go/) — **real, runnable** examples against the released gatekit Go API,
  verified by `go test`. Includes [`go/kv/`](go/kv/), a runnable **capstone binary**
  (`cd go && go run ./kv`) that rolls a record-format change across a clustered store
  — the executable form of the course's [worked rollout](../docs/appendix/worked-example.md).
- `kotlin/`, `rust/`, `python/` — **placeholders.** These are added as the
  corresponding implementations mature and pass the shared
  [conformance fixtures](../conformance). Until then their lessons are
  clearly-marked placeholders so the site never implies a library that does not exist
  yet.

Go is the reference implementation; the non-Go examples are spec-illustrative until
their implementations land.
