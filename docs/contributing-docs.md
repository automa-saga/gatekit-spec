# Contributing to the docs

This site is a zero-build [docsify](https://docsify.js.org) site served from
`docs/` on GitHub Pages. There is no build step: add a Markdown file, link it from
the sidebar, and it is live once merged.

Preview locally:

```bash
npx docsify-cli serve docs
```

## The three kinds of page

Keep the kinds separate so readers know what to expect.

| Kind | Folder | Purpose |
|---|---|---|
| **Tutorial** | `docs/tutorial/` | Teaches an idea from first principles, in order, with examples. |
| **Manual** | `docs/manual/` | Task-oriented how-to for a reader who already knows the ideas. |
| **Reference / appendix** | `docs/appendix/` | Exact rules, precise and complete; no teaching. |

The normative specs stay at the repository root and are authoritative over any page here.

## Adding a tutorial

1. Create `docs/tutorial/<number>-<slug>.md` (follow the numbering of the existing
   lessons), or a standalone `docs/tutorial/<slug>.md` for a topic outside the main course.
2. Add it to the *Tutorials* section of [`_sidebar.md`](_sidebar.md).
3. Link it from [`tutorial/index.md`](tutorial/index.md), in the lesson table or a reading path.

## Adding a manual page

1. Create `docs/manual/<slug>.md`.
2. Add it to the *User manual* section of [`_sidebar.md`](_sidebar.md).
3. Replace the matching row in the planned-chapters table in
   [`manual/index.md`](manual/index.md) with a link to the new page.

## Style

- Keep real technical terms; add a [glossary](appendix/glossary.md) entry and link
  the first use, with a short inline gloss.
- Examples must run against the real API. Mark anything not yet implemented as planned.
- Link between pages with site-absolute paths (`/tutorial/go.md`), not `../`: on GitHub Pages the site lives under `/gatekit-spec/`, and `../` links escape that prefix and 404.
