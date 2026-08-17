# Runbook

## Kickoff

Drop all these files at the repo root, then open Claude Code and paste:

```
Read CLAUDE.md and BUILD_SPEC.md. Then execute Phase 0.

Do not write feature code yourself — write the task brief for
Antigravity in the handoff format from CLAUDE.md, and I'll run it there.
When Antigravity returns code, review it against the spec and the hard
rules, write the tests, and integrate.

Stop at the Phase 0 gate and show me the result before continuing.
```

Then paste each brief into Antigravity, and paste its output back to
Claude Code for review. One phase at a time. Do not batch phases.

---

## Prerequisites

| Thing | Why | Note |
|---|---|---|
| Go 1.22+ | The agent | |
| Node 20+ | Astro | |
| Ollama + `nomic-embed-text` | Local embeddings | `ollama pull nomic-embed-text` |
| Gemini API key | Summaries, CI embeddings | Free tier |
| OpenRouter API key | Fallback | Free tier is ~50 req/day unfunded — fallback only, never primary |
| GitHub repo under **personal** account | Vercel Hobby rejects Git-org repos | |

---

## Daily operation

```bash
scouter doctor          # check providers and feeds before anything else
scouter run --dry       # full pipeline, no LLM calls, no push
scouter run             # for real
```

Review `data/digest/YYYY-MM-DD-linkedin.md` before posting. That one
stays manual.

---

## Tuning

Never tune two things in the same pass.

**Clustering wrong**
```bash
scouter cluster --debug     # prints pairs between 0.75 and 0.90
```
Same story split across clusters → lower threshold.
Unrelated stories merged → raise it. Move in 0.02 steps.

**Ranking wrong**
```bash
scouter rank && head -40 data/index.json
```
Press outranking lab blogs → raise `weights.tier`.
Old stories sticking around → shorten `recency.half_life_hours`.
Slop getting through → add phrases to `hype_signals`.

**Summaries wrong**
See the maintenance table at the bottom of `docs/PROMPTS.md`.
Rerun `scouter write --dry` over the same clusters to compare fairly.

---

## Failure modes

| Symptom | Cause | Fix |
|---|---|---|
| One source returns nothing | Feed URL changed | `scouter doctor`, update `sources.json` |
| Reddit returns 429 | Missing/generic User-Agent | Set `REDDIT_USER_AGENT` |
| All LLM calls fail | Free model rotated out | Update model ID in `.env`; the chain should already have fallen through |
| CI cron stopped firing | Actions disables schedules after 60d repo inactivity | Should never happen — pipeline commits every run. If it does, push manually to re-arm |
| Duplicate stories appear | `state.json` not written or pruned wrong | Check R7; the second run of `--dry` must produce zero new items |
| Site build fails after agent run | Frontmatter violates the Zod schema | This is the safety net working. Fix the writer, not the schema |
| Vercel won't connect the repo | Repo is under a Git organization | Move it to your personal account |

---

## Before going public

- [ ] `robots.txt` and `sitemap.xml` generated
- [ ] RSS feed valid
- [ ] Every story page links to every source
- [ ] No summary exceeds 80 words anywhere in the corpus
- [ ] Run the verbatim-overlap check across all existing stories, not just new ones
- [ ] OG images render
- [ ] Lighthouse 100/100/100/100
- [ ] An "About / how this works" page — state plainly that summaries are
      AI-generated and link to originals. This is both honest and a
      differentiator.
