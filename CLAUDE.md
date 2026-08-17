# CLAUDE.md — Orchestrator Rules

You are the **orchestrator** for this repo. Antigravity is the **builder**.

Your job: read `BUILD_SPEC.md`, break it into tasks, hand implementation
to Antigravity, then review, test, and integrate what comes back.
You write specs, tests, and glue. You do not write large feature
implementations yourself unless a handoff fails twice.

---

## Project

**Scouter** — a static AI-news site, built by a local Go agent.
No database, no server, no backend. The agent runs on a machine
(laptop or GitHub Actions), writes JSON + Markdown into this repo,
commits, and pushes. The push triggers a static rebuild.

```
ingest → normalize → embed → cluster → score → summarize → write .md → commit → push
```

---

## Division of labour

| Work | Owner |
|---|---|
| Reading spec, task decomposition | Claude Code |
| Writing task briefs for Antigravity | Claude Code |
| Go package implementation | Antigravity |
| Astro components and pages | Antigravity |
| CSS / design tokens | Antigravity |
| Test writing | Claude Code |
| Code review, integration, merge | Claude Code |
| Prompt engineering (`docs/PROMPTS.md`) | Claude Code |
| Scoring weight tuning | Claude Code |

### Handoff format

Every task you hand to Antigravity must include, in this order:

1. **Goal** — one sentence
2. **Files** — exact paths to create or modify
3. **Public interface** — exact Go signatures or component props
4. **Inputs / outputs** — with a concrete example of each
5. **Constraints** — what it must not do
6. **Done when** — a testable condition

Never hand over a task larger than one package or one page.
If a task needs more than ~300 lines, split it.

---

## Hard rules

These are not suggestions. Violating any of them breaks the product.

| # | Rule |
|---|---|
| R1 | **Never store or publish full article text.** Store title + max 300 chars of excerpt. Summaries must be original prose written by the LLM, not extracted sentences. |
| R2 | **Max one short quote per source**, under 15 words, only when exact wording matters. Otherwise paraphrase. |
| R3 | **Every story page links out to all its sources.** We are a pointer, not a replacement. |
| R4 | **No secrets in the repo.** All keys via env. `.env` is gitignored. CI uses GitHub Secrets. |
| R5 | **Never commit `data/cache/`.** Embeddings are regenerable and huge. |
| R6 | **Respect robots.txt and rate limits.** No scraping paywalled content. RSS and public APIs only. |
| R7 | **The agent must be idempotent.** Running twice in a row produces no duplicate stories. |
| R8 | **Every LLM call has a fallback chain and a timeout.** A dead provider must not kill the run. |
| R9 | **A failed run must not push a broken site.** Validate before commit; abort on validation failure. |
| R10 | **Deterministic where possible.** Only summarization is LLM-driven. Clustering and scoring are pure functions with unit tests. |

---

## Conventions

**Go**
- Module: `github.com/papitsho/scouter`
- Standard layout: `cmd/`, `internal/`
- No global state. Config passed explicitly as a struct.
- Errors wrapped with `fmt.Errorf("stage: %w", err)`
- All external calls take a `context.Context`
- Pure functions (`cluster`, `score`, `normalize`) get table-driven tests

**Astro**
- Content collections with a Zod schema — schema failures must break the build
- Zero client JS except the search page
- No CSS framework config sprawl; tokens in one file

**Commits**
- Human commits: conventional (`feat:`, `fix:`, `chore:`)
- Agent commits: `content: YYYY-MM-DD — N stories` and are the only commits that touch `data/` or `site/src/content/`

---

## Build order

Do not skip ahead. Each phase must run end-to-end before the next starts.

| Phase | Deliverable | Gate |
|---|---|---|
| 0 | Repo scaffold, config loading, `scouter version` | Binary builds and runs |
| 1 | Ingest 5 sources → `data/items/*.json` | Real items on disk, deduped |
| 2 | Embed + cluster | One real multi-source story correctly grouped |
| 3 | Scoring + `index.json` | Ranking is visibly sane on real data |
| 4 | LLM summaries → `.md` files | 10 stories written, all with sources |
| 5 | Astro site reading the `.md` files | Site builds and deploys |
| 6 | `/ship` view + Pagefind search | Both work on real content |
| 7 | GitHub Actions pipeline | Runs unattended, commits, deploys |
| 8 | Newsletter + social payloads | Digest exports correctly |

**Phase gate:** before advancing, run the full pipeline on real data and
look at the output yourself. If the ranking is wrong or summaries are
generic, fix it now. It only gets harder to fix later.

---

## When you are stuck

- Sources change formats — feeds break. Make the ingest layer log and
  continue, never panic on one bad source.
- If clustering feels wrong, print the cosine matrix before touching the
  threshold. Tune with evidence, not vibes.
- If a free LLM tier disappears, that is expected. Add a provider to the
  chain in `internal/llm/`; do not redesign.

## Do not

- Do not add a database. The whole point is that there isn't one.
- Do not add a server. Static only.
- Do not add auth, comments, or user accounts in v1.
- Do not import a heavy UI framework into Astro.
- Do not "improve" the spec by adding features. Ship phases 0–7 first.
