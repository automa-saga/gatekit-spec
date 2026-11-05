# Kotlin (coming with the implementation)

> **Status: placeholder.** There is no Kotlin implementation of gatekit yet. This
> page exists so the course's structure is complete; it will be filled in *when a
> Kotlin implementation lands and passes the shared
> [conformance fixtures](/appendix/conformance.md).* Nothing here implies a Kotlin
> library you can install today.

## What will go here

A Kotlin tutorial mirroring the Go lessons, but idiomatic Kotlin:

- declaring a `Feature` and a registry,
- building a gate and resolving features,
- the migration/activation layer and `finalize`.

## Why the concepts carry over unchanged

gatekit is **spec-first**: every implementation must pass the same language-neutral
conformance fixtures, so feature resolution and activation behave identically across
languages. The ideas in [Lessons 1–6](1-why-feature-gating.md) are the real content;
only the syntax will differ here.

To preview the shape, read the [Go lessons](go.md) — the Kotlin API will follow the
same model.

> Interested in building the Kotlin implementation? See
> [CONTRIBUTING](https://github.com/automa-saga/gatekit-spec/blob/main/CONTRIBUTING.md).
> Start by passing the conformance fixtures.
