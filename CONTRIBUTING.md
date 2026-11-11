# Contributing to gatekit-spec

This repository is the **normative specification** for gatekit and its
language-neutral **conformance fixtures** — the product here is the
specification, not any one implementation. Contributions that clarify the spec,
fix or extend fixtures, or improve the docs are all welcome.

## Ways to contribute

- **Propose a spec change** — open an issue describing the behavior and the use
  case first; spec changes are discussed before they land (see
  [Proposing a spec change](#proposing-a-spec-change)).
- **Add or fix a conformance fixture** — see
  [Conformance fixtures](#conformance-fixtures).
- **Improve the docs** — wording, examples, and cross-references.

## Spec-first

gatekit is spec-first: observable behavior is defined here in language-neutral
terms, and implementations conform to it. The Go implementation
([automa-saga/gatekit](https://github.com/automa-saga/gatekit)) is the first
conformant implementation, **not** the definition — **where an implementation and
the spec disagree, the spec is authoritative.**

The documents:

- [`core-spec.md`](core-spec.md) — the feature-gate model and deterministic
  resolution.
- [`activation-spec.md`](activation-spec.md) — the `{min, max, active}` triple,
  the run invariant, the support window, activation/finalize, and the serialized
  formats.
- [`migration-spec.md`](migration-spec.md) — the migration contract, plan
  computation, and execution.

## Proposing a spec change

1. Normative keywords (MUST, SHOULD, MAY) follow
   [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119). Keep each section's status
   accurate (draft / stable / reserved — see the status table in the README).
2. If the change alters observable behavior, add or update the **conformance
   fixtures** in the same PR so every implementation can verify it.
3. A spec change and the matching implementation land as separate PRs: the spec
   change here first, then the implementation (which vendors this repo as a
   submodule) bumps its pointer to it.

## Conformance fixtures

Fixtures are plain-JSON inputs + expected outcomes under
[`conformance/`](conformance), grouped by area (`resolution/`, `invariant/`,
`planning/`, `serialization/`). Each group's schema is documented in
[`conformance/README.md`](conformance/README.md), and every fixture is enumerated
in [`conformance/manifest.json`](conformance/manifest.json).

To add a fixture:

1. Create the `.json` file under the right group, following that group's schema.
2. **Register its path** in `conformance/manifest.json` under the matching group
   (an unregistered fixture is rejected by CI).
3. Validate locally — no dependencies required:

   ```sh
   python3 scripts/validate_fixtures.py
   ```

   This checks that the manifest is well-formed, every listed fixture exists and
   is valid JSON, and every on-disk fixture is registered in exactly one group.
   The same check runs on every pull request.

## Commits & sign-off

- **DCO sign-off is required.** Every commit must be signed off, certifying the
  [Developer Certificate of Origin](https://developercertificate.org/). Add the
  `Signed-off-by` trailer with `git commit -s`.
- **Commit messages** follow the
  [Conventional Commits](https://www.conventionalcommits.org/) format.

## Pull requests

- Open your PR against `main`.
- PRs are **squash-merged**, and the **PR title becomes the squashed commit
  message** — so the PR title **must** follow Conventional Commits (e.g.
  `docs: add negotiation message spec`, `fix: correct a planning fixture`).
- Keep PRs focused, and ensure the fixtures check is green before requesting
  review.

## Releases

Releases are cut by the **Deploy Release Artifact** workflow (Actions tab, run on
`main`), which uses [semantic-release](https://semantic-release.gitbook.io/) to
read the Conventional Commit titles since the last tag, pick the next version,
tag it, and publish the GitHub release notes. Tick **Perform Dry Run** first to
preview the version. Only `feat`, `fix`, `perf` and breaking changes trigger a
release; `docs`, `ci` and `chore` do not. While the spec is `0.x`, a breaking
change bumps the minor version.

## License

By contributing, you agree that your contributions will be licensed under the
project's [Apache License 2.0](LICENSE).
