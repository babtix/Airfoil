# Configuration System

This chapter documents Airfoil's configuration system: the `config.Load` mechanism, environment variable precedence with `.env` file support, the three JSON configuration files (`sources.json`, `scoring.json`, `keywords.json`), derived filesystem paths, source enablement logic, and validation rules that gate the pipeline.

## Table of Contents

- [Overview](#overview)
- [Configuration Loading Flow](#configuration-loading-flow)
- [Environment Variable Precedence](#environment-variable-precedence)
- [Dotenv File Support](#dotenv-file-support)
- [JSON Configuration Files](#json-configuration-files)
  - [sources.json](#sourcesjson)
  - [scoring.json](#scoringjson)
  - [keywords.json](#keywordsjson)
- [Derived Filesystem Paths](#derived-filesystem-paths)
- [Source Enablement](#source-enablement)
- [Validation Rules](#validation-rules)
- [Configuration Types Reference](#configuration-types-reference)
- [Referenced Files](#referenced-files)

---

## Overview

Airfoil's configuration is **explicit, file-backed, and validated at startup**. There are no globals, no `init()` functions, and no implicit environment reads outside `config.Load`. The `Config` struct is constructed once in `cmd/airfoil` and passed down through the pipeline.

The loading order is:

1. **CLI flags** (highest precedence) — `Options.ConfigDir`, `Options.DataDir`, `Options.Verbose`
2. **Process environment** — `AIRFOIL_*`, provider keys, `SITE_URL`, etc.
3. **`.env` file** — loaded only for variables not already set in the environment
4. **Hard-coded defaults** (lowest precedence)

All three JSON files in `config/` are parsed into typed structs and validated before the pipeline runs. A configuration error aborts the run immediately — no partial execution.

---

## Configuration Loading Flow

```mermaid
flowchart TD
    A[cmd/airfoil: root.go] --> B[config.Load(Options)]
    B --> C[loadDotEnv(".env")]
    C --> D[Build Config struct from env + defaults]
    D --> E[loadFiles: sources.json, scoring.json, keywords.json]
    E --> F[Config.Validate()]
    F --> G{Valid?}
    G -->|No| H[Return error, abort]
    G -->|Yes| I[Return *Config]
    I --> J[Pass to ingest.New, pipeline stages]
```

The `Load` function is the single entry point:

`agent/internal/config/config.go:115-175`

```go
// Load resolves configuration from .env, the process environment, and the JSON
// files in the config directory. Precedence: CLI flags > environment > default.
//
// A config file that does not parse or does not validate is a hard error here,
// at startup, rather than a surprise halfway through a run.
func Load(opts Options) (*Config, error) {
	// A .env file is convenience for local runs; CI supplies real env vars.
	// Values already present in the environment always win.
	if err := loadDotEnv(".env"); err != nil {
		return nil, err
	}

	cfg := &Config{
		DataDir:   firstNonEmpty(opts.DataDir, os.Getenv("AIRFOIL_DATA_DIR"), "./data"),
		ConfigDir: firstNonEmpty(opts.ConfigDir, os.Getenv("AIRFOIL_CONFIG_DIR"), "./config"),
		LogLevel:  firstNonEmpty(os.Getenv("AIRFOIL_LOG_LEVEL"), "info"),
		SiteURL:   os.Getenv("SITE_URL"),

		LLM: LLMConfig{
			Gemini: ProviderCreds{
				APIKey: os.Getenv("GEMINI_API_KEY"),
				Model:  firstNonEmpty(os.Getenv("GEMINI_MODEL"), "gemini-2.0-flash"),
			},
			NvidiaNIM: ProviderCreds{
				APIKey: os.Getenv("NVIDIA_NIM_API_KEY"),
				Model:  firstNonEmpty(os.Getenv("NVIDIA_NIM_MODEL"), "meta/llama-3.3-70b-instruct"),
			},
			OpenRouter: ProviderCreds{
				APIKey: os.Getenv("OPENROUTER_API_KEY"),
				Model:  firstNonEmpty(os.Getenv("OPENROUTER_MODEL"), "meta-llama/llama-3.3-70b-instruct:free"),
			},
			Ollama: OllamaConfig{
				Host:       firstNonEmpty(os.Getenv("OLLAMA_HOST"), "http://localhost:11434"),
				ChatModel:  firstNonEmpty(os.Getenv("OLLAMA_CHAT_MODEL"), "llama3.1"),
				EmbedModel: firstNonEmpty(os.Getenv("OLLAMA_EMBED_MODEL"), "nomic-embed-text"),
			},
		},

		Embed: EmbedConfig{
			Provider:    firstNonEmpty(os.Getenv("AIRFOIL_EMBEDDER"), EmbedderOllama),
			OllamaHost:  firstNonEmpty(os.Getenv("OLLAMA_HOST"), "http://localhost:11434"),
			OllamaModel: firstNonEmpty(os.Getenv("OLLAMA_EMBED_MODEL"), "nomic-embed-text"),
			GeminiKey:   os.Getenv("GEMINI_API_KEY"),
			GeminiModel: firstNonEmpty(os.Getenv("GEMINI_EMBED_MODEL"), "text-embedding-004"),
			NIMKey:      os.Getenv("NVIDIA_NIM_API_KEY"),
			NIMModel:    firstNonEmpty(os.Getenv("NVIDIA_NIM_EMBED_MODEL"), "nvidia/nv-embedqa-e5-v5"),
			NIMBaseURL:  firstNonEmpty(os.Getenv("NVIDIA_NIM_BASE_URL"), "https://integrate.api.nvidia.com/v1"),
		},

		Ingest: IngestConfig{
			GitHubToken:     os.Getenv("GITHUB_TOKEN"),
			RedditUserAgent: firstNonEmpty(os.Getenv("REDDIT_USER_AGENT"), "airfoil/0.1"),
		},
	}

	// The markdown collection lives in the site, not in data/.
	cfg.StoriesDir = firstNonEmpty(
		os.Getenv("AIRFOIL_STORIES_DIR"),
		filepath.Join("site", "src", "content", "stories"),
	)

	if err := cfg.loadFiles(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}
```

The helper `firstNonEmpty` implements the precedence chain:

`agent/internal/config/config.go:246-253`

```go
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
```

---

## Environment Variable Precedence

The following table lists every environment variable recognized by `config.Load`, its purpose, and its default (if any). **CLI flags > Environment > .env > Default**.

| Environment Variable | Config Field | Purpose | Default |
|---------------------|--------------|---------|---------|
| `AIRFOIL_CONFIG_DIR` | `Config.ConfigDir` | Directory containing `sources.json`, `scoring.json`, `keywords.json` | `./config` |
| `AIRFOIL_DATA_DIR` | `Config.DataDir` | Root for generated data (`items/`, `cache/`, `index.json`, `state.json`, `stories.json`) | `./data` |
| `AIRFOIL_LOG_LEVEL` | `Config.LogLevel` | `slog` level: `debug`, `info`, `warn`, `error` | `info` |
| `AIRFOIL_EMBEDDER` | `Config.Embed.Provider` | Embedding provider: `ollama`, `gemini`, `nvidia_nim` | `ollama` |
| `AIRFOIL_STORIES_DIR` | `Config.StoriesDir` | Where Markdown story files are written | `site/src/content/stories` |
| `SITE_URL` | `Config.SiteURL` | Base URL for generated site (used in RSS, sitemap) | *(empty)* |
| `GEMINI_API_KEY` | `Config.LLM.Gemini.APIKey` / `Config.Embed.GeminiKey` | Google Gemini API key for LLM + embeddings | *(empty)* |
| `GEMINI_MODEL` | `Config.LLM.Gemini.Model` | Gemini chat model ID | `gemini-2.0-flash` |
| `GEMINI_EMBED_MODEL` | `Config.Embed.GeminiModel` | Gemini embedding model ID | `text-embedding-004` |
| `NVIDIA_NIM_API_KEY` | `Config.LLM.NvidiaNIM.APIKey` / `Config.Embed.NIMKey` | NVIDIA NIM API key | *(empty)* |
| `NVIDIA_NIM_MODEL` | `Config.LLM.NvidiaNIM.Model` | NIM chat model ID | `meta/llama-3.3-70b-instruct` |
| `NVIDIA_NIM_EMBED_MODEL` | `Config.Embed.NIMModel` | NIM embedding model ID | `nvidia/nv-embedqa-e5-v5` |
| `NVIDIA_NIM_BASE_URL` | `Config.Embed.NIMBaseURL` | NIM API base URL | `https://integrate.api.nvidia.com/v1` |
| `OPENROUTER_API_KEY` | `Config.LLM.OpenRouter.APIKey` | OpenRouter API key | *(empty)* |
| `OPENROUTER_MODEL` | `Config.LLM.OpenRouter.Model` | OpenRouter chat model ID | `meta-llama/llama-3.3-70b-instruct:free` |
| `OLLAMA_HOST` | `Config.LLM.Ollama.Host` / `Config.Embed.OllamaHost` | Ollama daemon URL | `http://localhost:11434` |
| `OLLAMA_CHAT_MODEL` | `Config.LLM.Ollama.ChatModel` | Ollama chat model | `llama3.1` |
| `OLLAMA_EMBED_MODEL` | `Config.LLM.Ollama.EmbedModel` / `Config.Embed.OllamaModel` | Ollama embedding model | `nomic-embed-text` |
| `GITHUB_TOKEN` | `Config.Ingest.GitHubToken` | GitHub API token (raises rate limit 60→5000/hr) | *(empty)* |
| `REDDIT_USER_AGENT` | `Config.Ingest.RedditUserAgent` | Reddit API User-Agent (required, else 429) | `airfoil/0.1` |

> **Note**: `GEMINI_API_KEY` and `NVIDIA_NIM_API_KEY` are used for both LLM summarization and embeddings when their respective providers are selected.

---

## Dotenv File Support

A `.env` file in the working directory is loaded **only for variables not already set** in the process environment. This allows local development without polluting CI secrets.

`agent/internal/config/dotenv.go:21-51`

```go
// loadDotEnv reads a .env file into the process environment.
//
// Variables already set in the environment are never overwritten, so CI secrets
// always win over a stale local file. A missing .env is not an error — that is
// the normal case in CI.
//
// This is deliberately a small parser rather than a dependency: it handles
// KEY=value, optional `export` prefix, # comments, and single or double quoted
// values. It does not do interpolation.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("config: open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for line := 1; sc.Scan(); line++ {
		key, val, ok := parseDotEnvLine(sc.Text())
		if !ok {
			continue
		}
		if key == "" {
			return fmt.Errorf("config: %s:%d: malformed line", path, line)
		}
		if _, set := os.LookupEnv(key); set {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			return fmt.Errorf("config: set %s: %w", key, err)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("config: read %s: %w", path, err)
	}
	return nil
}
```

Line parsing rules (`parseDotEnvLine`):

- Skips blank lines and lines starting with `#`
- Strips optional `export ` prefix
- Splits on first `=`
- Trims whitespace from key and value
- Strips matching single or double quotes from value
- Removes trailing ` #` comment from unquoted values
- Returns `ok=false` for comments/blanks, `ok=true` with empty key for malformed lines

`agent/internal/config/dotenv.go:55-84`

```go
func parseDotEnvLine(raw string) (key, val string, ok bool) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")

	name, value, found := strings.Cut(line, "=")
	if !found {
		return "", "", true // malformed
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", true
	}

	value = strings.TrimSpace(value)
	switch {
	case len(value) >= 2 && strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`):
		value = value[1 : len(value)-1]
	case len(value) >= 2 && strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`):
		value = value[1 : len(value)-1]
	default:
		// An unquoted trailing comment is not part of the value.
		if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
	}
	return name, value, true
}
```

---

## JSON Configuration Files

All three files live in `ConfigDir` (default `./config/`) and are loaded by `Config.loadFiles()`:

`agent/internal/config/config.go:178-190`

```go
// loadFiles reads sources.json, scoring.json, and keywords.json.
func (c *Config) loadFiles() error {
	var err error
	if c.Sources, err = loadSources(filepath.Join(c.ConfigDir, "sources.json")); err != nil {
		return err
	}
	if c.Scoring, err = loadScoring(filepath.Join(c.ConfigDir, "scoring.json")); err != nil {
		return err
	}
	if c.Keywords, err = loadKeywords(filepath.Join(c.ConfigDir, "keywords.json")); err != nil {
		return err
	}
	return nil
}
```

Each uses `internal/store.ReadJSON[T]` for atomic, typed reads.

### sources.json

Defines all ingest sources. Each source has an ID, name, type, URL, tier (1–5), tags, enabled flag, and type-specific options.

**Source types** (validated against `validSourceTypes`):

| Constant | Value | Adapter |
|----------|-------|---------|
| `SourceRSS` | `"rss"` | `rssAdapter` |
| `SourceHN` | `"hn"` | `hnAdapter` |
| `SourceReddit` | `"reddit"` | `rssAdapter` (Reddit JSON feed) |
| `SourceHFPapers` | `"hf_papers"` | `rssAdapter` (HF Daily Papers API) |
| `SourceGitHub` | `"github"` | `rssAdapter` (GitHub Search API) |

`agent/internal/config/files.go:11-24`

```go
// Source types recognised by the ingest layer.
const (
	SourceRSS      = "rss"
	SourceHN       = "hn"
	SourceReddit   = "reddit"
	SourceHFPapers = "hf_papers"
	SourceGitHub   = "github"
)

var validSourceTypes = map[string]bool{
	SourceRSS:      true,
	SourceHN:       true,
	SourceReddit:   true,
	SourceHFPapers: true,
	SourceGitHub:   true,
}
```

**Source struct**:

`agent/internal/config/files.go:27-36`

```go
// Source is one entry in config/sources.json.
type Source struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Type    string        `json:"type"`
	URL     string        `json:"url"`
	Tier    int           `json:"tier"`
	Tags    []string      `json:"tags"`
	Enabled bool          `json:"enabled"`
	Options SourceOptions `json:"options"`
}
```

**SourceOptions** — union of per-adapter fields; each adapter reads only what it needs:

`agent/internal/config/files.go:40-57`

```go
// SourceOptions is the union of per-adapter options. Each adapter reads only
// the fields it cares about; the rest stay zero.
type SourceOptions struct {
	// hn
	Queries   []string `json:"queries"`
	MinPoints int      `json:"min_points"`
	HoursBack int      `json:"hours_back"`

	// reddit
	Limit    int `json:"limit"`
	MinScore int `json:"min_score"`

	// hf_papers
	MinUpvotes int `json:"min_upvotes"`

	// github
	CreatedWithinDays int `json:"created_within_days"`
	MinStars          int `json:"min_stars"`
	MaxPerQuery       int `json:"max_per_query"`
}
```

**Example from `config/sources.json`** (truncated):

```json
{
  "sources": [
    {
      "id": "openai",
      "name": "OpenAI",
      "type": "rss",
      "url": "https://openai.com/news/rss.xml",
      "tier": 1,
      "tags": ["labs"],
      "enabled": true
    },
    {
      "id": "hn",
      "name": "Hacker News",
      "type": "hn",
      "url": "https://hn.algolia.com/api/v1/search_by_date",
      "tier": 5,
      "tags": ["community"],
      "enabled": true,
      "options": {
        "queries": ["AI", "LLM", "GPT", "Claude", "Gemini", "open weights", "inference"],
        "min_points": 50,
        "hours_back": 48
      }
    },
    {
      "id": "github-trending-ai",
      "name": "GitHub Trending",
      "type": "github",
      "url": "https://api.github.com/search/repositories",
      "tier": 3,
      "tags": ["tools", "repos"],
      "enabled": true,
      "options": {
        "queries": ["llm", "ai-agent", "rag", "inference", "fine-tuning"],
        "created_within_days": 7,
        "min_stars": 100,
        "max_per_query": 5
      }
    }
  ]
}
```

**Tier semantics** (from `config/sources.json` comment):

| Tier | Meaning |
|------|---------|
| 1 | Official lab (OpenAI, Google, DeepMind, Meta, Mistral) |
| 2 | Research (HF Blog, arXiv) |
| 3 | Dev/tools (GitHub Trending) |
| 4 | Press (TechCrunch, VentureBeat, The Verge) |
| 5 | Community (HN, Reddit) |

---

### scoring.json

All ranking knobs live here so they can be tuned without rebuilding. The `Scoring` struct is loaded directly from this file.

`agent/internal/config/files.go:73-122`

```go
// Scoring is config/scoring.json. Every knob the ranking depends on lives here
// so it can be tuned without a rebuild.
type Scoring struct {
	Clustering struct {
		SimilarityThreshold float64    `json:"similarity_threshold"`
		WindowHours         int        `json:"window_hours"`
		DebugRange          [2]float64 `json:"debug_range"`
	} `json:"clustering"`

	Weights struct {
		Sources float64 `json:"sources"`
		Tier    float64 `json:"tier"`
		HN      float64 `json:"hn"`
		Reddit  float64 `json:"reddit"`
		Builder float64 `json:"builder"`
		Hype    float64 `json:"hype"`
	} `json:"weights"`

	TierWeights map[string]float64 `json:"tier_weights"`

	Recency struct {
		HalfLifeHours float64 `json:"half_life_hours"`
	} `json:"recency"`

	Caps struct {
		BuilderSignalMax int `json:"builder_signal_max"`
		HypeSignalMax    int `json:"hype_signal_max"`
		LLMCallsPerRun   int `json:"llm_calls_per_run"`
	} `json:"caps"`

	Tiers struct {
		MajorMinScore   int `json:"major_min_score"`
		NotableMinScore int `json:"notable_min_score"`
	} `json:"tiers"`

	BuilderRelevantMinSignals int `json:"builder_relevant_min_signals"`

	Limits struct {
		ExcerptMaxChars         int `json:"excerpt_max_chars"`
		SummaryMaxWords         int `json:"summary_max_words"`
		TakeawaysMax            int `json:"takeaways_max"`
		VerbatimOverlapMaxWords int `json:"verbatim_overlap_max_words"`
		QuotesMax               int `json:"quotes_max"`
		QuoteMaxWords           int `json:"quote_max_words"`
	} `json:"limits"`

	Retention struct {
		ItemsDays          int `json:"items_days"`
		SeenURLsDays       int `json:"seen_urls_days"`
		EmbeddingCacheDays int `json:"embedding_cache_days"`
	} `json:"retention"`
}
```

**TierWeight helper** — returns weight for a source tier (1–5), or 0 if missing:

`agent/internal/config/files.go:126-128`

```go
func (s Scoring) TierWeight(tier int) float64 {
	return s.TierWeights[fmt.Sprint(tier)]
}
```

**Example from `config/scoring.json`**:

```json
{
  "clustering": {
    "similarity_threshold": 0.82,
    "window_hours": 48,
    "debug_range": [0.75, 0.90]
  },
  "weights": {
    "sources": 22.0,
    "tier": 25.0,
    "hn": 6.0,
    "reddit": 5.0,
    "builder": 4.0,
    "hype": 6.0
  },
  "tier_weights": {
    "1": 1.00,
    "2": 0.85,
    "3": 0.80,
    "4": 0.50,
    "5": 0.30
  },
  "recency": { "half_life_hours": 48 },
  "caps": {
    "builder_signal_max": 5,
    "hype_signal_max": 5,
    "llm_calls_per_run": 15
  },
  "tiers": {
    "major_min_score": 70,
    "notable_min_score": 40
  },
  "builder_relevant_min_signals": 2,
  "limits": {
    "excerpt_max_chars": 300,
    "summary_max_words": 80,
    "takeaways_max": 3,
    "verbatim_overlap_max_words": 12,
    "quotes_max": 1,
    "quote_max_words": 15
  },
  "retention": {
    "items_days": 30,
    "seen_urls_days": 30,
    "embedding_cache_days": 7
  }
}
```

---

### keywords.json

Keyword lists for builder/hype signal detection and topic tagging. Matched case-insensitively against title + excerpt.

`agent/internal/config/files.go:198-211`

```go
// Keywords is config/keywords.json. Matched case-insensitively against
// title + excerpt.
type Keywords struct {
	BuilderSignals           []string            `json:"builder_signals"`
	BuilderStructuralSignals []string            `json:"builder_structural_signals"`
	HypeSignals              []string            `json:"hype_signals"`
	TopicTags                map[string][]string `json:"topic_tags"`
}
```

**Example from `config/keywords.json`**:

```json
{
  "builder_signals": [
    "release", "released", "launches", "now available", "general availability",
    "open source", "open-source", "open weights", "open-weight",
    "api", "sdk", "cli", "endpoint", "self-host", "self-hosted",
    "benchmark", "benchmarks", "evals", "leaderboard",
    "deprecated", "deprecation", "breaking change", "migration guide",
    "pricing", "price cut", "cheaper", "per million tokens",
    "context window", "context length",
    "quantized", "gguf", "fine-tune", "fine-tuning", "lora",
    "inference", "throughput", "latency",
    "model card", "weights", "checkpoint",
    "docs", "documentation", "changelog"
  ],
  "builder_structural_signals": [
    "has_repo_url",
    "has_paper_url",
    "source_tier_1",
    "source_tier_3"
  ],
  "hype_signals": [
    "shocking", "shocked", "you won't believe", "you wont believe",
    "game-changer", "game changer", "changes everything",
    "will replace all", "replace all human", "end of programming",
    "the end of", "nobody is talking about", "no one is talking about",
    "insane", "mind-blowing", "mind blowing", "jaw-dropping",
    "this is huge", "terrifying", "we're doomed", "we are doomed",
    "secretly", "they don't want you to know", "hidden feature nobody"
  ],
  "topic_tags": {
    "models": ["model", "gpt", "claude", "gemini", "llama", "mistral", "qwen", "deepseek"],
    "agents": ["agent", "agentic", "tool use", "mcp", "function calling"],
    "rag": ["rag", "retrieval", "vector", "embedding", "reranker"],
    "infra": ["inference", "serving", "vllm", "gpu", "cuda", "tpu", "datacenter"],
    "open-source": ["open source", "open-source", "open weights", "apache 2.0", "mit license"],
    "research": ["paper", "arxiv", "we propose", "state of the art", "sota"],
    "policy": ["regulation", "eu ai act", "executive order", "compliance", "export control"],
    "business": ["funding", "raises", "valuation", "acquisition", "revenue", "ipo"],
    "safety": ["alignment", "red team", "jailbreak", "interpretability", "safety"],
    "coding": ["copilot", "code", "ide", "developer tool", "programming"]
  }
}
```

---

## Derived Filesystem Paths

`Config` provides methods that derive all data paths from `DataDir` and `StoriesDir`. **Never hardcode these paths elsewhere.**

`agent/internal/config/config.go:81-89`

```go
// Paths to the generated data files.
func (c Config) ItemsDir() string    { return filepath.Join(c.DataDir, "items") }
func (c Config) CacheDir() string    { return filepath.Join(c.DataDir, "cache") }
func (c Config) DigestDir() string   { return filepath.Join(c.DataDir, "digest") }
func (c Config) IndexPath() string   { return filepath.Join(c.DataDir, "index.json") }
func (c Config) StatePath() string   { return filepath.Join(c.DataDir, "state.json") }
func (c Config) StoriesPath() string { return filepath.Join(c.DataDir, "stories.json") }
func (c Config) EmbedCachePath() string {
	return filepath.Join(c.CacheDir(), "embeddings.json")
}
```

| Method | Path | Purpose |
|--------|------|---------|
| `ItemsDir()` | `data/items/` | Per-item JSON files (one per ingested item) |
| `CacheDir()` | `data/cache/` | Embedding cache, **never committed** |
| `DigestDir()` | `data/digest/` | Digest output (future use) |
| `IndexPath()` | `data/index.json` | Ranked `IndexEntry` list for site consumption |
| `StatePath()` | `data/state.json` | Dedup tracker (`State.Seen/MarkSeen`) |
| `StoriesPath()` | `data/stories.json` | Serialized `Story` list (mirrors Markdown) |
| `EmbedCachePath()` | `data/cache/embeddings.json` | Embedding vectors keyed by model + content hash |
| `StoriesDir` | `site/src/content/stories/` | Markdown story files (frontmatter + body) |

---

## Source Enablement

`Config.EnabledSources()` filters the loaded sources to only those with `Enabled: true`. This is the list passed to the ingest runner.

`agent/internal/config/config.go:92-100`

```go
// EnabledSources returns only the sources marked enabled in sources.json.
func (c Config) EnabledSources() []Source {
	out := make([]Source, 0, len(c.Sources))
	for _, s := range c.Sources {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out
}
```

**Validation** requires at least one enabled source (see [Validation Rules](#validation-rules)).

---

## Validation Rules

`Config.Validate()` runs after loading all files. It collects **all** problems and returns a single error with a formatted list. The pipeline aborts on any validation failure.

`agent/internal/config/config.go:194-244`

```go
// Validate checks the loaded configuration for values that would produce
// silently wrong output.
func (c *Config) Validate() error {
	var problems []string

	if len(c.EnabledSources()) == 0 {
		problems = append(problems, "no enabled sources in sources.json")
	}
	seen := map[string]bool{}
	for i, s := range c.Sources {
		where := fmt.Sprintf("sources[%d]", i)
		if s.ID == "" {
			problems = append(problems, where+": missing id")
		} else if seen[s.ID] {
			problems = append(problems, where+": duplicate id "+s.ID)
		}
		seen[s.ID] = true

		if s.Name == "" {
			problems = append(problems, where+" ("+s.ID+"): missing name")
		}
		if s.URL == "" {
			problems = append(problems, where+" ("+s.ID+"): missing url")
		}
		if !validSourceTypes[s.Type] {
			problems = append(problems, fmt.Sprintf("%s (%s): unknown type %q", where, s.ID, s.Type))
		}
		if s.Tier < 1 || s.Tier > 5 {
			problems = append(problems, fmt.Sprintf("%s (%s): tier %d out of range 1-5", where, s.ID, s.Tier))
		}
	}

	problems = append(problems, c.Scoring.problems()...)

	if len(c.Keywords.BuilderSignals) == 0 {
		problems = append(problems, "keywords.json: builder_signals is empty")
	}
	if len(c.Keywords.HypeSignals) == 0 {
		problems = append(problems, "keywords.json: hype_signals is empty")
	}

	switch c.Embed.Provider {
	case EmbedderOllama, EmbedderGemini, EmbedderNvidiaNIM:
	default:
		problems = append(problems, fmt.Sprintf("AIRFOIL_EMBEDDER=%q: want one of %q, %q, %q",
			c.Embed.Provider, EmbedderOllama, EmbedderGemini, EmbedderNvidiaNIM))
	}

	if len(problems) > 0 {
		return fmt.Errorf("config: invalid:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}
```

### Source Validation Rules

| Check | Error Message |
|-------|---------------|
| At least one enabled source | `no enabled sources in sources.json` |
| `ID` non-empty | `sources[i]: missing id` |
| `ID` unique | `sources[i]: duplicate id <id>` |
| `Name` non-empty | `sources[i] (<id>): missing name` |
| `URL` non-empty | `sources[i] (<id>): missing url` |
| `Type` in `validSourceTypes` | `sources[i] (<id>): unknown type "..."` |
| `Tier` in 1–5 | `sources[i] (<id>): tier N out of range 1-5` |

### Scoring Validation Rules (`Scoring.problems()`)

`agent/internal/config/files.go:139-194`

```go
func (s Scoring) problems() []string {
	var out []string
	add := func(format string, args ...any) {
		out = append(out, "scoring.json: "+fmt.Sprintf(format, args...))
	}

	if t := s.Clustering.SimilarityThreshold; t <= 0 || t > 1 {
		add("clustering.similarity_threshold %v must be in (0, 1]", t)
	}
	if s.Clustering.WindowHours <= 0 {
		add("clustering.window_hours must be > 0")
	}
	if s.Recency.HalfLifeHours <= 0 {
		add("recency.half_life_hours must be > 0")
	}

	for tier := 1; tier <= 5; tier++ {
		if _, ok := s.TierWeights[fmt.Sprint(tier)]; !ok {
			add("tier_weights is missing tier %d", tier)
		}
	}

	if s.Tiers.MajorMinScore <= s.Tiers.NotableMinScore {
		add("tiers.major_min_score (%d) must exceed tiers.notable_min_score (%d)",
			s.Tiers.MajorMinScore, s.Tiers.NotableMinScore)
	}
	if s.Caps.LLMCallsPerRun <= 0 {
		add("caps.llm_calls_per_run must be > 0")
	}

	// R1 and R2 depend on these limits being real numbers.
	if s.Limits.ExcerptMaxChars <= 0

<!-- kaioken:files agent/internal/config/config.go,agent/internal/config/files.go,agent/internal/config/dotenv.go,config/sources.json,config/scoring.json,config/keywords.json,agent/internal/config/config_test.go -->
