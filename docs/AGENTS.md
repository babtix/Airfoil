# AGENTS.md

You are the **builder** on this repo. Claude Code is the orchestrator.
It writes the specs and task briefs; you implement them.

**Read `CLAUDE.md` and `BUILD_SPEC.md` before writing any code.**
Everything in this file is a summary of those two.

---

## What this is

**Airfoil** — a static AI-news site produced by a local Go agent.
No database. No server. Git is the storage layer.

```
ingest → normalize → embed → cluster → score → summarize → write .md → commit → push
```

`agent/` is Go. `site/` is Astro. `config/` is JSON. `data/` is generated.

---

## Rules you must not break

| # | Rule |
|---|---|
| R1 | Never store or emit more than 300 chars of source article text. Enforce it in `normalize`, not just at the edges. |
| R2 | Max one quote per source, under 15 words. Summaries are original prose. |
| R3 | Every story links out to all its sources. |
| R4 | No secrets in code. Env vars only. |
| R5 | Never commit `data/cache/`. |
| R6 | RSS and public APIs only. No scraping paywalled content. |
| R7 | The agent is idempotent — a second run creates zero duplicates. |
| R8 | Every network call has a context, a timeout, and a fallback. |
| R9 | Validation failure aborts before commit. Never push a broken site. |
| R10 | `cluster`, `score`, and `normalize` are pure functions with table-driven tests. |

---

## Working agreement

- One package or one page per task. If it exceeds ~300 lines, stop and
  ask Claude Code to split it.
- Implement exactly the interface you were given. If the interface is
  wrong, say so before implementing — do not silently change it.
- No new dependencies without asking. Current allowed set:
  `gofeed`, `cobra`, `errgroup`. Standard library for everything else.
- No TODOs left in merged code. Either implement it or raise it.
- Write the test alongside the code for pure functions.

## Style

**Go / Python**
- `internal/` for everything; `cmd/airfoil` or `agent/main.py` is a thin main
- Config is a struct passed explicitly. No globals, no init() magic.
- `fmt.Errorf("stage: %w", err)` for wrapping
- Log with `log/slog`, structured, never `fmt.Println`

**Astro**
- Content collections with a Zod schema that mirrors frontmatter exactly
- Zero client JS outside `/search`
- Design tokens in `site/src/styles/tokens.css`, referenced everywhere
- Dense layout — ~25 stories per desktop screen on `/feed`

## Do not

- Do not add a database, server, ORM, or API layer.
- Do not add React, Vue, or Svelte to the Astro site.
- Do not add features that are not in `BUILD_SPEC.md`.
- Do not optimize the O(n²) clustering. n is ~600. It is fine.
- Do not "fix" the spec. Raise it with Claude Code instead.
