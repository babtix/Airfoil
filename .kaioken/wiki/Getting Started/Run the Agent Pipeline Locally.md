# Run the Agent Pipeline Locally

This chapter demonstrates how to execute the full Airfoil agent pipeline locally using the `run` command, from environment setup through output inspection. The pipeline performs: **ingest → normalize → embed → cluster → score → summarize → write → commit → push**.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Environment Configuration](#environment-configuration)
- [Building the Agent](#building-the-agent)
- [Running the Pipeline](#running-the-pipeline)
- [Inspecting Generated Outputs](#inspecting-generated-outputs)
- [Idempotent Re-runs](#idempotent-re-runs)
- [Validation and Troubleshooting](#validation-and-troubleshooting)

---

## Prerequisites

Before running the pipeline, ensure you have:

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.22+ | Build the agent binary |
| Git | 2.30+ | Commit and push generated content |
| Ollama | Latest | Local embedding model (default embedder) |
| Node.js | 20+ | Run the React dev server (optional, for site preview) |

The agent binary is built from `agent/cmd/airfoil/main.go`, which serves as the thin entry point:

`agent/cmd/airfoil/main.go:12-17`

```go
func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "airfoil:", err)
		os.Exit(1)
	}
}
```

The `newRootCmd()` function (defined in `root.go`, not shown in the provided source) registers the `run`, `version`, and `doctor` subcommands via Cobra.

---

## Environment Configuration

Copy the example environment file and populate required values:

```bash
cp .env.example .env
```

Key environment variables (all prefixed with `AIRFOIL_` or provider-specific):

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `AIRFOIL_CONFIG_DIR` | No | `config/` | Directory containing `sources.json`, `scoring.json`, `keywords.json` |
| `AIRFOIL_DATA_DIR` | No | `data/` | Output directory for `items/`, `index.json`, `state.json`, `stories.json`, `cache/` |
| `AIRFOIL_STORIES_DIR` | No | `site/src/content/stories/` | Markdown story output directory |
| `AIRFOIL_EMBEDDER` | No | `ollama` | Embedding provider: `ollama` (local) or `gemini` (CI) |
| `AIRFOIL_LOG_LEVEL` | No | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `OLLAMA_EMBED_MODEL` | If using Ollama | `nomic-embed-text` | Ollama embedding model name |
| `GEMINI_API_KEY` | If using Gemini | — | Google Gemini API key |
| `GEMINI_EMBED_MODEL` | If using Gemini | `text-embedding-004` | Gemini embedding model |
| `GEMINI_API_KEY` / `NVIDIA_NIM_API_KEY` / `OPENROUTER_API_KEY` | At least one | — | LLM summarization provider keys (fallback chain: Gemini → NVIDIA NIM → OpenRouter → Ollama) |
| `GITHUB_TOKEN` | No | — | GitHub token (raises rate limit from 60→5000/hr) |
| `REDDIT_USER_AGENT` | Yes for Reddit | — | Descriptive User-Agent string (required to avoid 429) |
| `SITE_URL` | No | — | Base URL for canonical links in generated stories |

The configuration loading mechanism (`internal/config.Load`) merges:
1. JSON files from `AIRFOIL_CONFIG_DIR` (`sources.json`, `scoring.json`, `keywords.json`)
2. Environment variables (override JSON)
3. Derived paths (`ItemsDir`, `CacheDir`, `IndexPath`, `StatePath`, `StoriesPath`, `EmbedCachePath`)

Validation (`Config.Validate()`) gates the pipeline — invalid config aborts before any network calls.

---

## Building the Agent

From the repository root:

```bash
cd agent
go build -o ../airfoil ./cmd/airfoil
cd ..
```

This produces the `airfoil` binary at the repo root. Verify it works:

```bash
./airfoil version
./airfoil doctor   # Validates config, embedder connectivity, and write permissions
```

The `doctor` command checks:
- Config directory exists and JSON files are valid
- Embedder (Ollama/Gemini) is reachable
- LLM provider keys are present (at least one in fallback chain)
- Output directories are writable
- Git repository is clean (no uncommitted changes outside `data/` and `site/src/content/stories/`)

---

## Running the Pipeline

Execute the full pipeline with:

```bash
./airfoil run
```

### Pipeline Execution Flow

Per the architecture, `./airfoil run` triggers:

1. **Config Load** — `config.Load(Options)` reads JSON + env, validates, derives paths
2. **Ingest** — `ingest.Runner.Run(ctx, state, since)` orchestrates adapters:
   - `hnAdapter` → Hacker News API
   - `rssAdapter` → RSS feeds (Reddit, Hugging Face Papers, GitHub all use RSS)
   - Each `Adapter.Fetch` returns `[]normalize.Raw` items
3. **Normalize** — `normalize.Build(raw, now, maxExcerpt)` → `model.Item`:
   - Excerpt capped at 300 chars (`normalize.Excerpt`)
   - Canonical URL (wrapper unwrapping, variant collapsing)
   - Repo/paper URL extraction (`ExtractRepoURL`, `ExtractPaperURL`)
4. **Dedupe** — `normalize.Dedupe` against `state.Seen` (from `data/state.json`) → new items only
5. **Persist Items** — New items written to `data/items/*.json` (one file per item)
6. **Embed** — Embedder (Ollama/Gemini per `AIRFOIL_EMBEDDER`) generates vectors, cached in `data/cache/`
7. **Cluster** — O(n²) cosine similarity clustering → `model.Cluster` with tier assignment (`TierMajor`/`TierNotable`/`TierMinor`)
8. **Score** — Tier weights (`scoring.json`) + keyword boosts (`keywords.json`) + source multipliers + recency decay → ranked `Index`
9. **Summarize** — LLM fallback chain (Gemini → NVIDIA NIM → OpenRouter → Ollama) → `model.Story`
10. **Write** — Markdown stories to `site/src/content/stories/*.md` + `data/index.json` + `data/state.json` + `data/stories.json`
11. **Validate** — Schema checks on generated outputs
12. **Commit & Push** — `git commit -m "content: YYYY-MM-DD — N stories"` → `git push`

### Command-Line Options

| Flag | Description |
|------|-------------|
| `--since duration` | Ingest window (default: 24h). Example: `--since 48h` |
| `--dry-run` | Execute pipeline but skip git commit/push |
| `--no-embed` | Skip embedding/clustering (use existing cache) |
| `--no-llm` | Skip summarization (use placeholder summaries) |
| `--config-dir string` | Override `AIRFOIL_CONFIG_DIR` |
| `--data-dir string` | Override `AIRFOIL_DATA_DIR` |

Example with custom window and dry-run:

```bash
./airfoil run --since 72h --dry-run
```

---

## Inspecting Generated Outputs

After a successful run, examine the generated artifacts:

### 1. Items (`data/items/`)

One JSON file per ingested item (named by `ItemID` — hash of canonical URL):

```json
{
  "id": "a1b2c3d4...",
  "title": "Example Title",
  "url": "https://example.com/article",
  "canonical_url": "https://example.com/article",
  "excerpt": "First 300 characters of cleaned content...",
  "source": "hackernews",
  "source_type": "community",
  "published_at": "2025-01-15T10:30:00Z",
  "repo_url": "",
  "paper_url": "",
  "cluster_id": ""
}
```

Key fields enforced during normalization:
- `excerpt` ≤ 300 chars (hard limit in `normalize.Excerpt`)
- `canonical_url` deduplicated via wrapper unwrapping
- `repo_url` / `paper_url` extracted when detectable

### 2. Index (`data/index.json`)

Ranked list of `IndexEntry` for site consumption:

```json
{
  "generated_at": "2025-01-15T12:00:00Z",
  "entries": [
    {
      "story_id": "cluster-abc123",
      "score": 94.7,
      "tier": "major",
      "title": "Major AI Breakthrough",
      "date": "2025-01-15",
      "tags": ["llm", "research"],
      "sources": ["hackernews", "github"]
    }
  ]
}
```

The site reads this at build time (`npm run build`).

### 3. Stories (`site/src/content/stories/`)

One Markdown file per cluster (story), with frontmatter:

```markdown
---
id: "cluster-abc123"
title: "Major AI Breakthrough"
date: "2025-01-15T10:30:00Z"
tier: "major"
tags: ["llm", "research"]
sources:
  - name: "hackernews"
    type: "community"
    url: "https://news.ycombinator.com/item?id=12345"
  - name: "github"
    type: "lab"
    url: "https://github.com/org/repo"
cluster_id: "cluster-abc123"
score: 94.7
---

## Summary

Original LLM prose summarizing the cluster. Not extracted sentences.
Maximum one short quote (<15 words) per source.

## Sources

- [Hacker News discussion](https://news.ycombinator.com/item?id=12345)
- [GitHub repository](https://github.com/org/repo)
```

Frontmatter mirrors `site/src/types/story.ts` exactly (`Story`, `Source`, `SourceType`).

### 4. State (`data/state.json`)

Deduplication tracker (per-day keys):

```json
{
  "seen": {
    "a1b2c3d4...": "2025-01-15",
    "e5f6g7h8...": "2025-01-15"
  }
}
```

`State.Seen(id)` / `State.MarkSeen(id, day)` enables idempotent re-runs.

### 5. Stories Index (`data/stories.json`)

Array of full `Story` objects (mirrors Markdown frontmatter + body) for programmatic access.

### 6. Embedding Cache (`data/cache/`)

**Never committed to Git**. Contains vector embeddings keyed by `ItemID`. Regenerable from items.

---

## Idempotent Re-runs

Running `./airfoil run` twice in the same day produces **zero new stories** on the second run:

1. `State.Seen(id)` checks `data/state.json` for today's date
2. `normalize.Dedupe` drops items already marked seen
3. No new items → no new clusters → no new stories → no git commit

To force re-processing, either:
- Wait for a new day (date key changes)
- Manually remove entries from `data/state.json`
- Use `--since` with a window that includes new items not yet seen

---

## Validation and Troubleshooting

### Pre-commit Validation

The pipeline validates before committing:
- All generated JSON is valid and matches expected schemas
- Every story has required frontmatter fields
- No duplicate `cluster_id` values
- Markdown files render without errors

Validation failure **aborts before commit** — never pushes broken site.

### Common Issues

| Symptom | Cause | Resolution |
|---------|-------|------------|
| `doctor` fails: "Ollama not reachable" | Ollama not running | `ollama serve` and `ollama pull nomic-embed-text` |
| `doctor` fails: "No LLM provider keys" | Missing API keys | Set at least one: `GEMINI_API_KEY`, `NVIDIA_NIM_API_KEY`, `OPENROUTER_API_KEY` |
| Reddit returns 429 | Missing/poor `REDDIT_USER_AGENT` | Set descriptive UA: `REDDIT_USER_AGENT="airfoil/1.0 (contact@example.com)"` |
| GitHub rate limited | No `GITHUB_TOKEN` | Add token (60→5000 req/hr) |
| Pipeline hangs | LLM timeout | Each provider has timeout; fallback chain continues. Check logs with `AIRFOIL_LOG_LEVEL=debug` |
| `data/cache/` huge | Embeddings accumulate | Never commit `data/cache/`; safe to delete anytime (regenerates) |
| O(n²) clustering slow | >600 items | Expected; consider `--no-embed` for testing |

### Logs

Structured logging via `log/slog`. Set `AIRFOIL_LOG_LEVEL=debug` for detailed stage timing:

```bash
AIRFOIL_LOG_LEVEL=debug ./airfoil run --since 24h
```

Output includes per-stage duration, item counts, cluster sizes, and LLM provider used.

---

## Referenced Files

- `agent/cmd/airfoil/main.go` — CLI entry point (thin, delegates to `root.go`)
- Architecture brief (authoritative system model) — pipeline stages, data models, config, conventions

> **Note**: The provided source scope includes only `agent/cmd/airfoil/main.go`. The full pipeline implementation resides in `internal/config`, `internal/ingest`, `internal/normalize`, `internal/store`, `internal/model`, and `cmd/airfoil/root.go` — described in the architecture brief but not included in the source block.

<!-- kaioken:files agent/cmd/airfoil/main.go -->
