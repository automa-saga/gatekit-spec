# 3. How it compares to other tools

> **The question:** Feature gating is not new. Big systems have solved versions of
> this for years. What did gatekit borrow, where does it differ, and — the question
> you actually care about — *which tool should you reach for?*

> **Prerequisites:** [Lesson 2](2-why-gatekit.md). You need *in-binary resolution*,
> *activation version*, *support window*, and *the triple*.

**You will learn:** how gatekit sits next to five well-known patterns, and a simple
rule for choosing between them.

## Two families of tool

Almost every tool in this space falls into one of two families. Getting the family
right is 90% of the decision.

**Family A — runtime flag services.** A flag lives *outside* your binary, on a
server, and is decided at runtime, often per user. You change it from a dashboard.
*Examples: LaunchDarkly, Unleash, Flagsmith.*

**Family B — in-binary version gates.** A flag is compiled *into* your binary and
decided deterministically against a version number shared across the fleet. You
change behavior by advancing the version. *Examples: Kubernetes feature gates,
CockroachDB cluster version, Kafka protocol version — and gatekit.*

gatekit is firmly Family B. The quickest way to understand it is to see what each
neighbor does.

## LaunchDarkly / Unleash (Family A)

These are **runtime flag services**. Your code asks a client library "is this flag
on for *this user*?" and the answer comes from a central service that you drive from
a web UI.

**What they are brilliant at:**

- **Targeting.** On for 5% of users, or just account #42, or just Canada.
- **No-deploy, no-engineer changes.** A product manager flips a flag from a
  dashboard.
- **Experiments.** A/B tests with built-in measurement.

**Where they differ from gatekit:**

- The answer is **per-user and runtime**, not a fleet-wide generation. Two machines
  can legitimately get different answers at the same instant — wonderful for a
  gradual user rollout, wrong for "the whole fleet must agree which protocol it
  speaks."
- They add a **network dependency and a runtime story** ("what if the service is
  down?"). gatekit has neither.
- They have **no built-in model for data migrations or an irreversible one-way
  step.** That is not their job.

**What gatekit borrows:** the core idea that *deploy and activate are different
events* — a flag you can flip without redeploying.

## Kubernetes feature gates + emulation version (Family B)

Kubernetes has long had `component-base/featuregate`: in-code features with Alpha /
Beta / GA stages, flipped with `--feature-gates=Foo=true`. gatekit's gate core is a
close cousin — the maturity stages in [Lesson 4](4-build-and-release-a-feature.md)
come straight from this lineage.

More recently, [KEP-4330](https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/4330-compatibility-versions)
added the **emulation version**: a running binary can be told to *behave like* an
older version, so a newer binary and an older one can coexist during an upgrade.
This is the same insight as gatekit's `active` — "which generation of behavior am I
running?" decoupled from "how new is this build?"

The terminology maps almost one to one:

| Kubernetes | gatekit |
|------------|---------|
| `--emulation-version` | `active` |
| binary version | `max` |
| `--min-compatibility-version` | `min` |
| fixed ~3-version skew | configurable window `W` (default 1) |

**Where gatekit differs:**

- **Release-independent counter.** Kubernetes' emulation version is tied to the
  Kubernetes release minor. gatekit's activation version moves *only* on a behavior
  change, so many releases share one number.
- **Configurable window.** Kubernetes fixes the skew at roughly three minors;
  gatekit lets you choose `W`.
- **Dependency weight.** `component-base` pulls in a large slice of the Kubernetes
  module tree and its release cadence. gatekit's gate core is dependency-light and
  usable in any project.
- **Migrations included.** gatekit bundles an optional migration/activation layer;
  Kubernetes does storage migration and conversion with separate machinery.

**Rule of thumb:** *building a Kubernetes component that already imports
component-base? Use component-base.* It is battle-tested and integrates with
Kubernetes' own version-skew machinery. gatekit is for everything else.

## CockroachDB cluster version + `preserve_downgrade_option` (Family B)

CockroachDB runs as a cluster of nodes that upgrade one at a time. It keeps a
**cluster version** — a shared number, separate from the binary version, that marks
which behaviors and on-disk formats are active. New behaviors stay dormant until the
cluster version is *finalized* to the new value, and finalization only happens once
every node is new enough.

Crucially, an operator can set `preserve_downgrade_option` to **delay
finalization**, keeping the door open to roll back to the old binary. Once they
finalize, that door closes — because the new version may have written data the old
binary cannot read.

This is almost exactly gatekit's model:

| CockroachDB | gatekit |
|-------------|---------|
| cluster version | `active` (activation version) |
| finalize the cluster version | `finalize` ([Lesson 5](5-activation-use-cases.md)) |
| `preserve_downgrade_option` | the **preserve-downgrade window** — the span before finalize when rollback is still safe |
| "nodes must be new enough" | the run invariant + support window |

**What gatekit borrows:** the whole shape — a dormant-until-finalized version, a
deliberate finalize, and a window before it where going back is still safe.

**Where gatekit differs:** gatekit is a *general-purpose library*, not a database.
It gives you the model and the guard rails; you supply the migrations and the
version source.

## Kafka inter-broker protocol version (Family B)

A Kafka cluster is a set of brokers that must agree on the wire protocol they use to
talk to each other. During an upgrade you install the new broker binaries first but
**pin** the `inter.broker.protocol.version` to the old value. The new binaries run,
but they keep speaking the old protocol, so they interoperate with any broker not
yet upgraded. Only once *every* broker is on the new binary do you raise the pinned
version — and from then on the new protocol is in force.

This is the clearest real-world example of the **protocol version pattern** from
Lesson 2: a dedicated counter, separate from the release, deliberately advanced
*after* the fleet is ready.

**What gatekit borrows:** the two-step "deploy new binaries dormant, then raise the
protocol number when everyone is ready" dance — generalized so any product can do it,
not just a message broker.

## One table, five tools

| Tool | Family | Decided | Keyed on | Migrations | Rollback model |
|------|--------|---------|----------|------------|----------------|
| LaunchDarkly / Unleash | A: runtime | per user, at runtime | a dashboard flag | none | flip back anytime |
| K8s feature gates + emulation | B: in-binary | per binary | emulation version (tied to release) | separate machinery | version skew window |
| CockroachDB | B: in-binary | per cluster | cluster version | built in | preserve-downgrade, then one-way |
| Kafka protocol version | B: in-binary | per cluster | protocol version | n/a (wire format) | pin, then raise |
| **gatekit** | **B: in-binary** | **per fleet** | **activation version (release-independent)** | **optional layer** | **window + finalize one-way line** |

## How to choose

A short decision path:

1. **Do you need to target individual users, or let non-engineers flip flags from a
   UI, or run A/B experiments?** → You want a **runtime flag service** (Family A).
   gatekit will not do this, and that is by design.
2. **Are you building a Kubernetes component that already uses `component-base`?** →
   Use **component-base feature gates**. Don't add a second system.
3. **Do you need a fleet (or cluster) to agree, deterministically, on which
   generation of behavior/protocol it runs, and move forward together — possibly
   with data migrations and the option to roll back?** → You want a **Family B
   version gate.** If you are not already inside Kubernetes or a database that has
   one, that is exactly what **gatekit** is for.

You can also use two of these at once: a runtime flag service for per-user product
experiments, *and* gatekit for fleet-wide protocol/behavior generations. They
answer different questions.

## Check yourself

- Your PM wants to turn a feature on for 10% of users and watch conversion. Which
  family? *(A — a runtime flag service. gatekit does not target users.)*
- Your storage format is changing and every node must agree before the new format is
  written. Which family, and which gatekit concept matches CockroachDB's
  `preserve_downgrade_option`? *(B; the preserve-downgrade window before finalize.)*
- Why is gatekit's activation version "release-independent" an advantage over
  Kubernetes tying emulation version to the release minor? *(The number only moves on
  a real behavior change, so many releases share it and the number alone tells you
  the behavior generation.)*

## Further reading

- [LaunchDarkly: what are feature flags](https://launchdarkly.com/blog/what-are-feature-flags/).
- [CockroachDB: upgrade finalization](https://www.cockroachlabs.com/docs/stable/upgrade-cockroach-version).
- [Kafka: upgrading and `inter.broker.protocol.version`](https://kafka.apache.org/documentation/#upgrade).
- Kubernetes [KEP-4330](https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/4330-compatibility-versions).

**Next:** [4. Build and release a feature](4-build-and-release-a-feature.md)
