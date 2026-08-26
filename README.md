# AIRFOIL

<p align="center">
  <strong>High-velocity, low-drag AI intelligence pipeline.</strong>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat-square&logo=go" alt="Go Version" />
  <img src="https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react" alt="React Version" />
  <img src="https://img.shields.io/badge/Vite-6-646CFF?style=flat-square&logo=vite" alt="Vite" />
  <img src="https://img.shields.io/badge/License-License%20Zero%20NC%202.0.1-red?style=flat-square" alt="License" />
</p>

---

## Overview

**Airfoil** is an automated AI intelligence pipeline and high-density static publication. It cuts through the marketing fluff, vendor noise, and redundant hype across the artificial intelligence ecosystem by systematically ingesting primary sources, semantically clustering overlapping coverage, scoring for technical authority, and distilling technical breakthroughs into dense, zero-slop summaries.

There is **no database** and **no runtime server backend**. Git acts as the storage layer and state machine:

```
Ingest → Normalize → Embed → Cluster → Score → Summarize → Write .md → Commit → Push
```

---

## Key Features

- **Primary Source Ingestion**: Fetches from arXiv (`cs.AI`, `cs.CL`, `cs.LG`), Hacker News, HuggingFace Papers, Reddit (`r/LocalLLaMA`, `r/MachineLearning`), GitHub Trending, and top AI research lab feeds (Anthropic, OpenAI, DeepMind).
- **Semantic Vector Clustering**: Computes high-dimensional vector embeddings to group identical stories from multiple sources into unified payloads.
- **Authority & Recency Scoring**: Applies exponential half-life time decay, source-tier weighting (Tier 1 labs vs community forums), and builder signal detection while suppressing speculative hype.
- **Zero-Slop Synthesis**: Generates original, concise summaries strictly capped at 80 words with verifiable technical takeaways and tight excerpt boundaries (<300 chars, max one short quote).
- **Interactive Terminal UI (TUI)**: A feature-rich 7-tab Bubble Tea terminal interface to monitor pipeline telemetry, run doctor health-checks, inspect clusters, and adjust scoring parameters in real time.
- **High-Density Web Interface**: A sleek, dark-mode static frontend designed for engineers with ~25 stories per viewport, tag filtering, client-side search, and zero client JS footprint outside search.

---

## System Architecture

```
Airfoil Repository
├── agent/                  # Pipeline engine written in Go
│   ├── cmd/airfoil/        # Main binary entrypoint & CLI commands
│   └── internal/           # Core modules: cluster, embed, ingest, llm, score, tui, write
├── config/                 # Human-editable JSON configurations
│   ├── sources.json        # Source feeds, tiers, and query parameters
│   ├── scoring.json        # Weights, half-life decay, limits, and thresholds
│   └── keywords.json       # Structural and keyword builder signals
├── data/                   # Generated pipeline artifacts & Git storage layer
│   ├── items/              # Raw ingested item records (JSON)
│   ├── stories.json        # Published story database for the frontend
│   └── index.json          # Ranked pipeline index & state tracking
├── site/                   # High-performance React + TypeScript + Vite frontend
│   ├── src/content/        # Generated Markdown stories
│   └── src/pages/          # Feed, Top Stories, Search, Ship Log, and Digest views
└── docs/                   # System specifications, runbooks, and architectural docs
```

---

## Terminal UI (TUI)

Launch the interactive terminal console to operate the pipeline and inspect telemetry:

```bash
cd agent
go run ./cmd/airfoil
```

### Console Tabs:
1. **1 Dashboard**: Live pipeline funnel, source volume bars, readiness status, storage footprint, and topic breakdowns.
2. **2 Run**: Execute pipeline stages, full automated runs, and recurring 24h loop mode with dry-run support.
3. **3 Sources**: Toggle, edit, add, or delete ingestion sources.
4. **4 Scoring**: Tune similarity thresholds, tier weights, decay half-life, and LLM call caps with instant feedback.
5. **5 Keywords**: Manage builder signals and hype filter keywords.
6. **6 Providers**: Configure and persist API keys, models (NVIDIA NIM, OpenRouter), embedding endpoints, and ingest tokens directly to `.env`.
7. **7 Doctor**: Live connectivity probes for all upstream feeds and LLM provider endpoints.
8. **8 Browse**: Interactive inspection of raw ingested items, vector clusters, ranked outputs, and final stories.
9. **9 Digest**: Multi-format daily digest viewer (Newsletter, LinkedIn draft, X thread cards, HTML email) with instant copy-to-clipboard.
10. **0 Purge**: Interactive multi-select story curation and bulk deletion.

---

## Getting Started

### Prerequisites
- **Go 1.24+**
- **Node.js 20+** and **npm**
- **LLM / Embedding Provider**: NVIDIA NIM API key (preferred) and/or OpenRouter API key

### Installation

1. **Clone the repository**:
   ```bash
   git clone https://github.com/babtix/Airfoil.git
   cd Airfoil
   ```

2. **Configure environment variables**:
   ```bash
   cp .env.example .env
   # Edit .env with your NVIDIA NIM / OpenRouter API credentials
   ```

3. **Build the Go agent**:
   ```bash
   cd agent
   go build -o bin/airfoil ./cmd/airfoil
   ```

4. **Install web frontend dependencies**:
   ```bash
   cd ../site
   npm install
   ```

---

## CLI & Pipeline Commands

Run commands from the repository root:

| Command | Directory | Description |
|---|---|---|
| `./agent/bin/airfoil doctor` | Repo Root | Live probe of embedder, LLMs, and all ingestion sources |
| `./agent/bin/airfoil run --dry` | Repo Root | Execute full pipeline in dry-run mode (writes nothing to disk) |
| `./agent/bin/airfoil run` | Repo Root | Execute full pipeline (ingest, cluster, score, summarize, publish) |
| `npm run dev` | `site/` | Start local Vite development server |
| `npm run build` | `site/` | Build production static website bundle (`tsc -b && vite build`) |
| `npm run lint` | `site/` | Run Oxlint on frontend code |
| `go test ./...` | `agent/` | Run full Go automated test suite |

---

## License

This project is licensed under the **License Zero Noncommercial Public License 2.0.1** (L0-NC-2.0.1). See the [LICENSE](LICENSE) file for complete details.
