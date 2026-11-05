# 6. Rolling-upgrade thinking

> **The question:** Zoom out. You are not just flipping one feature — you are
> designing a *product* so that a whole fleet of machines can move to a new version,
> one at a time, while users keep using it. What principles make that possible?

> **Prerequisites:** [Lesson 2](/tutorial/2-why-gatekit.md) (the triple, the run invariant) and
> [Lesson 5](/tutorial/5-activation-use-cases.md) (migrations, finalize, the one-way line).

**You will learn:** the five design rules behind a zero-downtime upgrade, and how
gatekit's pieces enforce each one. This lesson has less new API and more thinking —
it is the one to re-read when you design your own system.

## What "rolling upgrade" actually means

> **Term — rolling upgrade.** Upgrading a fleet by replacing machines (or restarting
> them on new code) **a few at a time**, so the service stays up throughout. There is
> no moment when everything is down and no moment when everything switches at once.

The defining fact of a rolling upgrade is **coexistence**: for a stretch of time, old
code and new code run side by side, serving the same users, reading the same data,
and talking to each other.

```
time ──►
node 1   [ v5 ]───────[ v6 ]────────────────────
node 2   [ v5 ]──────────────[ v6 ]─────────────
node 3   [ v5 ]─────────────────────[ v6 ]──────
                    ▲           ▲         ▲
              both versions live at once; the service never stops
```

Every rule below exists to make that coexistence safe.

## Rule 1: New code must run happily at the *old* behavior

This is the foundation. When a new binary (v6) starts up next to old ones (v5), it
must not immediately start *doing* v6 things — or it breaks its v5 neighbors. So:

> **Deploy new code dormant.** The new binary runs, but resolves features against the
> *current* active version (still 5), so it behaves exactly like its neighbors until
> the fleet agrees to move.

This is Lessons 1 and 4's "ship it dormant" seen from the fleet's point of view. It
is why activation is decoupled from deploy: deploying a machine must be a
**behavior-neutral** act.

gatekit enforces this by resolving against `active`, not `max`. A v6 binary *knows*
about version 6 (`max = 6`) but *acts* at whatever `active` says. Install it while
`active = 5` and it is a well-behaved v5 citizen.

## Rule 2: Keep the old behavior reachable — the support window

If new code must be able to behave like the old version, then the old behavior's code
has to still *exist*. That is the support window from Lesson 2, now with a clear job:

> **The support window `W` is your coexistence budget.** It is how many behavior
> generations back a binary still fully supports. `W = 1` means "me and the one
> before me." It must be **at least as wide as the version gap your fleet will span
> mid-upgrade.**

If your rollout can leave a machine two generations behind before it catches up, `W =
1` is too narrow — you need `W = 2` or more. Choose `W` from how slowly your slowest
machine upgrades.

This is also why Lesson 4's "delete the old path" has a *precondition*: you may only
delete it once its version drops **below `min`**, i.e. out of the window. Delete it
earlier and a lagging machine loses the behavior it is still running.

## Rule 3: Refuse to run outside the window — the run invariant

Coexistence has limits. A machine that has fallen *too far* behind — or one that is
somehow *too far ahead* of the active state — is dangerous: it can misread data or
undermine a flip. The guard is the **run invariant** (Lesson 2):

> At startup, check `min ≤ active ≤ max`. If it fails, **refuse to start** and report
> a typed error.

The whole entrypoint, start to finish — the check belongs in `main`, before the
process does any real work:

```go
func main() {
    // 1. Load the authoritative active version from its store (Lesson 5).
    store, err := activation.OpenFileStore("/opt/myapp/activation.json", int(catalog.MaxActivation))
    if err != nil {
        log.Fatalf("activation state: %v", err)
    }

    // 2. Build the gate from the registry and a live, store-backed version source.
    gate, err := gatekit.New(catalog.Registry, storeSource{store})
    if err != nil {
        log.Fatalf("invalid gate configuration: %v", err)
    }

    // 3. The run invariant: min <= active <= max. Check it once, here, before serving.
    if err := gate.Check(); err != nil {
        var ve *gatekit.VersionRangeError
        if errors.As(err, &ve) {
            // The typed error carries the offending triple and which way to fix it.
            log.Fatalf("refusing to start: activation %d is outside the supported window [%d, %d] — %s",
                ve.Active, ve.Min, ve.Max, remedy(ve))
        }
        log.Fatalf("refusing to start: %v", err)
    }

    // 4. Inside the window — safe to serve.
    serve(gate)
}

// remedy turns the two failure directions into an actionable message.
func remedy(ve *gatekit.VersionRangeError) string {
    if ve.Active > ve.Max {
        return "this binary is too old for the active state; upgrade the binary"
    }
    return "the active state fell behind this binary's support window; catch it up, or widen W with WithSupportWindow"
}
```

`OpenFileStore` and `storeSource` are the persistent store and live version source
from [Lesson 5](/tutorial/5-activation-use-cases.md) (and the manual's
[Implementing activation](/manual/implementing-activation.md)); `serve` is your app.
Everything else is the live gatekit API.

Why *refuse* rather than muddle on? Because a machine that runs outside the window is
exactly the "stale node silently corrupting a flip" failure. Failing loud and early
turns a subtle data-corruption bug into an obvious, fixable startup error. The run
invariant is the reason a rolling upgrade has a *floor* it cannot sink below.

## Rule 4: Activate in stages, with readiness

Now the move itself. You never flip the whole fleet in one instant — that would be
the all-at-once release you were trying to avoid. You flip in **stages**, gated on
**readiness**:

> **Term — readiness.** The condition that the fleet is safe to advance: *every*
> participant is new enough to handle the next activation version (its `max ≥
> target`). You do not finalize until readiness holds.

The staged sequence for a two-axis (shared) change:

1. **Roll the new binaries out, dormant.** Deploy v6 everywhere. `active` stays 5.
   Each machine passes the run invariant (`5` is in `[5,6]`). Service unaffected.
2. **Wait for readiness.** Confirm every machine now reports `max ≥ 6`. The slow ones
   finish catching up. (The coordinated, negotiated form of this check — every
   participant reporting in — is the piece gatekit reserves for a future spec; today
   you confirm it with your own fleet tooling.)
3. **Finalize, once.** Run the run-at-finalize `shared` migrations and advance `active` to 6 for
   the fleet. Now, and only now, the new behavior is live everywhere — together.
4. **Later, clean up.** When 5 leaves the window, delete the old paths (Rule 2).

```
 stage 1: deploy dormant     stage 2: readiness     stage 3: finalize
  active=5, binaries=6         every node max≥6        active=6 (once)
  ──────────────────────►     ───────────────────►    ──────────────────►
       service up                 service up               service up
```

Notice that the *risky* step (finalize) happens **after** the fleet is fully capable,
not before. You have inverted the dangerous ordering of the all-at-once release.

## Rule 5: Keep a way back — reversibility and the preserve-downgrade window

Even a careful rollout can reveal a problem. Design so you can retreat:

> Between "deploy dormant" and "finalize," you are in the **preserve-downgrade
> window** (Lesson 5). Rolling back to the old binary is clean here, because nothing
> irreversible has happened yet.

Two design habits keep this door open:

- **Delay finalize** as long as you reasonably can. The longer the preserve-downgrade
  window, the more time to catch a problem while retreat is still free.
- **Honor the write floor.** Never write data an older supported version cannot read
  (Lesson 5). This is what makes *lowering* the active version safe within the window,
  and it is a property you design into every migration, not something gatekit can add
  afterwards.

Once you cross the one-way line (finalize runs an irreversible migration), retreat is
no longer guaranteed — so you cross it *deliberately*, after readiness, with eyes
open. gatekit refuses to lower `active` across an irreversible step precisely so you
cannot cross back by accident.

## The run invariant as a one-line contract

Much of this course distills into a single sentence a well-designed participant must
be able to promise at all times:

> **The run invariant:** *"My active behavior generation is one my binary both knows
> (`≤ max`) and still supports (`≥ min`)."*

If every machine can promise that, every machine can coexist with every other machine
in the window, and the fleet can move forward one step at a time without a gap in
service. That one promise is what a rolling upgrade *is*.

## A design checklist for your own product

When you design a system for zero-downtime upgrades, ask:

- [ ] **Is deploying a machine behavior-neutral?** (New code runs at the old `active` until told otherwise — Rule 1.)
- [ ] **Is the old behavior still reachable for the whole window?** (Don't delete a path while a supported version still needs it — Rule 2.)
- [ ] **Does each machine refuse to run outside its window, loudly?** (The run invariant at startup — Rule 3.)
- [ ] **Does the risky flip happen only after the fleet is ready?** (Staged activation gated on readiness — Rule 4.)
- [ ] **Is there a preserve-downgrade window, and do writes honor the floor?** (A way back until you deliberately choose otherwise — Rule 5.)

If you can tick all five, you can upgrade your fleet while it is in use. gatekit gives
you the machinery; these rules are how you *use* it well.

> **See it all on one tool.** [Appendix E: a worked rollout](/appendix/worked-example.md)
> takes a small clustered key-value store, writes down its six rollout requirements, and
> walks the entire gatekit flow — ship dormant, defer, run the migration at a crash-safe
> finalize, keep a way back — against those requirements, start to finish.

## Check yourself

- Why must deploying a new binary be "behavior-neutral"? *(So it can coexist with
  older neighbors; it resolves against `active`, not `max`.)*
- Your rollout can leave a node two generations behind. Is `W = 1` safe? *(No — the
  window must be at least as wide as the gap the fleet spans; use `W ≥ 2`.)*
- Why does finalize come *after* readiness, not before? *(So the risky, possibly
  irreversible flip happens only once every machine can handle the new version —
  inverting the all-at-once ordering.)*
- What single promise, held by every machine, makes a rolling upgrade possible? *(The
  run invariant: `min ≤ active ≤ max`.)*

## Further reading

- [gatekit activation spec §3, §6](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md)
  — the run invariant and finalize/reversibility, normatively.
- Kubernetes [version skew policy](https://kubernetes.io/releases/version-skew-policy/)
  — a large fleet's coexistence rules in practice.
- [Appendix C: design notes](/appendix/design-notes.md) — why these rules are
  shaped the way they are.

**Next:** [Other languages](/tutorial/go.md) · or jump to the [FAQ](/faq.md).
