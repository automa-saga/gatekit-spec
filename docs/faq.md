# FAQ

The recurring questions, answered briefly. Each links to the lesson with the full
story.

## Do I need two version axes?

**Usually no.** If you deploy and control the software yourself and there is no
cross-machine protocol to coordinate, reuse your release version as the activation
version and activate on deploy — the [single-axis](/tutorial/5-activation-use-cases.md)
case. Reach for a second, dedicated axis only when a change needs fleet-wide agreement
(a wire or storage format) or you want a rollback-safe staging pause between deploy and
activate. See [Lesson 5](/tutorial/5-activation-use-cases.md).

## What if I just use my release version?

That is exactly the single-axis case, and it is fine for many projects. The one thing
to keep straight: your release version moves on *every* release, but a feature should
only turn on at the release that crosses its spec. As long as you map "which release
corresponds to which activation version" correctly, reusing the release number works.
You lose the ability to tell compatibility from the number alone (see the next
question), which is why coordinated fleets prefer a dedicated counter.

## How is this different from LaunchDarkly / Unleash?

They are **runtime flag services**: a flag lives on a server, is decided per user at
runtime, and is flipped from a dashboard. gatekit is an **in-binary, deterministic
version gate**: the answer is compiled in and depends only on the registry and the
version, with no network call. Use a flag service for per-user targeting, A/B tests,
and non-engineer flips; use gatekit when a *fleet must agree* on a behavior generation
and move forward together. They solve different problems and can be used together. Full
comparison in [Lesson 3](/tutorial/3-contrast-with-existing-patterns.md).

## How is this different from Kubernetes feature gates?

gatekit's gate core is a close cousin of `component-base/featuregate` (same maturity
stages, same in-binary idea). The differences: gatekit keys on a *release-independent*
activation version (Kubernetes ties emulation version to the release minor), has a
*configurable* support window (Kubernetes fixes ~3), is dependency-light, and bundles an
optional migration layer. **If you are building a Kubernetes component that already uses
component-base, use component-base.** gatekit is for everything else. See
[Lesson 3](/tutorial/3-contrast-with-existing-patterns.md).

## If I activate a gate later, when does its data migration run?

**At the moment of activation — not at deploy.** In gatekit a migration is tied to the
activation version, so it runs when that version activates: immediately for a
`local` (run-now) migration, or at `finalize` for a `run-at-finalize`/`shared` one. If you
deploy the new code today but defer activation to next week, the migration runs *next
week, at finalize*, together with the behavior it supports — never silently at deploy
and never forgotten. This coupling is the part plain flag services and most gate
libraries miss: they flip a boolean and leave the data change to you as a separate,
manual step. See [Lesson 5](/tutorial/5-activation-use-cases.md).

## What happens with an irreversible migration?

Rollback is only *guaranteed* before [finalize](/tutorial/5-activation-use-cases.md) runs
any migration — the preserve-downgrade window. Once finalize runs an irreversible
migration, retreat to the old binary is no longer guaranteed, because the new code may
have written data the old binary cannot read. gatekit makes this explicit: it **refuses
to lower `active` across an irreversible step** with a typed error, so you can never
silently assume a retreat that is not actually possible. Design to delay finalize and to
honor the [write floor](/appendix/glossary.md#write-floor) so you keep the door open as
long as you can. See [Lesson 5](/tutorial/5-activation-use-cases.md).

## When does a feature's default flip?

At the activation version of its *next* spec. You turn a feature on permanently by
recording a **new `FeatureSpec` at the next activation version** with `Default: true` —
never by editing the old spec in place. The old spec keeps resolving the old behavior so
machines still on the old version stay correct during a rolling upgrade. For an
immediate, version-independent flip on one machine, use an
[override](/appendix/glossary.md#override). See
[Lesson 4, Step 4](/tutorial/4-build-and-release-a-feature.md).

## When do I *not* need a gate at all?

Gate a change only if it **breaks the contract with existing users** of the feature. Bug
fixes, performance work, behavior-preserving refactors, and changes to metrics or logs
nobody depends on do **not** need a gate — adding one just creates dead code. The
activation version is a meaningful, scarce signal; spend it only on real behavior
changes. See [Lesson 4](/tutorial/4-build-and-release-a-feature.md).

## Why is my Alpha feature off even though its default is `true`?

By design. An `Alpha` feature resolves **off unless explicitly opted in**
(`WithExperimental(true)` or a per-feature override), *even when its `Default` is
`true`*. This lets you merge genuinely unfinished work with zero risk it turns itself
on. If you want it to follow its default, make it `Beta` or `GA`. See
[Lesson 4](/tutorial/4-build-and-release-a-feature.md) and
[Appendix A](/appendix/algorithm.md).

## Why did my feature silently stay off?

The most common cause is a **misspelled or unregistered identifier**: an unknown feature
resolves off, with no error. Declare each feature as a typed `Feature` constant and use
that constant at both registration and call sites so the compiler catches typos. Other
causes: the active version has not reached the feature's first spec, or its spec's
default is off. See [F.1 in Spot the failure](/appendix/spot-the-failure.md#f1).

## How wide should my support window be?

At least as wide as the version gap your fleet spans during an upgrade. If a rollout can
leave a machine one generation behind before it catches up, the default `W = 1` is
enough. If machines can lag two or more generations (operators upgrade on their own slow
schedule), widen it. Wider windows mean keeping more old code paths alive longer, so
choose the smallest `W` that covers your slowest upgrader. See
[Lesson 6](/tutorial/6-rolling-upgrade-thinking.md).

## Can I roll back after an upgrade?

Before finalize: yes, cleanly — you are in the preserve-downgrade window. After
finalize: only if every migration in the step was reversible (a working `rollback` that
honored the write floor); gatekit refuses to lower `active` across an irreversible step.
Crossing *below* `min` is never supported, and the run invariant forbids running there
anyway. See [Lesson 5](/tutorial/5-activation-use-cases.md) and
[Lesson 6](/tutorial/6-rolling-upgrade-thinking.md).

## Do I have to use the migration layer?

No. The gate core ("ship dormant, flip later") stands on its own and is dependency-light.
Add the migration/activation layer only if you also move stored/wire data or coordinate a
fleet with `finalize`. A consumer whose active version is set externally can use the core
with no activation model at all. See [Lesson 2](/tutorial/2-why-gatekit.md).

## Which languages are supported?

Go is the reference implementation and is real today. Kotlin, Rust, and Python are
planned; their lesson pages are clearly-marked placeholders until those implementations
land and pass the shared conformance fixtures. Because every implementation must pass the
same fixtures, the concepts are identical across languages — only syntax differs. See
[Other languages](/tutorial/go.md).

## Where is the exact, normative behavior?

In the specs:
[core](https://github.com/automa-saga/gatekit-spec/blob/main/core-spec.md),
[activation](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md),
and [migration](https://github.com/automa-saga/gatekit-spec/blob/main/migration-spec.md),
with the [conformance fixtures](/appendix/conformance.md) as the cross-language source of
truth. [Appendix A](/appendix/algorithm.md) restates the resolution algorithm in one
place.
