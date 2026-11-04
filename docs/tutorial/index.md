# Tutorials: releasing features & rolling upgrades

A short course on two skills every product engineer eventually needs:

1. How to **release a feature safely** — ship the code now, turn it on later, and
   never force a risky all-at-once switch.
2. How to **design a product that upgrades without downtime** — so a running fleet
   of servers can move to a new version one machine at a time, while users keep
   using it.

This is a course, not a reference: for task-oriented how-tos see the [User manual](../manual/index.md). It explains the ideas from first
principles, for someone who may have never thought about "feature gating" before.
**gatekit** — a small library that does exactly this — is the worked example that
makes the ideas concrete.

The source and issues live on GitHub:
[gatekit-spec](https://github.com/automa-saga/gatekit-spec) (this site and the
specification) and [gatekit](https://github.com/automa-saga/gatekit) (the Go
reference implementation).

## Who this is for

- A **new engineer** shipping their first change to a system that is always on.
- A **graduate** who knows how to write code but has not yet shipped it to a live
  fleet of machines.
- A **technical leader** deciding how their teams should release and upgrade.

You need a little programming background and nothing else. Every term is defined
the first time it is used. The reading level is plain and the examples are
concrete.

## What you will be able to do

By the end you should be able to, on your own product:

- decide **when a change needs a gate** and when it does not,
- ship a half-finished feature to production safely, **turned off**,
- turn it on later by moving **one number**, and roll it back if it misbehaves,
- reason about an upgrade across many machines without taking the service down,
- and tell when to reach for gatekit versus a tool like LaunchDarkly or
  Kubernetes' built-in feature gates.

## The six lessons

| # | Lesson | The question it answers |
|---|--------|-------------------------|
| [1](1-why-feature-gating.md) | Why feature gating is needed | What problem are we actually solving? |
| [2](2-why-gatekit.md) | Why gatekit is built this way | Why *this* design, and not a flag service? |
| [3](3-contrast-with-existing-patterns.md) | How it compares to other tools | LaunchDarkly, Kubernetes, CockroachDB, Kafka — which, when? |
| [4](4-build-and-release-a-feature.md) | Build and release a feature | Hands-on: declare a feature and ship it dormant. |
| [5](5-activation-use-cases.md) | Activation use cases | One version axis or two? Immediate or deferred? |
| [6](6-rolling-upgrade-thinking.md) | Rolling-upgrade thinking | How to design a product that upgrades with no downtime. |

Read them in order the first time — each builds on the last.

## The two ideas in one picture

gatekit is built on two ideas. Hold onto these; the whole course returns to them.

**Idea 1 — Separate *deploy* from *activate*.** Shipping the code and turning the
behavior on are two different events. You deploy today with the new path asleep,
and you flip it on next week when you are ready.

**Idea 2 — Key the flip on a dedicated number, not the release.** The switch is
controlled by a small counter that only moves when a behavior actually changes —
the **activation version** — kept separate from your product's release version
(like `v2.3.1`).

```
Release version     v2.3.0    v2.3.1    v2.4.0    v2.4.1    ──►   (ships constantly)
Activation version  3         3         4         4         ──►   (moves only on a change)
                                        ▲
                                 the feature turns on here
```

Many releases can share one activation version. The activation version only
advances when you decide a new behavior should become reachable.

## gatekit in two layers

You take only the layer you need.

- **Gate core** — a registry of features and a gate that resolves each one
  (on or off) against the current activation version. It is in-binary and
  deterministic: the answer depends only on the code and the version, never on a
  network call. Lesson 4 lives here.
- **Migration / activation** — an optional layer that adds *deferred* activation,
  one-time data migrations tied to a version, and a `finalize` step for upgrading a
  fleet safely. Lessons 5 and 6 live here.

## Pick a reading path

**I just want to gate a feature** — the whole "ship it dormant, flip it later" story:

1. [1. Why feature gating is needed](1-why-feature-gating.md)
2. [2. Why gatekit is built this way](2-why-gatekit.md)
3. [4. Build and release a feature](4-build-and-release-a-feature.md)

**I am upgrading a fleet without downtime** — the model, then deferred activation,
`finalize`, and the design rules:

1. [1. Why feature gating is needed](1-why-feature-gating.md)
2. [2. Why gatekit is built this way](2-why-gatekit.md)
3. [5. Activation use cases](5-activation-use-cases.md)
4. [6. Rolling-upgrade thinking](6-rolling-upgrade-thinking.md)

**I am choosing a tool for my team** — the comparison and the common questions:

1. [3. How it compares to other tools](3-contrast-with-existing-patterns.md)
2. [FAQ](../faq.md)

**I want to see it all on one real tool** — a capstone that rolls a format change
across a live cluster, end to end: [Appendix E: a worked rollout](../appendix/worked-example.md).

**I want the exact rules.** The [appendix](../appendix/algorithm.md) states the
resolution algorithm precisely, explains the [conformance fixtures](../appendix/conformance.md)
that keep every implementation in agreement, and records the [design decisions](../appendix/design-notes.md).

## A note on languages

Go is the reference implementation and every Go example in this course is **real**
code against the live API. Kotlin, Rust, and Python are planned; their lesson pages
exist as clearly-marked placeholders until those implementations land, so the site
never implies a library that does not exist yet. See
[Other languages](go.md).
