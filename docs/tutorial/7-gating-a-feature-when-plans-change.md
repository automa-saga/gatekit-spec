# 7. Gating a feature while plans change

> **The question:** Four engineers are building four features on one `main` branch.
> They merge early and often, each feature dormant behind a gate, instead of nursing
> four long-lived branches. Some features will not be ready on time. How does each
> engineer **create** a gate, **keep it dormant**, and **turn it on** when ready — and
> what happens to a feature that slips because someone else shipped urgent work first?

> **Prerequisites:** [Lesson 4](/tutorial/4-build-and-release-a-feature.md) for the
> basic life of a feature, and [Lesson 6](/tutorial/6-rolling-upgrade-thinking.md) for
> the support window and the run invariant.

**You will learn:** how to create a gate that is dormant and decoupled from everyone
else's, how to turn it on when the feature is ready, why an activation version is a
receipt and not a roadmap, and what to do with a feature that slips past its planned
release.

## The setup

Ana, Ben, Chloe and David each own a feature. They share one `main` and the current
activation version is **4**.

| Feature | Owner | What it is | Needs fleet coordination? |
|---|---|---|---|
| F1 | Ana | A new report format | No |
| F2 | Ben | A faster scheduler | No |
| F3 | Chloe | An extra CLI subcommand | No |
| F4 | David | A new on-disk record layout | **Yes** (old and new readers must agree) |

None of them wants a branch that lives for weeks. So each merges to `main` now, with
the feature **off**, behind a gate. The rest of this lesson is how.

## Step A — create the gate, dormant

A dormant gate is one spec, at the **current** activation version, `Alpha`, and
`Default: false`:

```go
var Registry = gatekit.Registry{
    F1: {Specs: gatekit.VersionedSpecs{{ActiveFrom: 4, Default: false, Maturity: gatekit.Alpha}}},
    F2: {Specs: gatekit.VersionedSpecs{{ActiveFrom: 4, Default: false, Maturity: gatekit.Alpha}}},
    F3: {Specs: gatekit.VersionedSpecs{{ActiveFrom: 4, Default: false, Maturity: gatekit.Alpha}}},
    F4: {Specs: gatekit.VersionedSpecs{{ActiveFrom: 4, Default: false, Maturity: gatekit.Alpha}}},
}
```

Two things to notice, and they are the whole point:

- **`ActiveFrom` is 4 — the current version, not a target.** None of these raises
  `max`, so none moves `min`. Adding F1 cannot affect F2, F3 or F4. Four engineers add
  four gates and never touch each other or the activation window. This is the decoupling
  you came for.
- **`Alpha` + `Default: false` means off for everyone.** The code ships on `main` and
  changes nothing for any user. A half-finished feature is safe to merge.

Each engineer also gates the code path and, if it is a command or flag, labels it by
maturity (Lesson 4). From here, F1 to F3 look identical. F4 is different because it needs
coordination — hold that thought until Step C.

## Step B — work on it, and test it while it is dormant

The feature is off, so how does its owner try it? With an **override**, the
highest-precedence input of all — above the version, the default, and the `Alpha`
opt-in.

```go
// In a unit test
gate, _ := gatekit.New(Registry, gatekit.StaticVersion(4),
    gatekit.WithOverrides(map[gatekit.Feature]bool{F2: true}))

// On a real binary, from the environment
//   SOLO_PROVISIONER_ENABLE_F2=true ./solo-provisioner ...
gate, _ := gatekit.New(Registry, source,
    gatekit.WithEnvOverrides("SOLO_PROVISIONER_ENABLE_"))
```

| Entry | Normal run | With `--experimental` | With an override |
|---|---|---|---|
| `Alpha`, `Default: false` | off | **off** | the override's value |
| `Alpha`, `Default: true` | off | on | the override's value |
| `Beta`/`GA`, version reached | on | on | the override's value |

The surprise is the middle column. An `Alpha` spec is off unless the gate is opted in,
and once opted in it follows its `Default` — so `Default: false` stays off even with
`--experimental`. While a feature is genuinely hidden, its owner tests it with an
override. (Overrides and the `--experimental` flag are covered in
[Lesson 4, steps 4b–4c](/tutorial/4-build-and-release-a-feature.md).)

An override is a per-process switch for testing and emergencies. It is not how you roll
a feature out.

## Step C — turn it on when it is ready

Here the four features split by that last column of the setup table.

**F1, F2, F3 — no coordination needed.** They turn on for everyone at the *current*
version, with no new activation version and no finalize. There are two ways:

```go
// Option 1: promote in place — flip the one spec to Beta/GA. On at deploy, fleet-wide.
F1: {Specs: gatekit.VersionedSpecs{{ActiveFrom: 4, Default: true, Maturity: gatekit.Beta}}},

// Option 2: once it has baked, just delete the gate and keep the new code path.
```

This is the everyday path, and the stacked-PR replacement. The feature was merged
dormant, tested by override, then switched on — and the activation version never moved.

**F4 — needs coordination.** A new on-disk layout means old and new binaries must agree,
so F4 is a *compatibility* change, and only this kind earns a new activation version. It
adds a second spec at the next integer, may carry a migration, and turns on at a
deliberate `finalize`:

```go
F4: {Specs: gatekit.VersionedSpecs{
    {ActiveFrom: 4, Default: false, Maturity: gatekit.Alpha}, // dormant while building
    {ActiveFrom: 5, Default: true,  Maturity: gatekit.Beta},  // activates at version 5
}},
```

So the dividing line is not "how big is the feature" but **"does the fleet have to agree
to turn it on?"** If no, it never needs the activation machinery. If yes, that is exactly
what the activation version, migrations and `finalize` are for (Lesson 5).

## The trap: "just move it to version 11"

Now plans change. Say F4 was loosely "planned for version 9" and slips, and someone
says: *"move F4 to activation version 11."* It sounds like editing a calendar. It is a
trap.

Three rules collide:

1. **The activation version goes up by at most 1 per release.** "Version 9" implies 6, 7
   and 8 all exist. 11 needs a real 10.
2. **`max` is the highest `ActiveFrom` of any spec, and `min = max − W`** (Lesson 6).
3. **The run invariant refuses to start when `active < min`.**

Put numbers on it. The fleet runs at `active = 4`, `W = 1`. Someone adds one F4 spec at
`ActiveFrom: 11`. Now `max` is 11 and `min` is 10, so every node at 4 is below `min` and
**refuses to start** — the whole fleet, over a feature that is still off. It happens even
though the spec is `Alpha` and `Default: false`, because `max` counts every `ActiveFrom`
regardless of maturity or default.

> **The activation version is not a plan. It is a receipt.**
> A release hands out the *next* integer when a gated change actually becomes reachable.
> Until then, a feature has no version of its own.

Keep the roadmap in your tickets ("targets the next bump"), never in the registry. The
registry only holds versions that exist or are about to ship.

## When a dormant feature slips below the window

Plans slip for real reasons. Suppose Ana and Chloe ship F1 and F3 (bump to 5), then an
urgent breaking change ships (bump to 6) — all while David's F4 is still being built,
sitting dormant at version 4. With `W = 1`, `min` is now 5, and **F4's spec at 4 is below
`min`.**

This is fine. A below-`min` dev gate is harmless:

- **It still resolves.** The effective spec is the highest `ActiveFrom ≤ active`; a lone
  spec at 4 still qualifies at any `active` of 5 or 6. F4 resolves off, and the override
  still works for David.
- **The run invariant is untouched.** It checks `active` against `[min, max]`, not
  individual specs. A spec below `min` is perfectly legal; the binary starts normally.
- **The removal lint stays quiet.** It flags a spec only when a *later* spec supersedes
  it at or below `min`. F4 has one spec, so there is nothing to flag.
- **It does not drag `min` down or widen the window.** With `W = 1`, `min` is `max − 1`
  regardless of F4.
- **David can still finish it.** Flipping the version-4 spec to `Beta` turns F4 on for
  every supported binary at once (all are at `active ≥ 4`), or David graduates it at the
  next version like any compatibility change.

So nothing breaks. But a gate that has drifted below `min` has lived through more than one
activation cycle without being finished, removed, or graduated — a smell worth surfacing:

> **Keep a dormant gate within the window (`ActiveFrom ≥ min`).** If it drifts below,
> **warn** — do not fail the build. A feature can legitimately miss its window when
> urgent work ships ahead of it, and a below-`min` gate is harmless. The warning is a
> nudge to finish it, remove it, or graduate it when you can.

This is the opposite of the removal lint (which fires on a stale *earlier* spec), and it
is a warning on purpose. A hard failure would punish a team for shipping an urgent fix
ahead of an unfinished feature — exactly the flexibility gates are supposed to give you.

## The runbook

Every gated feature moves through the same stages. The question at each step:
**does this change behavior for everyone at a higher version, or not?** If yes, add a new
entry. If not, edit the entry you already have.

| # | Stage | Registry entry | New entry or edit? |
|---|---|---|---|
| 1 | **Build.** Merge the unfinished work, dormant. | `{ActiveFrom: <current>, Default: false, Maturity: Alpha}` | New feature |
| 2 | Keep working. Test with an override. | No change. | None |
| 3 | **Ready to try.** Others can opt in. | Same entry, `Default: true` | **Edit in place** |
| 4a | **Turn on (no coordination).** | Same entry, `Maturity: Beta`/`GA`, or delete the gate | **Edit in place** |
| 4b | **Activate (needs coordination).** | `{ActiveFrom: <next>, Default: true, Maturity: Beta}` | **New entry** + finalize |
| 5 | **GA.** Declare it stable. | The live entry, `Maturity: GA` | **Edit in place** |
| 6 | **Clean up.** Remove dead entries. | Delete superseded specs when the lint reports them | Removal |

- `<current>` is today's `max`. **Never a future target.**
- `<next>` is `max + 1`, and only compatibility changes (step 4b) ever use it. Features
  that activate in the same release **share** that one integer.

### Why some steps are edits, not new entries

A feature **cannot have two specs with the same `ActiveFrom`** — `gatekit.New` rejects it
(core-spec §2), so versions within a feature strictly increase. That is why:

- **Step 3** edits an `Alpha` default. `Alpha` is not guaranteed across versions, so the
  flip only affects gates that opted in.
- **Steps 4a and 5** relabel maturity. `Beta` and `GA` resolve identically; `Maturity`
  only matters for the `Alpha` opt-in and the `Deprecated` lint.

Only **step 4b** changes what *everyone* gets at a *new* version, so it alone is a new
entry and a new bump.

## The four features, replayed

Start at `active = 4`, `W = 1`. All four merge dormant gates at version 4 (Step A).

| Release | What changes | `max` | `min` | Operator action |
|---|---|---|---|---|
| Start | F1–F4 added as `Alpha`, off, at 4 | 4 | 4 | none |
| R1 | F1, F3 ready, no coordination: flipped to `Beta` **in place at 4** | 4 | 4 | none |
| R2 | F2 ready, same: `Beta` in place at 4. An urgent breaking change bumps to **5** | 5 | 4 | finalize 4 → 5 |
| R3 | Another urgent breaking change bumps to **6**. F4 still dormant at 4 → now below `min` (5): **warning** | 6 | 5 | finalize 5 → 6 |
| R4 | F4 finally ready: new `Beta` spec at **7** + its migration | 7 | 6 | finalize 6 → 7 |

Notice what never happened: nobody wrote 9 or 11. F1, F2 and F3 turned on without
consuming a version at all. The activation version only ever moved for the breaking
changes and for F4 — the features that actually needed the fleet to agree. And when F4
slipped below the window at R3, it was a warning, not a wall.

## Common mistakes

| Mistake | What happens | Do this instead |
|---|---|---|
| `ActiveFrom` set to a future target | `max` and `min` jump; nodes refuse to start | Use the current version; take the next integer only at release |
| Giving a no-coordination feature a new version | Needless finalize and coupling | Flip it on in place at the current version |
| Two specs with the same `ActiveFrom` | `gatekit.New` errors | Edit in place, or use a higher version |
| `Alpha`, `Default: false`, tested with `--experimental` | Still off | Use an override (or `Default: true`) |
| Changing a *released* `Beta`/`GA` default | Old and new binaries disagree | Add a new spec at the next version |
| Jumping the version by more than 1 | `min` leaps past the fleet | At most +1 per release |
| Treating a drifted dormant gate as a build failure | Punishes shipping urgent work first | Warn, and finish/remove/graduate when you can |

## A checklist for every gated-feature PR

- [ ] Does this change even need a gate? (Only a contract-breaking change must be gated — Lesson 4.)
- [ ] Is the first spec at the **current** version, not a future target?
- [ ] Does this feature need fleet coordination? If no, plan to turn it on in place; if yes, it earns a new version at release.
- [ ] Am I adding a new entry only for a coordinated change at a higher version, and editing in place otherwise?
- [ ] Can a developer exercise the feature while it is dormant (a test override)?
- [ ] Did I run the removal lint and delete every spec it reports?

## Check yourself

- Why does adding one F1 gate not affect F2, F3 or F4? *(It is a single spec at the
  current version, so it does not raise `max` or move `min`; only a new, higher version
  touches the shared window.)*
- Which of F1–F4 need a new activation version to turn on, and why? *(Only F4 — it needs
  the fleet to agree on a new on-disk layout. F1–F3 flip on in place.)*
- Why can a spec at `ActiveFrom: 11` stop a fleet at 4 from starting? *(It makes `max` 11
  and `min` 10, so `active = 4` is below `min` and the run invariant refuses — even though
  the feature is off.)*
- F4 is dormant at version 4 and `min` has risen to 5. Is anything broken? *(No. It
  resolves off, the binary starts, the removal lint stays quiet, and the override still
  works. It only earns a warning to finish, remove, or graduate it.)*
- Should a below-`min` dormant gate fail CI? *(No — warn. A feature can slip when urgent
  work ships first, and a below-`min` gate is harmless.)*

## Further reading

- [gatekit core spec §2, §4, §6](https://github.com/automa-saga/gatekit-spec/blob/main/core-spec.md)
  — registry validity, maturity, and the author obligation.
- [gatekit activation spec §2, §4](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md)
  — how `max` and `min` are derived, and the at-most-one-per-release rule.
- [Declaring features](/manual/declaring-features.md) — the task-oriented reference.

**Next:** [Other languages](/tutorial/go.md) · or jump to the [FAQ](/faq.md).
