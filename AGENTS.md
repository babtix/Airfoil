# AGENTS.md

**Airfoil** — a static AI-news site produced by a local Go agent. No database, no server; Git is the storage layer. The agent pipeline: `ingest → normalize → embed → cluster → score → summarize → write .md → commit → push`. `agent/` (Go), `site/` (React + Vite), `config/` (JSON), `data/` (generated).

## Commands

| Command | Directory | Notes |
|---------|-----------|-------|
| `go build -o bin/airfoil ./cmd/airfoil` | `agent/` | Build agent binary |
| `go test ./...` | `agent/` | Full suite |
| `./agent/bin/airfoil doctor --ping` | repo root | Probe embedder + every source live |
| `./agent/bin/airfoil run --dry` | repo root | Full pipeline, writes nothing |
| `./agent/bin/airfoil run` | repo root | Full pipeline (requires `.env`) |
| `npm run dev` | `site/` | Vite dev server |
| `npm run build` | `site/` | `tsc -b && vite build` |
| `npm run lint` | `site/` | Oxlint |
| `npm run preview` | `site/` | Preview production build |

## Architecture

- **Go module**: `github.com/papitsho/airfoil` — standard layout `cmd/airfoil` (thin main), `internal/` (all logic)
- **Config**: `config.Load(Options)` returns `*Config` struct; paths derived via `Config.ItemsDir()`, `CacheDir()`, `IndexPath()`, `StatePath()`, `StoriesPath()`, `EmbedCachePath()`
- **Data flow**: `data/items/*.json` (ingested) → `clusters.json` → `ranked.json` (both gitignored) → `data/stories.json` + `data/index.json` + `site/src/content/stories/*.md` (published). `state.json` carries dedup across runs.
- **Pure functions** (table-driven tests required): `normalize`, `cluster`, `score` in `internal/`
- **React entrypoints**: `site/src/main.tsx` → `App.tsx` (router) → pages in `site/src/pages/`
- **Story types**: the site imports `data/stories.json` directly at build time and types it as `site/src/types/story.ts`. That file uses `ts` and `cluster`, **not** `date` and `cluster_size` — `model.Story`'s JSON tags match the site, and `write/contract_test.go` pins it. A renamed tag breaks the site silently, not loudly.

## Conventions

- **Go**: No globals, no `init()`. Config passed explicitly. Errors: `fmt.Errorf("stage: %w", err)`. Logging: `log/slog` only. All external calls take `context.Context`.
- **Env only**: Copy `.env.example` to `.env`; never commit `.env`. CI uses GitHub Secrets.
- **Idempotency**: Second agent run creates zero duplicates (state tracks seen IDs per day).
- **Content limits**: Max 300 chars of source text stored. Max one quote per source, <15 words. Summaries are original LLM prose.
- **Zero client JS** except `/search` page.
- **Design tokens**: `site/src/styles/tokens.css` — reference everywhere, no inline values.
- **Dense layout**: ~25 stories per desktop screen on `/feed`.
- **Agent commits**: `content: YYYY-MM-DD — N stories` — only commits touching `data/` or `site/src/content/`.

## Gotchas

- **Never commit `data/cache/`** — embeddings are regenerable and huge.
- **Validation gate**: Failed validation aborts before commit (R9); never push a broken site. This includes orphaned `.md` pages that no story in `stories.json` points at.
- **LLM fallback chain**: NVIDIA NIM → OpenRouter. Every call has a timeout and falls through (R8). **No local models** and **no Gemini** — the pipeline's real home is CI, where no daemon is listening, and the project has no Google API access.
- **Rate limits**: GitHub token optional (raises 60→5000/hr). Reddit requires descriptive `User-Agent` or returns 429.
- **Embedder**: `AIRFOIL_EMBEDDER=nvidia_nim` — the only supported value. Model IDs in env. `similarity_threshold` in `scoring.json` is tuned per embedding model — re-tune if you change it.
- **Source types**: `RSS`, `HN`, `Reddit`, `HFPapers`, `GitHub` — enabled via `config/sources.json`.
- **Scoring weights**: `config/scoring.json` — `TierWeight(tier)` for Major/Notable/Minor.
- **Keywords**: `config/keywords.json` drives tag extraction.
