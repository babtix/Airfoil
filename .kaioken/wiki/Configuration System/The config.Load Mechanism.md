# The config.Load Mechanism

This chapter details the `config.Load` function and its supporting machinery: how CLI flags, environment variables, `.env` files, and JSON configuration files are resolved into a single `*Config` struct; how derived paths are computed; how sources are filtered; and how validation gates the pipeline at startup.

## Table of Contents

- [Overview](#overview)
- [Precedence Model](#precedence-model)
- [Dotenv Loading](#dotenv-loading)
- [Configuration Structs](#configuration-structs)
- [Load Function Walkthrough](#load-function-walkthrough)
- [Derived Path Computation](#derived-path-computation)
- [JSON File Loading](#json-file-loading)
- [Source Enabling and Filtering](#source-enabling-and-filtering)
- [Validation Rules](#validation-rules)
- [Error Handling](#error-handling)
- [Referenced Files](#referenced-files)

---

## Overview

The `config.Load` function is the single entry point for configuration in the Airfoil agent. It produces a fully resolved `*Config` struct that is passed explicitly through the entire pipeline — no globals, no `init()`, no ambient `os.Getenv` calls elsewhere.

**Responsibilities:**
1. Load `.env` file (local convenience only; never overwrites existing environment)
2. Resolve all scalar settings with precedence: **CLI flags > environment > defaults**
3. Load and parse three JSON files from the config directory: `sources.json`, `scoring.json`, `keywords.json`
4. Compute derived filesystem paths (`ItemsDir`, `CacheDir`, `IndexPath`, etc.)
5. Validate the complete configuration; fail fast on any problem

**Non-goals:** No interpolation in `.env`, no remote config fetching, no hot reload.

---

## Precedence Model

The resolution order is fixed and documented in the `Load` function:

```
CLI flag (Options field)  →  Environment variable  →  Hard-coded default
```

| Setting | CLI Flag (`Options`) | Environment Variable | Default |
|---------|---------------------|---------------------|---------|
| Data directory | `opts.DataDir` | `AIRFOIL_DATA_DIR` | `./data` |
| Config directory | `opts.ConfigDir` | `AIRFOIL_CONFIG_DIR` | `./config` |
| Log level | — | `AIRFOIL_LOG_LEVEL` | `info` |
| Site URL | — | `SITE_URL` | (empty) |
| Stories directory | — | `AIRFOIL_STORIES_DIR` | `site/src/content/stories` |
| Embedder provider | — | `AIRFOIL_EMBEDDER` | `ollama` |
| Ollama host | — | `OLLAMA_HOST` | `http://localhost:11434` |
| Ollama chat model | — | `OLLAMA_CHAT_MODEL` | `llama3.1` |
| Ollama embed model | — | `OLLAMA_EMBED_MODEL` | `nomic-embed-text` |
| Gemini API key | — | `GEMINI_API_KEY` | (empty) |
| Gemini chat model | — | `GEMINI_MODEL` | `gemini-2.0-flash` |
| Gemini embed model | — | `GEMINI_EMBED_MODEL` | `text-embedding-004` |
| NVIDIA NIM API key | — | `NVIDIA_NIM_API_KEY` | (empty) |
| NVIDIA NIM chat model | — | `NVIDIA_NIM_MODEL` | `meta/llama-3.3-70b-instruct` |
| NVIDIA NIM embed model | — | `NVIDIA_NIM_EMBED_MODEL` | `nvidia/nv-embedqa-e5-v5` |
| NVIDIA NIM base URL | — | `NVIDIA_NIM_BASE_URL` | `https://integrate.api.nvidia.com/v1` |
| OpenRouter API key | — | `OPENROUTER_API_KEY` | (empty) |
| OpenRouter model | — | `OPENROUTER_MODEL` | `meta-llama/llama-3.3-70b-instruct:free` |
| GitHub token | — | `GITHUB_TOKEN` | (empty) |
| Reddit User-Agent | — | `REDDIT_USER_AGENT` | `airfoil/0.1` |

> **Note:** The `Options` struct only exposes `ConfigDir`, `DataDir`, and `Verbose`. All other settings are environment-only.

---

## Dotenv Loading

The `.env` file is a local-development convenience. It is loaded **before** any environment variables are read, but **never overwrites** variables already present in the process environment.

```
agent/internal/config/dotenv.go:21-51
```

```go
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

### Parsing Rules (`parseDotEnvLine`)

```
agent/internal/config/dotenv.go:55-84
```

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

**Supported syntax:**
- `KEY=value`
- `export KEY=value`
- `KEY="quoted value"` or `KEY='quoted value'`
- `# comment` lines (ignored)
- Inline trailing comments: `KEY=value # comment` (only for unquoted values)

**Not supported:** Variable interpolation (`${VAR}`), multiline values, escaping inside quotes.

**Error behavior:**
- Missing `.env` → **not an error** (normal in CI)
- Malformed line (no `=`, empty key) → returns error with file:line
- `os.Setenv` failure → wrapped error

---

## Configuration Structs

### Top-Level `Config`

```
agent/internal/config/config.go:14-30
```

```go
type Config struct {
	// Paths
	DataDir    string
	ConfigDir  string
	StoriesDir string // where the markdown files are written

	LogLevel string
	SiteURL  string

	LLM    LLMConfig
	Embed  EmbedConfig
	Ingest IngestConfig

	Sources  []Source
	Scoring  Scoring
	Keywords Keywords
}
```

### LLM Provider Credentials

```
agent/internal/config/config.go:34-50
```

```go
type LLMConfig struct {
	Gemini     ProviderCreds
	NvidiaNIM  ProviderCreds
	OpenRouter ProviderCreds
	Ollama     OllamaConfig
}

type ProviderCreds struct {
	APIKey string
	Model  string
}

func (p ProviderCreds) Configured() bool {
	return p.APIKey != "" && p.Model != ""
}

type OllamaConfig struct {
	Host       string
	ChatModel  string
	EmbedModel string
}
```

`ProviderCreds.Configured()` is used by the summarization fallback chain to decide whether a provider is attemptable.

### Embedding Configuration

```
agent/internal/config/config.go:61-72
```

```go
const (
	EmbedderOllama    = "ollama"
	EmbedderGemini    = "gemini"
	EmbedderNvidiaNIM = "nvidia_nim"
)

type EmbedConfig struct {
	Provider    string // ollama (local) | gemini | nvidia_nim
	OllamaHost  string
	OllamaModel string
	GeminiKey   string
	GeminiModel string
	NIMKey      string
	NIMModel    string
	NIMBaseURL  string
}

func (e EmbedConfig) Model() string {
	switch e.Provider {
	case EmbedderGemini:
		return e.GeminiModel
	case EmbedderNvidiaNIM:
		return e.NIMModel
	default:
		return e.OllamaModel
	}
}
```

The `Model()` method returns the model ID for the currently selected provider. The embedding cache (`data/cache/embeddings.json`) is keyed by model to prevent mixing vectors from different providers.

### Ingest Credentials

```
agent/internal/config/config.go:75-78
```

```go
type IngestConfig struct {
	GitHubToken     string
	RedditUserAgent string
}
```

---

## Load Function Walkthrough

```
agent/internal/config/config.go:115-175
```

```go
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

### Flow Diagram

```mermaid
flowchart TD
    A[Load(Options)] --> B[loadDotEnv(".env")]
    B --> C{Error?}
    C -->|Yes| D[Return error]
    C -->|No| E[Build Config struct with precedence resolution]
    E --> F[Resolve StoriesDir]
    F --> G[cfg.loadFiles()]
    G --> H{Error?}
    H -->|Yes| D
    H -->|No| I[cfg.Validate()]
    I --> J{Error?}
    J -->|Yes| D
    J -->|No| K[Return *Config]
```

### Helper: `firstNonEmpty`

```
agent/internal/config/config.go:246-253
```

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

This small utility implements the precedence chain. It returns the first non-empty string, or empty string if all are empty.

---

## Derived Path Computation

All derived paths are computed as methods on `Config` using `filepath.Join` for platform correctness. They are **not stored** in the struct; they are computed on demand.

```
agent/internal/config/config.go:81-89
```

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

### Path Summary Table

| Method | Path | Purpose |
|--------|------|---------|
| `ItemsDir()` | `<DataDir>/items` | Per-item JSON files (one per ingested item) |
| `CacheDir()` | `<DataDir>/cache` | Embedding cache directory (never committed) |
| `DigestDir()` | `<DataDir>/digest` | Digest output directory |
| `IndexPath()` | `<DataDir>/index.json` | Ranked story index for site consumption |
| `StatePath()` | `<DataDir>/state.json` | Deduplication state (seen item IDs per day) |
| `StoriesPath()` | `<DataDir>/stories.json` | Serialized story metadata for site |
| `EmbedCachePath()` | `<DataDir>/cache/embeddings.json` | Embedding vectors keyed by model + content hash |
| `StoriesDir` (field) | `AIRFOIL_STORIES_DIR` or `site/src/content/stories` | Markdown story files (committed to repo) |

> **Important:** `StoriesDir` is a **field** (set during `Load`), not a method, because it lives outside `DataDir` (in the site repo). The `data/` directory is gitignored except for `index.json`, `state.json`, `stories.json`; `data/cache/` is never committed.

---

## JSON File Loading

`loadFiles` reads the three JSON configuration files from `ConfigDir`.

```
agent/internal/config/config.go:178-190
```

```go
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

The actual parsing functions (`loadSources`, `loadScoring`, `loadKeywords`) are defined in separate files (`sources.go`, `scoring.go`, `keywords.go`) not shown in the provided source but referenced in the structure. Their signatures are:

```go
func loadSources(path string) ([]Source, error)
func loadScoring(path string) (Scoring, error)
func loadKeywords(path string) (Keywords, error)
```

Each returns a typed struct (not `interface{}`) so validation can operate on concrete types.

### Expected JSON Schemas (from validation logic)

**sources.json** — array of `Source` objects:
```json
[
  {
    "id": "hn",
    "name": "Hacker News",
    "url": "https://hacker-news.firebaseio.com/v0",
    "type": "hn",
    "enabled": true,
    "tier": 1,
    "options": {}
  }
]
```

**scoring.json** — `Scoring` struct with tier weights, decay, multipliers:
```json
{
  "tier_weights": { "1": 100, "2": 50, "3": 25, "4": 10, "5": 5 },
  "recency_half_life_hours": 72,
  "source_type_multipliers": { "lab": 1.2, "community": 1.0, "press": 0.8 },
  "min_score": 0.1
}
```

**keywords.json** — `Keywords` struct with signal arrays:
```json
{
  "builder_signals": ["release", "launch", "v1", "beta", "alpha"],
  "hype_signals": ["breakthrough", "revolutionary", "game-changer"]
}
```

---

## Source Enabling and Filtering

The `EnabledSources` method filters the loaded `Sources` slice to only those with `Enabled: true`.

```
agent/internal/config/config.go:92-100
```

```go
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

This is used by:
- `Validate()` — requires at least one enabled source
- `ingest.New()` — constructs adapters only for enabled sources

The `Source` struct (defined in `sources.go`) includes:
- `ID` — unique identifier (validated for uniqueness)
- `Name` — display name
- `URL` — endpoint
- `Type` — one of: `rss`, `hn`, `reddit`, `hf_papers`, `github`
- `Enabled` — boolean
- `Tier` — integer 1-5 (validated)
- `Options` — type-specific configuration (map[string]any)

---

## Validation Rules

`Validate` runs after all files are loaded. It collects **all** problems and returns a single error with a formatted list. This avoids "fix one, re-run, fix next" cycles.

```
agent/internal/config/config.go:194-244
```

```go
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

### Validation Rule Summary

| Category | Rule | Error Message Pattern |
|----------|------|----------------------|
| Sources | At least one enabled source | `no enabled sources in sources.json` |
| Source | `id` required | `sources[i]: missing id` |
| Source | `id` unique | `sources[i]: duplicate id <id>` |
| Source | `name` required | `sources[i] (<id>): missing name` |
| Source | `url` required | `sources[i] (<id>): missing url` |
| Source | `type` in known set | `sources[i] (<id>): unknown type "<type>"` |
| Source | `tier` in 1..5 | `sources[i] (<id>): tier <n> out of range 1-5` |
| Scoring | Delegated to `Scoring.problems()` | (varies) |
| Keywords | `builder_signals` non-empty | `keywords.json: builder_signals is empty` |
| Keywords | `hype_signals` non-empty | `keywords.json: hype_signals is empty` |
| Embedder | Provider in `{ollama, gemini, nvidia_nim}` | `AIRFOIL_EMBEDDER="<val>": want one of "ollama", "gemini", "nvidia_nim"` |

The `validSourceTypes` map (defined in `sources.go`) contains: `rss`, `hn`, `reddit`, `hf_papers`, `github`.

---

## Error Handling

All errors from `Load` are **startup-fatal**. The CLI (`cmd/airfoil`) calls `Load` in `app.run()` and exits with a non-zero code on error, printing the error message.

### Error Types and Sources

| Stage | Possible Errors |
|-------|-----------------|
| `loadDotEnv` | File open error (non-ENOENT), malformed line, `os.Setenv` failure |
| `Load` scalar resolution | None (uses `firstNonEmpty`, never fails) |
| `loadFiles` | File not found, JSON syntax error, type mismatch (via `json.Unmarshal`) |
| `Validate` | Aggregated validation problems (see table above) |

### Error Message Format

Validation errors are formatted as:

```
config: invalid:
  - sources[0]: missing id
  - sources[1] (hn): unknown type "hackernews"
  - keywords.json: builder_signals is empty
```

This makes it trivial to parse in CI logs or grep for specific issues.

---

## Referenced Files

| File | Purpose |
|------|---------|
| `agent/internal/config/config.go` | `Config` struct, `Load`, `Validate`, derived paths, `firstNonEmpty` |
| `agent/internal/config/dotenv.go` | `loadDotEnv`, `parseDotEnvLine` |
| `agent/internal/config/sources.go` | `Source` struct, `loadSources`, `validSourceTypes` (not in source block but referenced) |
| `agent/internal/config/scoring.go` | `Scoring` struct, `loadScoring`, `Scoring.problems()` (not in source block but referenced) |
| `agent/internal/config/keywords.go` | `Keywords` struct, `loadKeywords` (not in source block but referenced) |

---

## Appendix: Complete Environment Variable Reference

All environment variables recognized by `config.Load`, with their struct destination:

| Env Var | Struct Field | Default |
|---------|--------------|---------|
| `AIRFOIL_DATA_DIR` | `Config.DataDir` | `./data` |
| `AIRFOIL_CONFIG_DIR` | `Config.ConfigDir` | `./config` |
| `AIRFOIL_LOG_LEVEL` | `Config.LogLevel` | `info` |
| `SITE_URL` | `Config.SiteURL` | (empty) |
| `AIRFOIL_STORIES_DIR` | `Config.StoriesDir` | `site/src/content/stories` |
| `AIRFOIL_EMBEDDER` | `Config.Embed.Provider` | `ollama` |
| `OLLAMA_HOST` | `Config.LLM.Ollama.Host`, `Config.Embed.OllamaHost` | `http://localhost:11434` |
| `OLLAMA_CHAT_MODEL` | `Config.LLM.Ollama.ChatModel` | `llama3.1` |
| `OLLAMA_EMBED_MODEL` | `Config.LLM.Ollama.EmbedModel`, `Config.Embed.OllamaModel` | `nomic-embed-text` |
| `GEMINI_API_KEY` | `Config.LLM.Gemini.APIKey`, `Config.Embed.GeminiKey` | (empty) |
| `GEMINI_MODEL` | `Config.LLM.Gemini.Model` | `gemini-2.0-flash` |
| `GEMINI_EMBED_MODEL` | `Config.Embed.GeminiModel` | `text-embedding-004` |
| `NVIDIA_NIM_API_KEY` | `Config.LLM.NvidiaNIM.APIKey`, `Config.Embed.NIMKey` | (empty) |
| `NVIDIA_NIM_MODEL` | `Config.LLM.NvidiaNIM.Model` | `meta/llama-3.3-70b-instruct` |
| `NVIDIA_NIM_EMBED_MODEL` | `Config.Embed.NIMModel` | `nvidia/nv-embedqa-e5-v5` |
| `NVIDIA_NIM_BASE_URL` | `Config.Embed.NIMBaseURL` | `https://integrate.api.nvidia.com/v1` |
| `OPENROUTER_API_KEY` | `Config.LLM.OpenRouter.APIKey` | (empty) |
| `OPENROUTER_MODEL` | `Config.LLM.OpenRouter.Model` | `meta-llama/llama-3.3-70b-instruct:free` |
| `GITHUB_TOKEN` | `Config.Ingest.GitHubToken` | (empty) |
| `REDDIT_USER_AGENT` | `Config.Ingest.RedditUserAgent` | `airfoil/0.1` |

<!-- kaioken:files agent/internal/config/config.go,agent/internal/config/dotenv.go -->
