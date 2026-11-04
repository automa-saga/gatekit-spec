# User manual

Task-oriented documentation for people who already know *why* they want gatekit and
need to know *how*. If you are new to the ideas, start with the
[Tutorials](../tutorial/index.md) first.

> **Status:** the manual is being written. Each chapter below is a planned page;
> until it exists, the linked spec section or tutorial lesson is the best source.
> The specs are authoritative where anything here is missing or disagrees.

## Planned chapters

| Chapter | Covers | Until then, read |
|---|---|---|
| Installing | Adding gatekit to a project; choosing the gate core vs. the migration layer | [Other languages](../tutorial/go.md) |
| Declaring features | The registry, versioned specs, maturity levels | [Core spec](https://github.com/automa-saga/gatekit-spec/blob/main/core-spec.md), [Lesson 4](../tutorial/4-build-and-release-a-feature.md) |
| Resolving gates | How a feature resolves against the activation version | [Appendix A](../appendix/algorithm.md) |
| Activation | Immediate vs. deferred activation; the `{min, max, active}` triple; the support window | [Activation spec](https://github.com/automa-saga/gatekit-spec/blob/main/activation-spec.md), [Lesson 5](../tutorial/5-activation-use-cases.md) |
| Migrations | Writing migrations; run-now vs. held; plan computation | [Migration spec](https://github.com/automa-saga/gatekit-spec/blob/main/migration-spec.md) |
| Finalize & rollback | Finalizing a fleet; reversibility; crash safety | [Lesson 6](../tutorial/6-rolling-upgrade-thinking.md) |
| Testing against conformance | Running the shared fixtures in your implementation | [Appendix B](../appendix/conformance.md) |
| Troubleshooting | Common failure modes | [Appendix D](../appendix/spot-the-failure.md), [FAQ](../faq.md) |

To write one of these, see [Contributing to the docs](../contributing-docs.md).
