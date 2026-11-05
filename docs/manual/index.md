# User manual

Task-oriented documentation for people who already know *why* they want gatekit and
need to know *how*. If you are new to the ideas, start with the
[Tutorials](/tutorial/index.md) first.

The examples use the Go API, the only released implementation. The
[specs](https://github.com/automa-saga/gatekit-spec) are authoritative where
anything here disagrees.

| Chapter | Covers |
|---|---|
| [Installing](/manual/installing.md) | Adding gatekit; choosing the gate core, the migration layer, or the durable runner |
| [Declaring features](/manual/declaring-features.md) | The registry, versioned specs, maturity |
| [Resolving gates](/manual/resolving-gates.md) | `Enabled`, version sources, overrides, alpha opt-in, reports |
| [Activation](/manual/activation.md) | The `{min, max, active}` triple, support window, run invariant, immediate vs. deferred |
| [Migrations](/manual/migrations.md) | Writing migrations, scopes, plans, dry-run preview |
| [Finalize & rollback](/manual/finalize-and-rollback.md) | `RunNow`, `Finalize`, the one-way line, the durable runner |
| [Implementing activation](/manual/implementing-activation.md) | A worked design: state store, startup, synchronized vs. rolling, finalize command, auto-finalize |
| [Testing against conformance](/manual/conformance-testing.md) | Running the shared fixtures |
| [Troubleshooting](/manual/troubleshooting.md) | Why a feature is off, range errors, migrations that did not run |

To add a page, see [Contributing to the docs](/contributing-docs.md).
