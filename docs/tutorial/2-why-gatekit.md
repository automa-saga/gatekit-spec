# 2. Why gatekit is built this way

> **The question:** Lesson 1 said "flip a switch." But *which* switch, controlled by
> *what*, stored *where*? The answers are what make gatekit different from a flag
> service — and what make it safe for upgrading a whole fleet.

> **Prerequisites:** [Lesson 1](1-why-feature-gating.md). You should be comfortable
> with *deploy*, *activate*, and *gate*.

**You will learn:** the four design choices behind gatekit — in-binary resolution,
a release-independent version, the configurable support window, and the optional
migration layer — and *why* each one is the way it is.

## Start from what we need

We want to flip a feature on across a running fleet without downtime. Write that
goal down plainly and the design almost falls out of it:

1. Every machine must agree on whether a feature is on — **no split-brain**, where
   two servers disagree and users see different behavior at random.
2. The flip must not require a redeploy (Lesson 1, Scenario 3).
3. During an upgrade, old and new machines run **side by side** for a while, so both
   must keep working.
4. If the flip goes wrong, we want to **go back**.

Let us turn each need into a design choice.

## Choice 1: Resolve in the binary, deterministically

The first instinct is a **flag service**: a server somewhere that every machine asks,
"is `new-checkout` on?" This is how tools like LaunchDarkly work, and it is great
for some jobs (we compare them in [Lesson 3](3-contrast-with-existing-patterns.md)).
But for our goal it has a problem.

If the answer comes from a network call, then the answer can **differ between two
machines at the same instant** — one got "on," the other got "off," because the
service changed its mind between the two calls, or one call failed. That is exactly
the split-brain from need #1. And if the flag service is down, what does your code
do? Guess?

gatekit makes the opposite choice: **resolution is in-binary and deterministic.**

> **Term — deterministic.** Given the same inputs, you always get the same output,
> every time, everywhere. No randomness, no clock, no network.

In gatekit, whether a feature is on is a pure function of two things only:

- the **registry** — the feature definitions compiled into the binary, and
- the **version** — the single number currently in force.

```
Enabled(feature) = f(registry, version)      // nothing else
```

No wall clock. No environment. No remote call. Two machines running the same binary
at the same version and configuration **cannot disagree.** And there is nothing to be
"down." This is the property that makes a fleet-wide flip safe to reason about.

> **Term — registry.** The in-code table that lists every feature and, for each, the
> rule for when it is on. It is compiled into the program, so it ships *with* the
> code it gates.

## Choice 2: Key the flip on a release-independent number

So resolution depends on "the version." Which version? The obvious answer — your
release version, like `v2.4.1` — is the wrong one, and seeing why is the single most
important idea in gatekit.

Your release version moves **all the time**. You ship bug fixes, performance
tweaks, and typo corrections constantly, and each bumps the release version. But
almost none of those *change a gated behavior*. If features were keyed on the
release version, you could never tell, from the number alone, whether a given
release is safe to run next to another one.

So gatekit introduces a second, separate counter:

> **Term — activation version.** A non-negative whole number (0, 1, 2, …) that goes
> **up by at most one per release, and only when a gated, behavior-changing feature
> becomes reachable.** It is independent of the release version. Many releases can
> share the same activation version.

```
Release version    v2.3.0   v2.3.1   v2.4.0   v2.4.1   v2.5.0
                   ───┬────────┬────────┬────────┬────────┬───►
                      │        │        │        │        │
Activation version    5        5        6        6        6
                               bugfix   feature  perf     docs
                                        flips on
```

Think of the activation version as answering a different question than the release
version:

- Release version: *"How new is this build?"*
- Activation version: *"Which generation of behavior does this build run?"*

Because it is a plain [monotonic](/appendix/glossary.md#monotonic) counter (it only
ever goes up, never down), you can compare two machines at a glance: a
machine at activation 6 and one at activation 5 differ by exactly one behavior
generation, so they are one step apart — inside a support window of one, they are
compatible. That simple arithmetic is the backbone of the rolling upgrade in
[Lesson 6](6-rolling-upgrade-thinking.md).

### The "protocol version" pattern

You do not *have* to invent a new counter. A common and recommended pattern,
especially when a change affects how machines talk to each other, is to keep a
dedicated counter whose whole job is "which generation of the agreement are we on."
Teams often call this a **protocol version**.

> **Term — protocol version.** The name of the *pattern* where the decoupled
> activation counter is a dedicated, deliberately-advanced number — separate from
> the release version — that participants use to agree on behavior. Kafka's
> "inter-broker protocol version" and CockroachDB's "cluster version" are real
> examples. In gatekit's vocabulary, "protocol version" is a *use* of the activation
> version, not a different thing.

[Lesson 5](5-activation-use-cases.md) shows when to just reuse your release version
as the activation version (the single-axis case) and when to run a dedicated
protocol-version counter (the two-axis case).

## Choice 3: A configurable support window

During a rolling upgrade, machines at different versions run side by side (need #3).
How *far apart* are you willing to let them drift?

gatekit answers with three derived numbers it calls **the triple**:

> **Term — the triple `{min, max, active}`.**
> - **max** — the newest activation version this binary knows about. It is computed
>   from the registry, never hand-edited.
> - **active** — the activation version currently in force; the one resolution uses.
> - **min** — the oldest activation version this binary still supports.

The gap between `min` and `max` is the **support window**, written `W`:

> **Term — support window `W`.** How many activation versions *behind* `max` a binary
> still supports. `min = max(firstVersion, max − W)`. The default is `W = 1`
> ("I support the current generation and the one before it" — often written N/N+1).
> You can widen it (N/N+3, N/N+5) if your fleet upgrades slowly.

Why is this a knob and not a fixed rule? Because fleets differ. A tool you control
and upgrade in an afternoon is fine with `W = 1`. A platform whose operators upgrade
on their own slow schedule may need a wider window so a lagging machine is still
supported. Kubernetes fixes this at roughly three; gatekit lets you choose.

And here is the guard rail that keeps it honest — **the run invariant**:

> **Term — run invariant.** At startup, a binary must check `min ≤ active ≤ max`. If
> `active` is **below** `min`, this binary is too new to safely run that old state.
> If `active` is **above** `max`, this binary is too old to understand it. Either
> way it must refuse to start and say so clearly.

The run invariant is what stops a stale, left-behind machine from quietly
undermining a flip. It forces the fleet to catch up *within* the window instead of
drifting arbitrarily far apart. You will see it do real work in Lesson 6.

## Choice 4: Make migrations an optional second layer

Sometimes activating a feature is not just "run new code" — it also means
**changing stored data**. The new checkout writes receipts in a new format; the old
ones must be converted. That is a **migration**: a one-time data transformation.

> **Term — migration.** A one-time change to stored or on-the-wire data that goes
> with a behavior change — for example, rewriting every saved record into a new
> shape.

Migrations are where the need to **go back** (need #4) gets sharp. Running new code
is easy to undo — just run the old code. But a migration can be **irreversible**:
once you have thrown away the old-format data, there may be no way back. So the flip
that runs a migration has to be handled with extra care — a deliberate, one-way
step called **finalize**, covered in [Lesson 5](5-activation-use-cases.md).

gatekit keeps this as a **separate, opt-in layer** on purpose:

- If all you need is "ship it dormant, flip it on," you use only the **gate core**.
  It has no heavy dependencies and drops into any project.
- If you also move data and coordinate a fleet, you add the **migration /
  activation** layer on top.

```
┌─────────────────────────────────────────────┐
│  migration / activation layer   (opt-in)     │  ← deferred activation,
│  deferred activation · migrations · finalize │    one-time migrations,
├─────────────────────────────────────────────┤    finalize, reversibility
│  gate core                      (always)     │  ← registry + deterministic
│  registry · deterministic resolution         │    resolve against a version
└─────────────────────────────────────────────┘
```

You never pay for the layer you do not use.

## Why not just use a flag service?

It is worth saying plainly, because it is the question everyone asks. A remote flag
service (LaunchDarkly, Unleash) is excellent at things gatekit deliberately does not
do: targeting a flag to 5% of users, to a specific account, or to a country;
changing a flag from a web dashboard without involving engineers; A/B experiments.

But those powers come from the flag living **outside** your binary, decided at
runtime over the network — which is exactly the split-brain and the "what if it's
down" problem from Choice 1. gatekit is for a different job: **a fleet of machines
must agree, deterministically, on which generation of behavior they run, and move to
the next one together, safely, with the option to go back.** For that job, in-binary
and deterministic wins. [Lesson 3](3-contrast-with-existing-patterns.md) lays the
two side by side.

## Check yourself

- Why can't two machines running the same gatekit binary at the same `active`
  disagree about a feature? *(Resolution is a pure function of registry + version;
  no network, clock, or environment.)*
- Give one release that would **not** bump the activation version. *(Any bug fix,
  perf change, refactor, or docs change — anything that does not change a gated
  behavior.)*
- What does the run invariant protect against? *(A stale or too-new binary running
  against a state it cannot safely handle, which would undermine a flip.)*
- When do you need the migration layer, and when can you skip it? *(Need it when
  activation also moves stored/wire data or coordinates a fleet; skip it for plain
  "ship dormant, flip later.")*

## Further reading

- [gatekit core spec](https://github.com/automa-saga/gatekit-spec/blob/main/core-spec.md)
  — the exact resolution rules behind Choice 1.
- [gatekit activation spec](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md)
  — the triple, the window, and the run invariant behind Choices 2 and 3.
- Kubernetes [KEP-4330: Compatibility Versions](https://github.com/kubernetes/enhancements/tree/master/keps/sig-api-machinery/4330-compatibility-versions)
  — the same idea in a large real system.

**Next:** [3. How it compares to other tools](3-contrast-with-existing-patterns.md)
