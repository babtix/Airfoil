# Airfoil — Build Progress

**Last updated:** 2026-08-18
**Scope of this document:** state of the Go agent (`agent/`). Written as a handoff
so work can resume without the originating conversation.

---

## 1. Where the build is

| Phase | Deliverable | State |
|---|---|---|
| 0 | Module scaffold, config, model, store | **Done, gate met** |
| 1 | `normalize` + 5 ingest adapters | **Done, gate met** |
| 2 | Embed + cluster | **Done, gate met** — tuned against real NIM embeddings |
| 3 | Scoring → `ranked.json` | **Code done, tests green. Gate open — weights need a human call, see §4 blocker 3** |
| 4 | LLM chain + markdown writer | **Done, gate met** — 6 real stories written, all passing R1/R2 |
| 5 | `publish` + `doctor --ping` | **Done** — 484 stories validate; LLM + embedder + feeds all probed |
| 6 | GitHub Actions | **Done** — `.github/workflows/pipeline.yml`, cron 06/12/18 UTC |
| 7 | Digest | **Done** — newsletter (HTML + text), X thread, LinkedIn draft |

`go test ./...` is green across all twelve packages with tests. The site builds
and renders the real output — see §3.

**No local models, no Gemini.** Ollama was removed on 2026-08-18 because the
pipeline's real home is CI, where no local daemon is listening — a local
fallback passes on a laptop and fails in the only place that matters. Gemini
was removed the same day: the project has no Google API access, and a provider
that can never authenticate is noise in the chain and in `doctor` output.

The chain is **NVIDIA NIM → OpenRouter**, and both are verified working.

### Decisions taken (the docs hedged; these resolved them)

| Fork | Decision |
|---|---|
| Go vs Python | **Go** — matches CLAUDE.md conventions and the BUILD_SPEC struct definitions |
| Who builds | Claude Code directly, not Antigravity handoffs |
| Data contract | Agent writes **all three**: `.md` (canonical) + `index.json` + `stories.json`. The last one uses the **site's** field names (`ts`, `cluster`) because the site imports it directly — see §3 bug 5 |
| Embedder | **NVIDIA NIM** (`nvidia/nemotron-3-embed-1b`) — the only embedder |
| Local models | **Removed** on owner's instruction. Hosted APIs only — see §1 |
| Gemini | **Removed** on owner's instruction (2026-08-18): no Google API access. Chain is NIM → OpenRouter |

---

## 2. What exists

```
agent/
├── go.mod                          module github.com/papitsho/airfoil
├── cmd/airfoil/                    version · doctor · ingest · cluster · rank
│                                   write · publish · digest · run · tui
└── internal/
    ├── config/    env + .env parser + 3 JSON files, validated at startup
    ├── model/     Item, Metrics, Cluster, Story, State, Index
    ├── store/     atomic JSON r/w, items by day, state load/save
    ├── normalize/ PURE — canonical URL, ID, excerpt cap, title clean, extract
    ├── ingest/    rss · hn · reddit · hf · github, errgroup + fail-soft
    ├── embed/     Embedder iface → nvidia_nim, + disk cache
    ├── cluster/   PURE — cosine, centroid, greedy agglomerative
    ├── score/     PURE — weighted formula, signals, tags, ranking
    ├── llm/       provider chain: nvidia_nim → openrouter
    ├── summarize/ PURE prompt + R1/R2 validators, plus retry orchestration
    ├── write/     markdown + frontmatter, index.json, stories.json
    ├── publish/   PURE validators + prune, then git commit
    ├── digest/    newsletter, X thread, LinkedIn draft — generates, never sends
    ├── pipeline/  stages as callable ops, shared by CLI and TUI
    └── tui/       terminal UI (built by the TUI fork)
```

Dependencies are exactly the AGENTS.md allowlist: `gofeed`, `cobra`,
`golang.org/x/sync`. Everything else is standard library, including the `.env`
parser.

### Working CLI

```bash
cd agent && go build -o bin/airfoil.exe ./cmd/airfoil
```

| Command | State |
|---|---|
| `airfoil version` | Done |
| `airfoil doctor [--ping]` | Done. `--ping` probes every source URL and the embedder live |
| `airfoil ingest [--dry]` | Done |
| `airfoil cluster [--debug]` | Done |
| `airfoil rank [--explain] [--top N] [--dry]` | Done |
| `airfoil write [--dry]` | Done — summarize, validate, publish three outputs |
| `airfoil publish [--dry] [--push]` | Done — validate, prune, commit. Push is opt-in |
| `airfoil digest [--day] [--dry]` | Done — writes to `data/digest/`, sends nothing |
| `airfoil run [--dry] [--push]` | Done — chains every stage |
| `airfoil tui` | Built by the TUI fork |

Global flags: `--config` `--data` `--verbose` `--since`.

---

## 3. Verified against real data

One live run, 2026-08-17:

- **16 of 17 sources succeed**, 498 items on disk across 3 day-files
- **Max excerpt 299 chars, zero R1 violations**
- **Second consecutive run added 0 items** — R7 idempotency holds
- 437 items carry a paper URL, 36 carry a repo URL
- Cross-outlet merge confirmed **with a real embedder** (2026-08-18):
  TechCrunch + 2 independent HN threads on the Amazon rare-books story
  grouped into one 3-item, 2-source cluster and ranked #1 — the core
  product mechanic working end to end, ingest through rank

### Source fixes applied to `config/sources.json`

Four feeds in the original config were dead. Verified by probing, not guessed:

| Source | Was | Now |
|---|---|---|
| Anthropic | `anthropic.com/rss.xml` (404) | **Disabled** — no public feed exists on any documented path. Coverage arrives via HN and press |
| Meta AI | `ai.meta.com/blog/rss/` (404) | `engineering.fb.com/feed/`, renamed "Meta Engineering" |
| Mistral | `mistral.ai/news/feed.xml` (404) | `mistral.ai/rss.xml` |
| — | — | **Added** `blog.google/technology/ai/rss/` as a tier 1 |

### Bugs found and fixed during the phase-1 gate

1. **`Accept-Encoding: gzip` set by hand** disabled Go's transparent
   decompression, so every response arrived still compressed. This alone was
   failing 8 sources. Never set that header.
2. **Reddit JSON returns 403** unauthenticated. The `.rss` listing is still
   public, so the adapter tries JSON, falls back to RSS, and loses only the
   score metric. `min_score` cannot apply on the fallback path.
3. **The OpenAI feed serves its entire 1,132-entry history.** Ingest now
   defaults to the clustering window, cutting a run from 2,630 items to ~500.
   `--since` backfills deliberately.
4. **`mailto:` / `javascript:` URLs** were being prefixed with `https://` and
   parsed into plausible-looking hosts. Now rejected.

### Bug found during the phase-5 site gate

5. **`stories.json` did not match the shape the site imports.** The site reads
   `data/stories.json` directly at build time and types it as
   `site/src/types/story.ts`, which declares `cluster`, `ts` and `body`. The
   writer was emitting `cluster_size`, `date`, and omitting `body` — so the
   relative timestamps, the cluster badges and the 24H/7D filters would all
   have received `undefined`.

   Nothing failed: Go tests passed, the site built, TypeScript compiled. It
   would only have shown up as quietly wrong dates in a browser. `model.Story`
   now carries the site's JSON tags, `Body` always serializes as `[]` rather
   than `null`, and `internal/write/contract_test.go` pins the wire shape so a
   field rename fails a test instead of a page.

### Bug found while checking whether the project was done

6. **Orphaned markdown pages were invisible to `publish`.** The writer never
   deletes files, and a slug is derived from its title — so when
   `stories.json` was rebuilt to fix bug 5, the LLM produced slightly
   different titles, new slugs were written, and the previous six pages stayed
   on disk referenced by nothing. `publish` validated only in-memory data, so
   it would have committed them as dead URLs the site's own data does not know
   about.

   `publish.ValidateStoryFiles` now compares the stories directory against
   every slug in `stories.json` and reports what nothing points at. It reports
   rather than deletes: R9's job is to stop the run and say what is wrong, and
   silently removing files is not safe for a scheduled job. The six stale
   pages were removed by hand.

### Site gate — verified in a browser, 2026-08-18

`npm run build` succeeds, and the dev server renders real pipeline output:

- **484 stories** in the feed, zero console errors, no `Invalid Date`/`NaN`
- Source counts match the corpus: arXiv cs.AI ×268, HN ×44, TechCrunch ×8
- The Amazon story renders **`1D AGO · 3 SRC`** — relative time from `ts` and
  the cluster badge from `cluster`, the two fields that were broken
- 10 multi-source cards render correctly

---

## 4. Open blockers

### Blocker 4 — OPEN: OpenRouter's key is dead, so the chain has no fallback

`airfoil doctor --ping` now probes each provider individually — deliberately
not through the chain, which stops at the first success and would hide a dead
fallback until the day it is needed. Current state:

| Provider | Result |
|---|---|
| `nvidia_nim` | **ok**, ~0.9s |
| `openrouter` | **ok**, ~0.9s |

**Both providers work. The earlier "OpenRouter is dead" finding was wrong.**

The 401 was real, but the key in `.env` was not the cause. A stale
`OPENROUTER_API_KEY` was exported in the shell, and the dotenv loader
deliberately does not override a variable that is already set — that is what
lets CI secrets take precedence over a local file. The agent kept using the old
dead key while `curl`, reading `.env` directly, succeeded with the good one.

Verified with `env -u OPENROUTER_API_KEY airfoil doctor --ping`: both providers
answer in under a second. If `doctor --ping` ever reports a 401 that `curl` does
not, check the shell environment before suspecting the file.

### Blocker 1 — RESOLVED 2026-08-18: embedder key works; LLM chain now verified

`NVIDIA_NIM_API_KEY` was added and **confirmed working** —
`airfoil doctor --ping` embeds a real probe string through it successfully
(see blocker 2 for the full embedder story: switched from the default
`nv-embedqa-e5-v5` to `nvidia/nemotron-3-embed-1b`). `AIRFOIL_EMBEDDER` is now
`nvidia_nim` in `.env`. This unblocked Phase 2's gate.

`NVIDIA_NIM_MODEL=nvidia/nemotron-3-super-120b-a12b` drives the **chat** side
of the same NIM account, and is confirmed working: it produced six real
story summaries, all passing R1/R2 on the first or second attempt.

`airfoil doctor --ping` now covers the LLM chain too. It found that OpenRouter
is still dead — see blocker 4 above, which is the remaining open item.

Also still true: **OpenRouter serves no embedding models** — 414 models in the
catalogue, zero with embeddings. It is chat-completions only, which is why the
embedder had to come from NIM regardless of OpenRouter's key status.

### Blocker 2 — RESOLVED 2026-08-18: clustering threshold, tuned against real NIM data

`NVIDIA_NIM_API_KEY` landed. First embedder tried was the config default,
`nvidia/nv-embedqa-e5-v5` — it authenticated but reproduced the same failure
mode as Ollama's `mxbai-embed-large`: the known real cross-outlet story (see
below) measured cosine 0.65–0.75 between its own pairs, while unrelated arXiv
papers chained together above 0.82. Likely cause: `nv-embedqa-e5-v5` is a
retrieval/QA model (asymmetric query-vs-passage, hence `input_type: "passage"`
in the request), not a symmetric similarity model — the wrong tool for
event-clustering regardless of threshold.

Switched to `NVIDIA_NIM_EMBED_MODEL=nvidia/nemotron-3-embed-1b` (2048 dims).
Measured the same known pair directly, computing cosine between the three
items that make up one confirmed real story — TechCrunch's "Amazon destroying
rare texts" piece and two independent HN threads on the same event:

```
0.7479  HN "AirTag reveals..."        <-> TechCrunch "destroying rare texts"
0.7320  HN "AirTag reveals..."        <-> HN "We Tracked a Shipment..."
0.6114  TechCrunch                    <-> HN "We Tracked a Shipment..."
```

Swept the full 498-item corpus from 0.60 to 0.85. Below ~0.65 the same
single-link chaining problem reappears (a 98-item cluster of unrelated papers
at 0.60). From 0.68–0.74 the corpus is stable: largest cluster stays at 4,
multi-item cluster count declines smoothly, and the real Amazon story
(3 items, 2 distinct sources) clusters correctly across the entire plateau.

**Set `clustering.similarity_threshold = 0.70`** — mid-plateau, and the last
threshold at which the weaker leg (HN↔HN2 at 0.7320) still links transitively
into the full 3-item story rather than dropping to 2. Applied directly to
`config/scoring.json` with the evidence trail in its `$comment` field — this is
a config value, not a code change, and is fully re-tunable.

Re-ran `airfoil cluster` and `airfoil rank` against the real config. The
Amazon story now correctly places **#1** with 2 distinct sources — the
multi-source term produced a real result for the first time. Nine other
multi-item clusters formed, all plausible (same-source near-duplicate arXiv
papers, one HN cluster on a GitHub outage), none runaway.

**No cross-source-only restriction was needed.** With the right embedding
model, plain cosine separates signal from noise well enough on its own — the
proposed deviation from BUILD_SPEC's pure-cosine design in the earlier draft
of this blocker is no longer necessary and was not applied.

### Blocker 3 — OPEN: scoring weights need a human call, now on real clustered data

With blocker 2 resolved, `airfoil rank` runs against a corpus where
multi-source clusters actually exist, and the result is a genuine improvement:
the Amazon story now ranks **#1** with 2 distinct sources — the multi-source
term worked correctly for the first time.

But it's still the only multi-source item that reaches the top 20. Ranks 2–17
are single-source tier-5 Hacker News posts; no lab blog appears in the top 20.
Decomposing rank #2 (a single HN post, 44 pts, no multi-source boost):

| Term | Contribution |
|---|---|
| `W_hn * ln(1+points)` | ~36 |
| `W_tier * 0.30` (tier 5) | 7.5 |

The HN term alone still outweighs the tier term's **maximum** of 25 for a
tier-1 lab post. In an early scratch-config test, lowering `weights.hn` to 3.0
and `weights.reddit` to 2.5 rebalanced the ordering but pushed every story
below the `notable` cutoff (0 major, 0 notable, LLM budget 0) — the 70/40
tier thresholds may also need to move, not just the HN/Reddit weights.

**No weights were changed in `config/scoring.json`.** Unlike the clustering
threshold, this isn't a factual question with a measurable right answer — it's
a product-priority call (should a single viral HN post ever outrank a lab
announcement?) that BUILD_SPEC's own gate says should be "inspected by
[the owner]" before being applied. Recommend reviewing `airfoil rank --explain
--top 20` yourself and deciding the HN/Reddit/tier weight balance, one knob
per pass per BUILD_SPEC's tuning rule.

---

## 5. Rules held so far

| Rule | How it is enforced |
|---|---|
| R1 | `normalize.Excerpt` caps at 300 chars, and `normalize.Build` is the only constructor of `model.Item`. Verified: max 299 on real data |
| R3 | Every item keeps its canonical URL; cluster items are all retained |
| R4 | No secrets in code. `.env` parsed at runtime, gitignored |
| R5 | `data/cache/` gitignored and confirmed untracked |
| R6 | RSS and public APIs only; descriptive User-Agent on every request |
| R7 | `state.seen_urls` + `normalize.Dedupe`. Verified on real data |
| R8 | 15s per-source timeout, one jittered retry on 429/5xx, fail-soft |
| R9 | Ingest aborts before writing if more than half of sources fail. Fired correctly on the first live run |
| R10 | `normalize`, `cluster`, `score` are all pure, all table-driven tested. `summarize` and `publish` validators are pure too |
| R2 | `summarize.Validate` caps quotes at 1 and length at 15 words, counting straight *and* curly quotes |

R1 is enforced three times over: the excerpt cap in `normalize`, the summary
word cap and 12-gram verbatim check in `summarize`, and a final re-check in
`publish` before anything is committed.

---

## 6. TUI integration surface

A `internal/pipeline` package has been added (by the TUI fork) that exposes
stages as callable operations, so the CLI and TUI drive identical code:

```go
p := pipeline.New(cfg, log)
p.Ingest(ctx, pipeline.Options{Since: d, Dry: true})
p.Cluster(ctx, pipeline.Options{Debug: true})
p.LoadClusters()
p.ClustersPath()
```

`Rank` is now on the pipeline too:

```go
p.Rank(ctx, pipeline.Options{Dry: true})   // → RankResult{Results, Major, Notable, Minor, Budgeted}
p.LoadRanked()
p.RankedPath()
```

**The duplication noted earlier is resolved** — every CLI command now goes
through `internal/pipeline` via the `a.pipeline()` helper on the app struct.
There is one implementation of each stage.

What a TUI needs to reach:

| Surface | Path / API |
|---|---|
| Editable config | `config/sources.json`, `scoring.json`, `keywords.json` |
| Config round-trip | Structs now carry `$comment` fields, so comments survive an edit |
| Validation | `cfg.Validate()` returns every problem at once, as a joined list |
| Secrets | `.env`, parsed by `internal/config`. Never write keys into `config/` |
| Generated data | `data/items/YYYY-MM-DD.json`, `state.json`, `clusters.json`, `index.json`, `stories.json` |
| Per-source results | `ingest.Result.Sources` — id, fetched, kept, duration, error |
| Tuning loop | `cluster.Result.Borderline` — pairs in the debug range, sorted |
| Logging | `log/slog`; the TUI should inject its own handler to capture output |

---

## 7. Next steps, in order

All seven phases are built. What remains is tuning and operational setup.

1. **Clear the stale `OPENROUTER_API_KEY` from your shell.** The key in `.env`
   works; an older one exported in the shell shadows it and makes the fallback
   look dead (§4, blocker 4). Nothing to fix in code.
2. **Review `airfoil rank --explain --top 20`** and decide the HN/Reddit vs.
   tier weight balance (§4, blocker 3). This is a product-priority call, not a
   measurable one, which is why it was left alone.
3. **Add the CI secrets** the workflow expects: `NVIDIA_NIM_API_KEY` and
   `OPENROUTER_API_KEY`. Repository variables: `SITE_URL`, `REDDIT_USER_AGENT`.

### Known issue, deferred

**Pagefind cannot index this site as built.** It crawls static HTML; Vite emits
one `index.html` for the SPA, so there are no per-story pages. BUILD_SPEC's
phase-6 gate assumes there are. Fix is either prerendering routes or a
client-side index built from `stories.json`. Not a blocker until the search
phase.

### Housekeeping

Nothing has been committed — all work is staged or untracked in the working
tree.

`data/items/*.json`, `data/state.json`, `data/index.json` and
`data/stories.json` now all hold **real** pipeline output. The 12 hand-authored
mock stories were cleared (recoverable: `git checkout d947d34 -- data/stories.json`).
`data/clusters.json` and `data/ranked.json` are gitignored intermediates.

**Ten stale mock markdown files are still tracked** under
`site/src/content/stories/` — the ones dated 08-11 through 08-16 plus
`2026-08-17-claude-opus-5-500k-context.md`. They are fiction, they use tags
outside the allowed vocabulary, and the writer will never overwrite them
because they match no real cluster URL. They were left in place rather than
deleted unprompted; the six files generated on 2026-08-17 are the real ones.

**A secret leak was caught before it shipped:** `.env.example` is tracked and
contained live `NVIDIA_NIM_API_KEY` and `OPENROUTER_API_KEY` values. It has
been blanked. The keys were never committed — verified with
`git log -S` across all branches — so no history rewrite is needed. `.env`
itself is gitignored and holds the working key.
