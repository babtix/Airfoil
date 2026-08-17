# Airfoil — Build Spec

A static AI-news site with a local Go agent that does the work.
No database. No backend. Git is the database.

---

## 1. Product

**Positioning:** AI news, read like a builder.
Covers everything, but ranks by "does this change what I can build today?"

| Ranks high | Ranks low |
|---|---|
| Model releases, pricing/context changes | Funding with no product |
| Open weights, benchmarks with artifacts | Executive drama, lawsuits |
| SDKs, APIs, dev tools, notable repos | Op-eds and predictions |
| Breaking changes, deprecations | Conference announcements |
| Papers with working code | Papers with no code |

**Core mechanic:** 20 outlets covering one OpenAI release = **1 story**, not 20.
Clustering is what makes this not an RSS reader.

**Routes**

| Route | Content |
|---|---|
| `/` | Today's digest — top 5 stories |
| `/feed` | Everything, reverse chronological |
| `/top` | Ranked by score, 7d / 30d toggle |
| `/ship` | Builder cut: releases, repos, API changes only |
| `/story/[slug]` | Summary + every source link |
| `/tags/[tag]` | Tag archive |
| `/search` | Client-side, Pagefind |
| `/rss.xml` | Full feed |
| `/archive/[year]/[month]` | Date archive |

---

## 2. Architecture

```
┌─────────────────────────────────────────┐
│  Local machine  OR  GitHub Actions      │
│                                         │
│   airfoil run                           │
│     ingest → normalize → embed →        │
│     cluster → score → summarize →       │
│     write → publish                     │
└──────────────┬──────────────────────────┘
               │ git push
               ▼
        ┌─────────────┐
        │   GitHub    │  ← the database
        └──────┬──────┘
               │ webhook
               ▼
        ┌─────────────┐
        │   Vercel    │  build → static
        └─────────────┘
```

Same binary/script runs locally and in CI. One code path.

---

## 3. Repo layout

```
airfoil/
├── CLAUDE.md
├── AGENTS.md
├── BUILD_SPEC.md
├── .env.example
├── .gitignore
│
├── agent/                          # Agent module
│   ├── main.py / cmd/airfoil/main.go
│   └── internal/
│       ├── config/                 # env + config/sources.json loading
│       ├── model/                  # Item, Story, Source, Cluster types
│       ├── ingest/                 # rss, hn, reddit, hf, github
│       ├── normalize/              # canonical URL, dedupe, excerpt
│       ├── embed/                  # provider interface: ollama, gemini
│       ├── cluster/                # cosine + greedy agglomerative
│       ├── score/                  # pure scoring function
│       ├── llm/                    # provider chain: gemini → nvidia_nim → openrouter → ollama
│       ├── write/                  # markdown + frontmatter emit
│       ├── digest/                 # newsletter + social payloads
│       └── store/                  # JSON read/write, atomic
│
├── config/
│   ├── sources.json                # all feeds, tiers, tags
│   ├── scoring.json                # weights, tunable without rebuild
│   └── keywords.json               # builder signals, hype penalties
│
├── data/
│   ├── items/YYYY-MM-DD.json       # raw items, pruned at 30d
│   ├── index.json                  # slim story index
│   ├── state.json                  # seen-URL hashes, per-source cursors
│   └── cache/embeddings.json       # GITIGNORED
│
├── site/                           # Astro
│   ├── astro.config.mjs
│   ├── src/
│   │   ├── content/
│   │   │   ├── config.ts           # Zod schema
│   │   │   └── stories/*.md        # the product
│   │   ├── layouts/
│   │   ├── components/
│   │   ├── pages/
│   │   └── styles/tokens.css
│   └── public/
│
├── docs/
│   ├── PROMPTS.md
│   └── RUNBOOK.md
│
└── .github/workflows/pipeline.yml
```

---

## 4. Data model

### Item — one article from one source

```go
type Item struct {
    ID          string    `json:"id"`           // sha256(canonicalURL)[:16]
    SourceID    string    `json:"source_id"`
    URL         string    `json:"url"`          // canonical
    Title       string    `json:"title"`
    Excerpt     string    `json:"excerpt"`      // MAX 300 chars, hard cap
    Author      string    `json:"author,omitempty"`
    PublishedAt time.Time `json:"published_at"`
    FetchedAt   time.Time `json:"fetched_at"`
    Metrics     Metrics   `json:"metrics"`
    RepoURL     string    `json:"repo_url,omitempty"`
    PaperURL    string    `json:"paper_url,omitempty"`
}

type Metrics struct {
    HNPoints     int `json:"hn_points,omitempty"`
    HNComments   int `json:"hn_comments,omitempty"`
    RedditScore  int `json:"reddit_score,omitempty"`
    HFUpvotes    int `json:"hf_upvotes,omitempty"`
    GitHubStars  int `json:"github_stars,omitempty"`
}
```

**R1 enforcement:** `Excerpt` is truncated at ingest time, in
`normalize`, before anything is written to disk. There is no code path
that persists more than 300 characters of source text.

### Story — a cluster, after summarization

Written as `site/src/content/stories/YYYY-MM-DD-slug.md`:

```markdown
---
id: "a3f9c21b8e04d7f2"
title: "Anthropic ships Claude Opus 5 with 500K context"
summary: "One-paragraph original summary. Never extracted sentences."
score: 87
tier: "major"          # major | notable | minor
tags: ["models", "anthropic", "context-window"]
builder_relevant: true
date: 2026-08-17T09:12:00Z
cluster_size: 14
sources:
  - name: "Anthropic Blog"
    url: "https://..."
    tier: 1
  - name: "Hacker News"
    url: "https://..."
    tier: 5
    metrics: { points: 1240, comments: 380 }
takeaways:
  - "Context jumps from 200K to 500K"
  - "Pricing unchanged per million tokens"
  - "Available on the API today"
---

Optional longer body. Usually empty in v1.
```

### index.json — slim, for feed/top/ship views

```json
{
  "generated_at": "2026-08-17T09:30:00Z",
  "stories": [
    { "id": "a3f9...", "slug": "2026-08-17-claude-opus-5",
      "title": "...", "score": 87, "tier": "major",
      "tags": ["models"], "builder_relevant": true,
      "date": "2026-08-17T09:12:00Z", "cluster_size": 14 }
  ]
}
```

### state.json — idempotency

```json
{
  "seen_urls": { "sha256hash": "2026-08-17" },
  "cursors": { "hn": "1755412800", "reddit_localllama": "t3_abc123" },
  "last_run": "2026-08-17T09:30:00Z"
}
```

`seen_urls` entries older than 30 days are pruned each run.
This is what makes R7 (idempotency) true.

---

## 5. Pipeline stages

### 5.1 Ingest

Reads `config/sources.json`. Each adapter returns `[]Item`.

| Adapter | Sources | Notes |
|---|---|---|
| `rss.go` | Lab blogs, press, arXiv | `gofeed` library |
| `hn.go` | Hacker News | Algolia API, query AI keywords, `points>50` |
| `reddit.go` | r/LocalLLaMA, r/MachineLearning, r/singularity | Public `.json` endpoints, set User-Agent |
| `hf.go` | HuggingFace daily papers | Public API |
| `github.go` | Trending AI repos | Search API, `created:>7d`, sorted by stars |

**Rules**
- Every adapter runs in its own goroutine, bounded by `errgroup`
- A failing source logs a warning and returns empty — never fails the run
- 15s timeout per source
- If more than half of all sources fail, abort the run before writing

### 5.2 Normalize

| Step | Detail |
|---|---|
| Canonical URL | Strip `utm_*`, `ref`, `fbclid`, `?source=`; lowercase host; drop trailing slash; resolve known shorteners |
| ID | `sha256(canonicalURL)[:16]` |
| Dedupe | Drop if ID in `state.seen_urls` |
| Excerpt | Strip HTML, collapse whitespace, **truncate to 300 chars** |
| Title clean | Strip source suffix (`" | TechCrunch"`, `" - The Verge"`) |
| Time | Normalize to UTC; if missing, use `FetchedAt` |
| Extract | Detect GitHub repo URL and arXiv ID in body |

### 5.3 Embed

```go
type Embedder interface {
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    Dimensions() int
}
```

| Provider | When | Model |
|---|---|---|
| `ollama` | Local runs, default | `nomic-embed-text` |
| `gemini` | CI runs | `text-embedding-004` |

Embed `title + " " + excerpt[:200]`. Cache by item ID in
`data/cache/embeddings.json`, 7-day window, gitignored.
Batch requests. Never re-embed a cached item.

### 5.4 Cluster

Greedy agglomerative, single-link, over a **48-hour window**.

```
for each unclustered item i (sorted by published_at desc):
    find cluster c maximizing cosine(centroid(c), embed(i))
    if best_similarity >= THRESHOLD (default 0.82):
        add i to c, recompute centroid
    else:
        create new cluster with i
```

Hard override: if two items share the same canonical URL **or** the same
extracted repo/arXiv URL, force them into one cluster regardless of cosine.

n ≈ 600 items → O(n²) is ~360k comparisons. Fine. Do not optimize.

**Cluster title** = title of the highest-tier, earliest item in the cluster.

Tunable in `config/scoring.json`. Ship an `airfoil cluster --debug` flag
that prints pairs between 0.75 and 0.90 so the threshold can be tuned
against real data instead of guesses.

### 5.5 Score

Pure function. Fully unit-tested. Output normalized to 0–100.

```
raw =   W_sources  * log(1 + distinct_source_count)
      + W_tier     * max_tier_weight(cluster)
      + W_hn       * log(1 + hn_points)
      + W_reddit   * log(1 + reddit_score)
      + W_builder  * builder_signal_count
      - W_hype     * hype_signal_count

score = clamp(raw * recency_decay, 0, 100)
recency_decay = exp(-ln(2) * age_hours / 48)
```

**Source tiers** (in `sources.json`)

| Tier | Weight | Examples |
|---|---|---|
| 1 | 1.00 | Official lab blogs — Anthropic, OpenAI, DeepMind, Meta AI, Mistral |
| 2 | 0.85 | Research — arXiv, HuggingFace papers |
| 3 | 0.80 | Dev/tools — GitHub trending, official SDK changelogs |
| 4 | 0.50 | Press — TechCrunch, VentureBeat, The Verge |
| 5 | 0.30 | Community — HN, Reddit |

**Builder signals** (`keywords.json`, +1 each, cap 5): release, open-source,
open weights, API, SDK, benchmark, deprecated, breaking change, pricing,
context window, self-host, quantized, fine-tune, has repo URL, has paper URL.

**Hype signals** (−1 each, cap 5): shocking, game-changer, you won't believe,
will replace all, insane, mind-blowing, the end of, nobody is talking about.

**Tier assignment**

| Score | Tier | Treatment |
|---|---|---|
| ≥ 70 | `major` | Summarized, digest candidate, larger card |
| 40–69 | `notable` | Summarized |
| < 40 | `minor` | Index entry only, **no LLM call** |

**Budget cap:** max 15 LLM summary calls per run, highest score first.
`builder_relevant = true` when builder signal count ≥ 2.

### 5.6 Summarize

Provider chain in `internal/llm/`:

```go
type Provider interface {
    Complete(ctx context.Context, sys, user string) (string, error)
    Name() string
}
```

| Order | Provider | Notes |
|---|---|---|
| 1 | Gemini free tier | Highest free daily request ceiling |
| 2 | OpenRouter `:free` | ~20 req/min, 50/day unfunded — fallback only |
| 3 | Ollama local | If running locally and both above fail |

Chain rules: 30s timeout each, one retry with jitter on 429/5xx, then
next provider. If all fail for a cluster, skip that cluster (index entry
only) and continue — do not fail the run.

Prompts live in `docs/PROMPTS.md`. Output is strict JSON, validated
against a schema. Retry once on parse failure, then skip.

**R1/R2 enforcement in code, not just in the prompt:**
- Reject any summary containing a 12+ word substring found verbatim in any source excerpt
- Reject any summary over 80 words
- Reject more than one quoted span
- On rejection: retry once, then skip the cluster

### 5.7 Write

- Slug: `YYYY-MM-DD-` + kebab-case title, truncated to 60 chars, deduped with `-2`
- Write `.md` atomically (temp file + rename)
- Update `index.json`
- Update `state.json`
- **Never overwrite an existing story file.** If a cluster grows later,
  append new sources to the existing file's frontmatter and bump `score`.
  Title and summary stay frozen — URLs must be stable for SEO.

### 5.8 Digest

Generates, but does not send:

| Output | Path |
|---|---|
| Newsletter HTML + text | `data/digest/YYYY-MM-DD.html` / `.txt` |
| X thread | `data/digest/YYYY-MM-DD-x.json` |
| LinkedIn draft | `data/digest/YYYY-MM-DD-linkedin.md` |

LinkedIn stays semi-manual by design — your commentary is why people follow.

### 5.9 Publish

```
1. Validate: index.json parses, every .md has required frontmatter,
   every story has ≥1 source, no summary exceeds 80 words
2. Prune: items/ older than 30d, seen_urls older than 30d
3. git add data/ site/src/content/stories/
4. git commit -m "content: 2026-08-17 — 12 stories"
5. git push
```

Validation failure = abort before commit (R9).

---

## 6. CLI
 
| Command | Does |
|---|---|
| `airfoil ingest` | Sources → `data/items/` |
| `airfoil cluster` | Embed + group. `--debug` prints similarity pairs |
| `airfoil rank` | Score, write `index.json` |
| `airfoil write` | LLM summaries → `.md` |
| `airfoil digest` | Build newsletter + social payloads |
| `airfoil publish` | Validate, prune, commit, push |
| `airfoil run` | All of the above |
| `airfoil run --dry` | Everything except LLM calls and push |
| `airfoil doctor` | Check env, providers, feed reachability |
 
Global flags: `--config`, `--data`, `--verbose`, `--since`
 
---
 
## 7. Site
 
**Stack:** React + Vite, TypeScript, Tailwind / Design Tokens.
 
Content schema mirrors the frontmatter exactly.
 
**Design direction:** dense and readable, not a blog template.
Reference: Hacker News density with modern typography.
 
| Token | Value |
|---|---|
| Body | Inter or system sans |
| Meta / scores / tags | JetBrains Mono |
| Score display | Monospace, right-aligned, always visible |
| Tier styling | `major` = larger + accent bar, `notable` = normal, `minor` = compact single line |
| Dark mode | Default, respects `prefers-color-scheme` |
| Density | Feed shows ~25 stories per screen on desktop |
 
Every story card shows: score, title, source count, primary source name,
relative time, and a `SHIP` badge when `builder_relevant`.
 
**Performance targets:** Lighthouse 100 across the board. Fast client-side transitions.
 
---
 
## 8. CI
 
`.github/workflows/pipeline.yml`
 
```yaml
on:
  schedule:
    - cron: '0 6,12,18 * * *'   # 3x daily UTC
  workflow_dispatch:
```
 
Steps: checkout → setup python/node → `airfoil run` → commit → push.
Vercel auto-deploys on push.
 
**Free-tier constraints, designed around:**
 
| Constraint | Handling |
|---|---|
| Actions disables cron after 60d repo inactivity | Pipeline commits on every run — repo never goes idle |
| Actions cron fires late under load | Never assume exact timing anywhere |
| Vercel Hobby cron is once-per-day only | Not used. Actions is the scheduler |
| Vercel Hobby forbids Git-org repos | Repo must live under a personal account |
| Free LLM models rotate out | Provider chain is config-driven |
 
Env via GitHub Secrets: `GEMINI_API_KEY`, `NVIDIA_NIM_API_KEY`, `OPENROUTER_API_KEY`.
Embedder in CI = `gemini`.
 
---
 
## 9. Phase gates
 
| Phase | Done when |
|---|---|
| 0 | `airfoil version` runs; config loads; sources.json parses |
| 1 | 5 sources ingested, real items on disk, second run adds zero duplicates |
| 2 | One real multi-outlet story correctly grouped; `--debug` output inspected |
| 3 | Top 10 by score looks right to you on real data |
| 4 | 10 stories written; every summary passes R1/R2 checks |
| 5 | Site builds, deploys, renders real stories |
| 6 | `/ship` filters correctly; Pagefind returns results |
| 7 | Unattended CI run commits and deploys with no human touch |
| 8 | Digest files generate correctly |

---

## 10. Explicitly out of scope for v1

Comments. User accounts. Personalization. Mobile app. Podcast.
Translations. Paid tier. Full-text article storage. Real-time updates.
Any database. Any server.

Ship phases 0–7. Then reassess.
