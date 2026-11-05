# 1. Why feature gating is needed

> **The question:** Your product is running right now, with real users on it. You
> have a change to ship. What makes that hard, and what would make it easy?

**You will learn:** the real problems that feature gating solves, told through
scenarios you will recognize, and the three words the rest of the course is built
on — *deploy*, *activate*, and *gate*.

No code in this lesson. We are building the mental model first.

## The core difficulty

Modern software is **always on**. It runs on many machines, it talks to other
services, and users are using it *while* you change it. When you release a new
version, your users may not use the latest release at the same time. That turns one
hard question into the heart of this whole course:

> How do you change a system that is already running, used, and connected — without
> breaking it, and without having to get everything perfect in a single risky
> moment?

Three habits make this hard. Let us look at each through a story.

## Scenario 1: "It all has to go out at once"

Priya is adding a new checkout flow. It touches the cart, the payment step, and the
receipt email. She works on it for three weeks on a separate copy of the code — a
**branch** (a parallel line of development that has not yet been merged into the
main codebase).

While she works, the rest of the team keeps shipping. Every day her branch drifts
further from everyone else's. When she finally tries to merge, there are dozens of
**merge conflicts** (two people changed the same lines, and the tools cannot decide
which to keep). The merge takes two more days and introduces a bug no one catches
until it is live.

The problem was not Priya's code. The problem was that **the only moment her work
could reach production was the moment it was 100% finished.** A long-lived branch is
a bet that nothing else will change underneath you, and that bet always loses.

> **Term — trunk-based development.** A way of working where everyone merges small
> changes into one shared line (the *trunk*, or *main*) often — many times a day —
> instead of holding work on long branches. It only works if you can merge code
> that is *not finished yet* without turning it on. That "merge it off, turn it on
> later" trick is exactly what a feature gate gives you.

## Scenario 2: "Every change has to be backward-compatible"

Sam's service writes records to a shared database that three other services read.
Sam wants to change the record format. But the other services expect the old
format. If Sam ships the new format, they break. If Sam waits for all four services
to change at the same instant, he is back to the single risky moment from Scenario
1 — now across four teams.

So Sam does the thing everyone does: he makes the change **backward-compatible**.
The new code can read both the old and the new format. This is the right instinct,
but notice what it costs:

- The code now carries **two code paths** — old and new — and must keep both until
  every reader has caught up.
- Nobody writes down *when* the old path can finally be deleted, so it lives
  forever. Five years later the code is a museum of compatibility shims no one dares
  remove.

Backward compatibility is not the enemy. **Unmanaged, permanent backward
compatibility is.** Feature gating gives the old path an explicit expiry date.

## Scenario 3: "Turning it on means redeploying"

Dana finished a feature and shipped it. It is live. At 2 a.m. it starts causing
errors. To turn it off, Dana has to build a new version of the software and roll it
out to every machine — which takes forty minutes Dana does not have while the site
is down.

The feature's **behavior was welded to the deployment.** The only knob Dana had was
"ship a different build." There was no way to say "leave the code where it is, just
stop *doing* the new thing."

This is the deepest of the three problems, and it points straight at the fix.

## The one idea behind the fix

Look back at the three scenarios. The common thread is that **two separate events
got fused into one**:

- *Deploy* — putting the code onto the machines.
- *Activate* — making the new behavior actually happen.

When those are the same event, you get all three problems: work can only ship when
it is finished (Scenario 1), the live behavior can only change by redeploying
(Scenario 3), and so you compensate with permanent compatibility code (Scenario 2).

**Feature gating pulls the two events apart.**

```
         fused                              separated
   ┌───────────────────┐          ┌──────────────┐   ┌──────────────┐
   │ deploy = activate │   ───►    │    deploy    │   │   activate   │
   │  (one risky step) │          │ code arrives │   │ behavior on  │
   └───────────────────┘          │  but asleep  │   │  (flip a #)  │
                                   └──────────────┘   └──────────────┘
                                        today              later
```

You **deploy** the new code today, with the new behavior *asleep* — the code is
there, but it runs the old path. Then, when you are ready — tomorrow, next week,
after you have watched it in production — you **activate** it by flipping a switch,
with no new deployment.

## What a "gate" actually is

A **feature gate** is just a controlled `if` statement.

Instead of writing:

```
// the new behavior, live the instant this ships
doItTheNewWay()
```

you write:

```
if gate.Enabled("new-checkout") {
    doItTheNewWay()   // asleep until activated
} else {
    doItTheOldWay()   // what everyone runs today
}
```

The feature ships in the *off* branch. Nothing changes for users. Later you flip
`"new-checkout"` to on, and the new path wakes up — for a single service, or across
a whole fleet — without shipping a new build.

That one `if` solves all three scenarios:

- **Scenario 1:** Priya merges her half-finished checkout into main every day,
  behind a gate that is off. No long branch, no merge pileup.
- **Scenario 2:** Sam ships the new record format behind a gate and keeps the old
  path. The gate's version tells everyone *exactly which release* makes the old path
  safe to delete.
- **Scenario 3:** Dana flips the gate off at 2 a.m. in seconds, no redeploy. The
  code stays; only the behavior sleeps.

## Three words to carry forward

| Word | Plain meaning |
|------|---------------|
| **Deploy** | Put the code on the machines. The behavior may still be asleep. |
| **Activate** | Turn the new behavior on. No new deploy needed. |
| **Gate** | The controlled `if` that decides, right now, whether a feature is on. |

Everything else in this course is about doing this *well*: how the gate decides,
what number controls the flip, how to turn it on across many machines safely, and
when the old path is finally safe to delete.

## Check yourself

- A teammate says "we can't merge that, it's not done." What is the feature-gating
  answer? *(Merge it behind a gate that is off; finish it on main; flip it on when
  ready.)*
- Why is "just make everything backward-compatible" an incomplete fix? *(It is
  correct, but without a managed expiry the old path never gets deleted.)*
- What two events does feature gating pull apart, and why does fusing them cause
  trouble? *(Deploy and activate; fusing them means behavior can only change by
  redeploying, and work can only ship when finished.)*

## Further reading

- Martin Fowler, ["Feature Toggles (Feature Flags)"](https://martinfowler.com/articles/feature-toggles.html)
  — the canonical taxonomy of flag types.
- Paul Hammant, ["Trunk-Based Development"](https://trunkbaseddevelopment.com/)
  — why short-lived branches and gates go together.

**Next:** [2. Why gatekit is built this way](/tutorial/2-why-gatekit.md)
