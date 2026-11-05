# gatekit

**gatekit** is a version-keyed feature-gate and rolling-upgrade mechanism: ship code
dormant, turn it on later by moving one number, and upgrade a running fleet one
machine at a time without downtime.

For a multi-language project the **product is the specification**; every
implementation must behave identically. This site is the human-friendly front door
to the specification, the conformance fixtures, and the implementations.

## Where to go

| I want to… | Go to |
|---|---|
| **Learn** the ideas from first principles, with runnable examples | [Tutorials](/tutorial/index.md) |
| **Do** a specific task with gatekit (install, gate a feature, run a migration, finalize) | [User manual](/manual/index.md) |
| Look up the **exact rules** (resolution algorithm, conformance, design decisions) | [Reference & appendix](/appendix/algorithm.md) |
| Find a term | [Glossary](/appendix/glossary.md) |
| Get quick answers | [FAQ](/faq.md) |
| Read the **normative specs** | [core](https://github.com/automa-saga/gatekit-spec/blob/main/core-spec.md), [activation](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md), [migration](https://github.com/automa-saga/gatekit-spec/blob/main/migration-spec.md) |
| **Add** a tutorial or manual page | [Contributing to the docs](/contributing-docs.md) |

## Repositories

- [gatekit-spec](https://github.com/automa-saga/gatekit-spec) — the specification,
  conformance fixtures, and this site.
- [gatekit](https://github.com/automa-saga/gatekit) — the Go reference
  implementation.
- [gatekit-automa](https://github.com/automa-saga/gatekit-automa) — a durable,
  crash-resumable migration runner for the Go implementation.

## Languages

Go is the reference implementation. Kotlin, Rust, and Python are planned; see
[Other languages](/tutorial/go.md).
