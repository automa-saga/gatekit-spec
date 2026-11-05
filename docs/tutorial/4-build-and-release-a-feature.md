# 4. Build and release a feature

> **The question:** Enough theory. You have a change to ship. How do you actually
> declare a feature, ship it asleep, turn it on, and — the part everyone forgets —
> clean up the old code afterwards?

> **Prerequisites:** [Lesson 2](/tutorial/2-why-gatekit.md) for *activation version* and
> *registry* (*maturity* is introduced in this lesson). A little Go helps, but the
> shape is the same in any language.

**You will learn:** the full life of a feature in gatekit — declare it, gate the
code, ship it dormant, open the support window, flip it on, and delete the old path
— with real, runnable Go.

> **Go is real here.** Every snippet below is the live gatekit API. The complete,
> compilable versions live in
> [`examples/go/`](https://github.com/automa-saga/gatekit-spec/tree/main/examples/go)
> and run as Go example tests.

## The feature's whole life, in one picture

```
 declare ──► gate the code ──► ship DORMANT ──► flip ON ──► support N/N+1 ──► DELETE old path
  (reg.)       (if/else)        (activ. n)      (activ. n   (one window)      (window moves on)
                                                 default on)
```

We will walk this left to right with a concrete feature: switching from a single
catalog file to a per-minor **split catalog**.

## Step 1: Declare the feature

A feature is a **stable string identifier**. The single most important habit in this
whole lesson: **declare it once as a named constant**, and use that constant
everywhere.

```go
package catalog

import "github.com/automa-saga/gatekit"

// One source of truth for the identifier. Registration and every call site use
// this constant — so a typo can never silently resolve a *different* feature off.
const SplitCatalog gatekit.Feature = "SplitCatalog"
```

> **Why a constant and not a bare string?** If you type `"SplitCatalog"` by hand at
> the call site and misspell it as `"split-catlog"`, gatekit sees an *unknown*
> feature — and an unknown feature resolves **off, silently.** No error, no warning,
> just the old path forever. A named constant makes the compiler catch the typo.

Now put it in a **registry** — the table of all your features:

```go
// The registry is compiled into your binary. "default: false" is what ships the
// feature dormant; "Beta" is its maturity stage (see below).
var Registry = gatekit.Registry{
    SplitCatalog: {
        Description: "Load the per-minor split catalog instead of the single file",
        Specs: gatekit.VersionedSpecs{
            {ActiveFrom: 5, Default: false, Maturity: gatekit.Beta},
        },
    },
}
```

Read the spec out loud: *"From activation version 5 onward, `SplitCatalog` exists.
Its default is off. It is Beta."* That is the entire declaration.

### A word on maturity

`Maturity` labels how finished and how stable a feature is. It changes how it
resolves and how tools treat it:

| Maturity | Resolves by default? | Meaning |
|----------|----------------------|---------|
| `Alpha` | **No** — off unless explicitly opted in | Experimental. May change or vanish between versions; you are *not* required to keep it stable. |
| `Beta` | Yes, by its `Default` | Nearly ready. On a path to GA. |
| `GA` | Yes, by its `Default` | Generally available. A stable promise. |
| `Deprecated` | Yes, by its `Default` | On its way out. Lints flag it for removal. |

`Alpha` is special: even if its `Default` is `true`, it stays **off** unless the gate
is opted in with `WithExperimental(true)` (the equivalent of an `--experimental`
switch an operator would set). This lets you merge genuinely unfinished work without
any risk that it turns itself on.

## Step 2: Gate the code

Build the gate from the registry and a **version source** (where the current
activation version comes from), then write the `if/else`:

```go
import "github.com/automa-saga/gatekit"

// StaticVersion is the simplest version source: a fixed number. Real systems read
// it from config or the migration layer (Lesson 5).
gate, err := gatekit.New(catalog.Registry, gatekit.StaticVersion(5))
if err != nil {
    return err // the registry was invalid (empty specs, duplicate version, …)
}

// The feature gate: one controlled if, exactly as in Lesson 1.
if gate.Enabled(catalog.SplitCatalog) {
    loadSplitCatalog()   // the new path — asleep until activation reaches 5 AND default is on
} else {
    loadSingleCatalog()  // the old path — what everyone runs today
}
```

At activation version 5, with `Default: false`, `gate.Enabled` returns `false`. The
new code is **shipped but dormant.** Users run `loadSingleCatalog()` exactly as
before.

## Step 3: Ship it dormant — and prove it

"Dormant" is a claim you can test. `EnabledAt` resolves a feature against *any*
version without touching the live source — perfect for a test that locks in the
behavior at every version that matters:

```go
gate, _ := gatekit.New(catalog.Registry, gatekit.StaticVersion(0))

gate.EnabledAt(catalog.SplitCatalog, 4) // false — feature's first spec (5) not reached yet
gate.EnabledAt(catalog.SplitCatalog, 5) // false — reached, but default is off (dormant!)
```

This is the safety of the whole approach made concrete: you have *proof*, in a test,
that shipping this code changes nothing for anyone. You can merge it to main today.

## Step 4: Flip it on

When you are ready — the code has baked in production, you have watched the metrics —
you turn the feature on. There are two ways, for two different situations.

### 4a. Permanently, for the next generation: add a spec

The durable way to turn a feature on is to record a **new spec at the next
activation version** saying "from here on, the default is on." This is how the
feature graduates.

```go
var Registry = gatekit.Registry{
    SplitCatalog: {
        Description: "Load the per-minor split catalog instead of the single file",
        Specs: gatekit.VersionedSpecs{
            {ActiveFrom: 5, Default: false, Maturity: gatekit.Beta}, // born dormant
            {ActiveFrom: 6, Default: true,  Maturity: gatekit.GA},   // on from 6, now GA
        },
    },
}
```

Now resolution depends on the active version:

```go
gate, _ := gatekit.New(Registry, gatekit.StaticVersion(0))

gate.EnabledAt(catalog.SplitCatalog, 5) // false — the 5-spec: default off
gate.EnabledAt(catalog.SplitCatalog, 6) // true  — the 6-spec: default on, GA
```

> **The rule:** when a change breaks the contract with existing users, you record it
> as a **new spec at the next activation version** — you never edit the old spec in
> place. Keeping the old spec is what lets a machine still at version 5 resolve the
> *old* behavior correctly during a rolling upgrade. (More in
> [Lesson 6](/tutorial/6-rolling-upgrade-thinking.md).)

### 4b. Right now, for one machine: an override

Sometimes you need to force a feature on or off *immediately*, regardless of the
version — to test the new path on one box, or to kill a misbehaving feature at 2
a.m. (Lesson 1, Scenario 3). That is an **override**, and it is the
highest-precedence input of all:

```go
gate, _ := gatekit.New(
    catalog.Registry,
    gatekit.StaticVersion(5), // version says off…
    gatekit.WithOverrides(map[gatekit.Feature]bool{
        catalog.SplitCatalog: true, // …but the override forces it on
    }),
)

gate.Enabled(catalog.SplitCatalog) // true — the override wins
```

Overrides map directly to Kubernetes' `--feature-gates=Foo=true`. gatekit rejects an
override for a feature that is not registered, so a typo in an override is caught at
startup rather than silently doing nothing.

### 4c. From the environment: an operator override

`WithOverrides` lives in your code. To let an **operator** flip a feature on an
already-shipped binary — no code change, no recompile — read overrides from the
environment with `WithEnvOverrides`:

```go
gate, _ := gatekit.New(
    catalog.Registry,
    versionSource,
    gatekit.WithEnvOverrides("SOLO_PROVISIONER_ENABLE_"), // scan env for this prefix
)
```

gatekit maps each `<prefix>NAME` variable back to a feature with `EnvVarName`: the
prefix is yours, the suffix is the feature identifier in `UPPER_SNAKE_CASE`. This is
why feature identifiers are CamelCase — `SplitCatalog` becomes:

```
SOLO_PROVISIONER_ENABLE_SPLIT_CATALOG=true
```

Set it, restart, and the feature is forced on — beating version and maturity exactly
like `WithOverrides`. It **fails loudly**: the binary refuses to start if a
`<prefix>*` variable names no known feature or holds a non-boolean value, so a
mistyped opt-in never silently does nothing.

Use it for a per-box canary, a per-environment default (on in staging, off in prod),
or the 2 a.m. kill switch on a binary that is already deployed — the operator-facing
counterpart to `WithOverrides`.

### The precedence, once and for all

When gatekit resolves a feature, it checks, highest priority first:

1. **An explicit override** (`WithOverrides`, or `WithEnvOverrides` from the
   environment) — wins, regardless of version.
2. **The version-resolved default** of the effective spec — *except* an `Alpha` spec
   resolves off unless the gate is opted in (`WithExperimental`).
3. **Off** — unknown feature, or the first spec not yet reached.

## Step 5: Live in the support window

After the flip, your fleet is mid-upgrade: some machines at activation 6, some still
at 5. The **support window** (Lesson 2) is the promise that both work. With the
default `W = 1`, a binary whose `max` is 6 supports `min = 5` too:

```go
gate, _ := gatekit.New(Registry, gatekit.StaticVersion(6)) // default window W=1

ev := gate.EffectiveVersion()
// ev.Min = 5, ev.Max = 6, ev.Active = 6

// Call this at startup and refuse to run on error — the run invariant.
if err := gate.Check(); err != nil {
    log.Fatal(err) // active is outside [min, max]: binary too old or too new
}
```

During this window you keep **both** code paths. That is not debt — it is the
feature working as designed. The window is the agreed time for the fleet to catch up.

## Step 6: Delete the old path

This is the step that makes feature gating *sustainable* instead of a graveyard of
dead `if`s. When the window moves forward and version 5 drops below `min` — no
supported binary runs the old behavior anymore — you have an **obligation**:

- delete the old code path (`loadSingleCatalog()` and its `else`),
- delete the now-unreachable spec (the `ActiveFrom: 5` entry),
- leave only the live path.

```go
// After version 5 leaves the window, the feature has done its job. Collapse it:
var Registry = gatekit.Registry{
    SplitCatalog: {
        Description: "Load the per-minor split catalog",
        Specs: gatekit.VersionedSpecs{
            {ActiveFrom: 6, Default: true, Maturity: gatekit.GA},
        },
    },
}
```

```go
// …and the call site collapses to just the new path:
loadSplitCatalog()
```

Eventually you remove the feature entirely. The gate was **scaffolding** — it held
the two paths apart while the fleet crossed from one to the other, and then it comes
down. A feature gate that never gets deleted is a bug, and lints should remind you
(see [design notes](/appendix/design-notes.md)).

## Do I even need a gate?

Not every change needs one. Gate a change **only if it breaks the contract with
existing users of the feature.** You do **not** need a gate for:

- bug fixes,
- performance work,
- refactors that keep behavior identical,
- changes to metrics or logs that nobody depends on.

Adding a gate to those just adds dead code. The activation version is a scarce,
meaningful signal — spend it only on real behavior changes.

## The whole life, as code

| Step | What you write | Result |
|------|----------------|--------|
| Declare | a named `Feature` constant + a registry entry (`Default: false`) | the feature exists, dormant |
| Gate | `if gate.Enabled(f) { new } else { old }` | both paths present |
| Ship | merge to main, deploy | nothing changes for users |
| Flip | add a spec at the next version (`Default: true`) **or** an override | behavior turns on |
| Support | keep both paths while the version is in the window | rolling upgrade works |
| Delete | remove the old path + the stale spec once it leaves the window | scaffolding comes down |

## Check yourself

- Why declare the feature identifier as a constant instead of a string literal?
  *(An unknown feature resolves off silently; a constant makes the compiler catch
  typos.)*
- You shipped at activation 5 with `Default: false`. A user reports the new behavior
  is live. What are the two likeliest causes? *(An override forcing it on, or the
  active version is actually ≥ a later spec whose default is on.)*
- When is it *safe* to delete the old code path? *(Once its activation version drops
  below `min` — no supported binary runs it anymore.)*
- Which release should bump the activation version: a refactor, a perf fix, or a
  contract-breaking behavior change? *(Only the contract-breaking change.)*

## Further reading

- [gatekit core spec §3–6](https://github.com/automa-saga/gatekit-spec/blob/main/core-spec.md)
  — resolution, maturity, overrides, and the author obligation, stated normatively.
- [Appendix A: the resolution algorithm](/appendix/algorithm.md) — the exact rule
  behind every example above.
- [`examples/go/`](https://github.com/automa-saga/gatekit-spec/tree/main/examples/go)
  — the runnable versions of these snippets.

**Next:** [5. Activation use cases](/tutorial/5-activation-use-cases.md)
