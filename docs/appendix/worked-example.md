# Appendix E: A worked rollout

> This appendix puts the whole course to work on one concrete tool. We take a real
> rollout requirement, list what makes it hard, then solve it with gatekit end to end —
> declare, ship dormant, defer activation, run the migration at finalize, and keep a
> way back. If you have read Lessons [4](/tutorial/4-build-and-release-a-feature.md),
> [5](/tutorial/5-activation-use-cases.md), and
> [6](/tutorial/6-rolling-upgrade-thinking.md), this is where they come together.

> **This one is runnable.** The data-migration spine below is a real, working program you
> can build and run — [`examples/go/kv/`](https://github.com/automa-saga/gatekit-spec/tree/main/examples/go/kv).
> It runs the story in a **single process** for runnability: `cd examples/go && go run ./kv`
> writes records, holds the migration, and finalizes, stage by stage. The fleet-wide
> coexistence this scenario describes is shown live by its two-process companion,
> [`pingpong`](https://github.com/automa-saga/gatekit-spec/tree/main/examples/go/pingpong).
> The snippets below follow the kv source closely.

## The tool: `kv`, a small clustered key-value store

Imagine a modest distributed database — call it `kv`. It is a **cluster** of, say,
five identical server processes (nodes). Clients read and write keys; the nodes
**replicate** each write to each other so every node holds a copy. It runs on real
traffic, around the clock.

Two facts about `kv` matter for this story:

1. Each node **writes records to its own disk** in a particular on-disk format.
2. Nodes **send records to each other** over the network in a replication format.

Today both are "format **v1**."

## The change we want to ship

We want to improve how records are stored: a new **format v2** that adds an integrity
checksum. This is not a small internal tweak — it changes **both** facts above:

- what each node writes to **disk** (a stored-data change → a **migration**), and
- what nodes **send each other** (a wire-protocol change → nodes must **agree**).

A node speaking v2 to a node that only understands v1 is a broken cluster. So this is
the hardest kind of change: a shared format, across a running fleet.

## The rollout requirements

Before writing any code, write down what "done safely" means. For `kv`:

| # | Requirement | In plain terms |
|---|-------------|----------------|
| R1 | **Zero downtime** | The cluster keeps serving reads and writes the whole time. |
| R2 | **No flag day** | We cannot upgrade all five nodes at the same instant. They go one at a time. |
| R3 | **Mixed versions coexist** | While rolling, some nodes run the new binary and some the old. They must interoperate. |
| R4 | **The fleet flips together** | v2 (disk + wire) must turn on for *all* nodes at once, never node-by-node, or they split-brain. |
| R5 | **Rollback until committed** | If v2 misbehaves during the rollout, we must be able to go back to the old binary cleanly. |
| R6 | **Crash-safe migration** | Rewriting every record takes a while. If a node crashes mid-rewrite, it must resume, not corrupt or restart blindly. |

These six are the generic requirements of almost any fleet rollout. Now watch each one
map onto a gatekit mechanism.

## Requirements → gatekit, at a glance

| Requirement | gatekit mechanism |
|-------------|-------------------|
| R1 zero downtime | ship dormant; activate separately ([Lesson 1](/tutorial/1-why-feature-gating.md)) |
| R2 no flag day | deploy is behavior-neutral; `active` unchanged on install ([Lesson 6](/tutorial/6-rolling-upgrade-thinking.md), Rule 1) |
| R3 mixed versions coexist | the support window `W`; the run invariant ([Lesson 2](/tutorial/2-why-gatekit.md)) |
| R4 fleet flips together | deferred activation + `finalize` after readiness ([Lesson 5](/tutorial/5-activation-use-cases.md)) |
| R5 rollback until committed | the preserve-downgrade window; reversibility + the write floor |
| R6 crash-safe migration | a durable runner — [gatekit-automa](https://github.com/automa-saga/gatekit-automa) |

Let us build it.

## Step 1 — Declare the feature and its migration

The behavior change is "use record format v2," gated and introduced at the next
activation version (we are at 5; v2 is 6). The data change is a migration tied to that
same version. Because it changes the **wire** format too, its scope is `shared` — it
must be **run-at-finalize** for a coordinated finalize, not run at install.

```go
const RecordFormatV2 gatekit.Feature = "RecordFormatV2"

var Registry = gatekit.Registry{
    RecordFormatV2: {
        Description: "Store and replicate records in the checksummed v2 format",
        Specs: gatekit.VersionedSpecs{
            {ActiveFrom: 5, Default: false, Maturity: gatekit.Beta}, // born dormant
            {ActiveFrom: 6, Default: true,  Maturity: gatekit.GA},   // on at 6
        },
    },
}
```

```go
// The data migration, tied to activation version 6. shared → run at finalize.
type RewriteToV2 struct{ /* db handle, etc. */ }

func (RewriteToV2) ID() string            { return "rewrite-records-v2" }
func (RewriteToV2) Description() string    { return "Rewrite all records from v1 to v2" }
func (RewriteToV2) Version() int           { return 6 }
func (RewriteToV2) Scope() migration.Scope { return migration.Shared } // R4: run at finalize

// R6 groundwork: read ground truth, so a re-run after a crash is safe.
func (m RewriteToV2) Applies(ctx context.Context) (bool, error) { return anyRecordStillV1(ctx) }
func (m RewriteToV2) Execute(ctx context.Context) error         { return rewriteRecordsToV2(ctx) }
func (m RewriteToV2) Rollback(ctx context.Context) error        { return rewriteRecordsToV1(ctx) }
```

The gate decides which format the code uses; both paths exist side by side:

```go
if gate.Enabled(RecordFormatV2) {
    writeRecordV2(rec)   // new path — asleep until activation reaches 6
} else {
    writeRecordV1(rec)   // old path — what the cluster runs today
}
```

## Step 2 — Roll out the binaries, dormant (R1, R2, R3)

Deploy the new `kv` binary to the five nodes **one at a time**. Each new node knows
about version 6 (`max = 6`) but runs at the current `active = 5`, so it keeps writing
and replicating **v1** — exactly like the old nodes beside it. That satisfies R2 (one
at a time) and R3 (mixed versions interoperate, because everyone is still *behaving*
as v1).

Every node checks the run invariant at startup, which holds the fleet together (R3):

```go
if err := gate.Check(); err != nil { // min ≤ active ≤ max ?
    log.Fatalf("refusing to start: %v", err)
}
```

With `max = 6` and the default window `W = 1`, `min = 5`, so a node at `active = 5`
starts cleanly. No data has changed. The cluster has not noticed anything.

## Step 3 — Wait in the preserve-downgrade window (R5)

Now you **wait**. The whole fleet is on the new binary but still speaking v1. Nothing
irreversible has happened, so if you spot a problem you can roll the old binary back
cleanly — this is the **preserve-downgrade window** (R5). Stay here as long as you
like: hours, days. Confirm **readiness** — every node reports `max ≥ 6` — before moving
on.

## Step 4 — Finalize: the migration runs *here* (R4, R6)

When the fleet is ready, you finalize **once**. This is the moment the whole course has
been pointing at: the run-at-finalize migration runs at the point of activation, and only then
does the behavior turn on, for everyone, together.

```go
plan, _ := migration.NewPlanner(RewriteToV2{}).Plan(ctx, 5, 6)

// R6: a durable runner, so a crash mid-rewrite resumes instead of corrupting.
runner := gkautoma.NewRunner(gkautoma.WithJournal("/var/lib/kv/finalize.journal"))

if err := runner.Finalize(ctx, plan, store); err != nil {
    return err // records left consistent; active stayed 5; you can retry or roll back
}
// On success: every record is v2, and active is now 6 — the whole cluster speaks v2.
```

Two requirements are satisfied right here:

- **R4 (flip together).** `Finalize` runs the run-at-finalize `RewriteToV2` and *then* advances
  `active` to 6. The advance lands only if the migration succeeds, so the cluster
  crosses from v1 to v2 as one step — never node-by-node.
- **R6 (crash-safe).** Because the runner journals each step, a node that crashes
  halfway through the rewrite **resumes**: completed records are skipped (that is what
  `Applies` reading ground truth buys you), the interrupted work re-runs, and `active`
  advances only at the very end. With the plain `StdRunner` a crash would force you to
  re-run the whole thing by hand; `gatekit-automa` makes it automatic.

## Step 5 — Clean up next cycle (R-everything)

Finalize is the **one-way line**: the v2 rewrite is not perfectly reversible (you threw
away the v1 encoding), so gatekit will now **refuse** to lower `active` back across it —
it will not let you pretend a rollback is safe when it is not. From here you move
forward. When version 5 later drops out of the support window, you delete the old v1
code path and its spec ([Lesson 4, Step 6](/tutorial/4-build-and-release-a-feature.md)).

## The timeline, in one view

```
  deploy nodes 1..5 (dormant)      preserve-downgrade window        FINALIZE (once)
  ───────────────────────────────────────────────────────────────────────────────►
  active = 5, everyone speaks v1    rollback is safe                 RewriteToV2 runs
  run invariant passes on each      wait for readiness (max ≥ 6)     then active → 6
  (R1, R2, R3)                      (R5)                             cluster speaks v2
                                                                     crash-safe (R4, R6)
```

## What this would look like *without* gatekit

Strip the tool away and every requirement becomes a manual, error-prone ritual:

- You would hand-coordinate a "flag day" to flip all five nodes at once (breaks R1/R2),
  or write bespoke version-negotiation code per release.
- You would track "which nodes are new enough" in a spreadsheet or an ad-hoc script
  (R3 by hope).
- The record rewrite would be a separate migration script someone runs by hand, at some
  time only loosely related to when the code flips (R4/R6 by luck) — and if it crashed
  halfway, recovery would be a 2 a.m. judgment call.
- "Can we still roll back?" would be answered by memory, not by a guard rail that
  *refuses* an unsafe downgrade (R5 by nerve).

gatekit's contribution is not any single clever trick. It is that **all six
requirements are handled by the same small, deterministic model** — ship dormant,
activate on a dedicated version, run the migration at finalize, guard the window — so
the safe path is the default and the dangerous one refuses.

## Run it yourself

The whole walkthrough above is a runnable binary:

```bash
cd examples/go
go run ./kv          # or: go build ./kv && ./kv
```

It writes two records while dormant (stored v1), holds the migration, finalizes — watch
`formats` stay `[v1 v1]` until the `finalize` line — then writes a third record (now v2)
and reads them all back. That is the entire point: **the migration runs at activation, not
at deploy**, and the format a write uses follows the gate. The source is a handful of small
files (`record.go`, `store.go`, `kv.go`, `feature.go`, `migration.go`, `rollout.go`,
`main.go`) in
[`examples/go/kv/`](https://github.com/automa-saga/gatekit-spec/tree/main/examples/go/kv).

### A live two-peer version

Where `kv` focuses on the **data migration** in one process, its companion
[`examples/go/pingpong/`](https://github.com/automa-saga/gatekit-spec/tree/main/examples/go/pingpong)
shows the **protocol coexistence** across two **real processes** over TCP:

```bash
cd examples/go
go run ./pingpong pong     # terminal 1 — listens
go run ./pingpong ping     # terminal 2 — connects
# or both in one process:
go run ./pingpong demo
```

Ping and pong exchange messages while the wire format rolls from v1 to v2. Each message
advertises the sender's activation triple (`ev=min/max/active`, i.e.
`gate.EffectiveVersion()`). You can watch each peer get **upgraded** — its `max` rises
from 5 to 6, so it can read v2 — while it keeps **sending v1** (its `active` is still 5),
so it still interoperates with the peer that has not upgraded. Once both advertise
`max = 6`, they **negotiate a flip message number on the wire** (a `flip=` field) and
switch to v2 together at that point — the mixed-binary coexistence and one-flip story of
[Lesson 6](/tutorial/6-rolling-upgrade-thinking.md), live. Because a new binary reads both
formats, the peers need not switch in the same instant, only at the same agreed message
(decode both, send one).

## See also

- [Lesson 5: activation use cases](/tutorial/5-activation-use-cases.md) — the
  mechanics used here, in general form.
- [Lesson 6: rolling-upgrade thinking](/tutorial/6-rolling-upgrade-thinking.md) — the
  five design rules this rollout follows.
- [Appendix D: spot the failure](/appendix/spot-the-failure.md) — what happens when a rollout
  skips one of these steps.
- [Implementing activation](/manual/implementing-activation.md) — the how-to for building
  this wiring (state store, startup routine, finalize command) into your own tool.
