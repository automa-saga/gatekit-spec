# Other languages

gatekit is a **spec-first** project. The product is the
[specification](https://github.com/automa-saga/gatekit-spec) and the shared
[conformance fixtures](/appendix/conformance.md) — not any single implementation.
That is what lets an activation state written by one language be read identically by
another.

## Go — the reference implementation

Go is the **first conformant implementation** and the one every example in this
course uses. It is real, released code.

- **Module:** [`github.com/automa-saga/gatekit`](https://github.com/automa-saga/gatekit)
- **Gate core:** `github.com/automa-saga/gatekit` — the registry and `FeatureGate`.
  Dependency-light; drop it into any Go project.
- **Migration / activation:** `github.com/automa-saga/gatekit/migration` — deferred
  activation, migrations, `finalize`. Opt-in.
- **Durable runner:** [`github.com/automa-saga/gatekit-automa`](https://github.com/automa-saga/gatekit-automa)
  — a crash-resumable `Runner` backed by the automa saga engine, swappable in behind
  the same interface.

Install:

```bash
go get github.com/automa-saga/gatekit
```

The runnable versions of this course's snippets live in
[`examples/go/`](https://github.com/automa-saga/gatekit-spec/tree/main/examples/go)
and run as Go example tests.

## Kotlin, Rust, Python — coming with the implementation

The pages below are **placeholders**. The lessons and examples for these languages
are written *as each implementation lands and passes the shared conformance
fixtures* — so this site never implies a library that does not exist yet.

- [Kotlin (coming)](/tutorial/kotlin.md)
- [Rust (coming)](/tutorial/rust.md)
- [Python (coming)](/tutorial/python.md)

Because every implementation must pass the same fixtures, the *concepts* in Lessons
1–6 are identical across languages. Only the syntax changes. If you have read the Go
lessons, you will understand the others as they arrive; you are waiting only on the
code.

> Want to help bring one of these to life? See
> [CONTRIBUTING](https://github.com/automa-saga/gatekit-spec/blob/main/CONTRIBUTING.md)
> — a new implementation starts by passing the conformance fixtures.
