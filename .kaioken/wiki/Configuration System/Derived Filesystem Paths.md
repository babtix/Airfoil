# Derived Filesystem Paths

This chapter documents every derived path method on `config.Config`, its implementation, default fallback behavior, and where each path is used in the agent pipeline. All paths are computed from the two root directories: `DataDir` (for generated data) and `StoriesDir` (for Markdown story files).

## Table of Contents

- [Root Directories](#root-directories)
- [Derived Path Methods](#derived-path-methods)
- [Path Summary Table](#path-summary-table)
- [Usage in Pipeline](#usage-in-pipeline)
- [Validation & Edge Cases](#validation--edge-cases)

---

## Root Directories

Before the derived paths are computed, `config.Load` resolves two root directories with the following precedence (CLI flag → environment variable → default):

| Root Directory | CLI Flag | Environment Variable | Default |
|----------------|----------|---------------------|---------|
| `DataDir` | `--data-dir` | `AIRFOIL_DATA_DIR` | `./data` |
| `ConfigDir` | `--config-dir` | `AIRFOIL_CONFIG_DIR` | `./config` |
| `StoriesDir` | — | `AIRFOIL_STORIES_DIR` | `site/src/content/stories` |

```go
agent/internal/config/config.go:115-175
```

```go
func Load(opts Options) (*Config, error) {
	// ...
	cfg := &Config{
		DataDir:   firstNonEmpty(opts.DataDir, os.Getenv("AIRFOIL_DATA_DIR"), "./data"),
		ConfigDir: firstNonEmpty(opts.ConfigDir, os.Getenv("AIRFOIL_CONFIG_DIR"), "./config"),
		// ...
	}
	// The markdown collection lives in the site, not in data/.
	cfg.StoriesDir = firstNonEmpty(
		os.Getenv("AIRFOIL_STORIES_DIR"),
		filepath.Join("site", "src", "content", "stories"),
	)
	// ...
}
```

`firstNonEmpty` returns the first non-empty string in its variadic arguments, establishing the precedence chain.

```go
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

---

## Derived Path Methods

All derived paths are **methods on `Config`** (not fields), computed lazily from the resolved root directories. They use `path/filepath.Join` for platform-correct path construction.

### ItemsDir

```go
agent/internal/config/config.go:81
```

```go
func (c Config) ItemsDir() string    { return filepath.Join(c.DataDir, "items") }
```

- **Returns**: `<DataDir>/items`
- **Default**: `./data/items`
- **Purpose**: Directory containing per-item JSON files (`data/items/*.json`). Each ingested and normalized `model.Item` is written here as a separate file named by its `ItemID` (hash of canonical URL).
- **Consumers**: `store.WriteJSON` (item persistence), `ingest.Runner` (reading existing items for dedupe context).

### CacheDir

```go
agent/internal/config/config.go:82
```

```go
func (c Config) CacheDir() string    { return filepath.Join(c.DataDir, "cache") }
```

- **Returns**: `<DataDir>/cache`
- **Default**: `./data/cache`
- **Purpose**: Parent directory for all cached artifacts. **Never committed to Git** (embeddings are regenerable and large).
- **Consumers**: `EmbedCachePath()` builds on this; embedder implementations write provider-specific cache files here.

### DigestDir

```go
agent/internal/config/config.go:83
```

```go
func (c Config) DigestDir() string   { return filepath.Join(c.DataDir, "digest") }
```

- **Returns**: `<DataDir>/digest`
- **Default**: `./data/digest`
- **Purpose**: Directory for digest-related output (e.g., daily/weekly digest JSON or Markdown). Currently reserved for future digest generation feature.
- **Consumers**: Not yet used in the current pipeline; present for forward compatibility.

### IndexPath

```go
agent/internal/config/config.go:84
```

```go
func (c Config) IndexPath() string   { return filepath.Join(c.DataDir, "index.json") }
```

- **Returns**: `<DataDir>/index.json`
- **Default**: `./data/index.json`
- **Purpose**: Serialized `model.Index` (ranked list of `IndexEntry` pointing to stories with scores). Primary data source for the React site at build time.
- **Consumers**: `store.WriteJSON` (pipeline write), `site` build (Vite reads this at compile time).

### StatePath

```go
agent/internal/config/config.go:85
```

```go
func (c Config) StatePath() string   { return filepath.Join(c.DataDir, "state.json") }
```

- **Returns**: `<DataDir>/state.json`
- **Default**: `./data/state.json`
- **Purpose**: Serialized `model.State` (deduplication tracker). Contains per-day `Seen` map keyed by `ItemID`. Enables idempotent re-runs: `State.Seen(id)` checks this file; `State.MarkSeen(id, day)` updates it.
- **Consumers**: `normalize.Dedupe` (reads at start, writes at end), `model.State` methods.

### StoriesPath

```go
agent/internal/config/config.go:86
```

```go
func (c Config) StoriesPath() string { return filepath.Join(c.DataDir, "stories.json") }
```

- **Returns**: `<DataDir>/stories.json`
- **Default**: `./data/stories.json`
- **Purpose**: Serialized `[]model.Story` (all stories for the run, including frontmatter). Written alongside Markdown files for programmatic access.
- **Consumers**: Pipeline write step; not directly consumed by the site (site reads Markdown from `StoriesDir`).

### EmbedCachePath

```go
agent/internal/config/config.go:87-89
```

```go
func (c Config) EmbedCachePath() string {
	return filepath.Join(c.CacheDir(), "embeddings.json")
}
```

- **Returns**: `<DataDir>/cache/embeddings.json`
- **Default**: `./data/cache/embeddings.json`
- **Purpose**: Embedding vector cache keyed by `(model, itemID)`. Prevents re-embedding identical items across runs. **Never committed**.
- **Consumers**: Embedder implementations (Ollama, Gemini, NVIDIA NIM) read/write this file. The cache is model-scoped so switching providers doesn't mix vectors.

---

## Path Summary Table

| Method | Path Expression | Default | Git Tracked? | Primary Consumer |
|--------|----------------|---------|--------------|------------------|
| `ItemsDir()` | `DataDir/items` | `./data/items` | ✅ (individual item JSONs) | `store`, `ingest.Runner` |
| `CacheDir()` | `DataDir/cache` | `./data/cache` | ❌ (gitignored) | Embedders |
| `DigestDir()` | `DataDir/digest` | `./data/digest` | ✅ (future) | — |
| `IndexPath()` | `DataDir/index.json` | `./data/index.json` | ✅ | Site build (Vite) |
| `StatePath()` | `DataDir/state.json` | `./data/state.json` | ✅ | `normalize.Dedupe`, `model.State` |
| `StoriesPath()` | `DataDir/stories.json` | `./data/stories.json` | ✅ | Pipeline write |
| `EmbedCachePath()` | `DataDir/cache/embeddings.json` | `./data/cache/embeddings.json` | ❌ (gitignored) | Embedders |

> **Note**: `StoriesDir` (the Markdown output directory) is **not** a derived path—it is a root directory resolved directly from `AIRFOIL_STORIES_DIR` or default `site/src/content/stories`. Markdown stories are written to `StoriesDir/*.md`, while `StoriesPath()` points to the JSON aggregate in `DataDir`.

---

## Usage in Pipeline

The derived paths are threaded through the pipeline via the `*Config` pointer passed from `cmd/airfoil` → `internal/config` → downstream packages.

```mermaid
flowchart TD
    Load[config.Load] --> Cfg[*Config]
    Cfg --> ItemsDir[ItemsDir()]
    Cfg --> CacheDir[CacheDir()]
    Cfg --> DigestDir[DigestDir()]
    Cfg --> IndexPath[IndexPath()]
    Cfg --> StatePath[StatePath()]
    Cfg --> StoriesPath[StoriesPath()]
    Cfg --> EmbedCachePath[EmbedCachePath()]

    ItemsDir --> StoreWriteItems[store.WriteJSON<br/>data/items/*.json]
    ItemsDir --> IngestRunner[ingest.Runner<br/>read existing items]

    CacheDir --> EmbedCachePath
    EmbedCachePath --> Embedders[Ollama/Gemini/NIM<br/>embedding cache]

    IndexPath --> StoreWriteIndex[store.WriteJSON<br/>data/index.json]
    IndexPath --> SiteBuild[Vite build<br/>reads index.json]

    StatePath --> StateLoad[model.State.Load<br/>data/state.json]
    StatePath --> StateSave[model.State.Save<br/>data/state.json]
    StateLoad --> NormalizeDedupe[normalize.Dedupe]
    NormalizeDedupe --> StateSave

    StoriesPath --> StoreWriteStories[store.WriteJSON<br/>data/stories.json]
    StoriesDir[StoriesDir<br/>root dir] --> MarkdownWrite[Write *.md<br/>site/src/content/stories/]
```

---

## Validation & Edge Cases

### Config.Validate

`Config.Validate` does **not** validate the derived paths themselves (they are pure functions of `DataDir`/`StoriesDir`). It validates that the root directories lead to a usable configuration:

```go
agent/internal/config/config.go:194-244
```

```go
func (c *Config) Validate() error {
	var problems []string

	if len(c.EnabledSources()) == 0 {
		problems = append(problems, "no enabled sources in sources.json")
	}
	// ... source validation ...
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

### Edge Cases

| Scenario | Behavior |
|----------|----------|
| `AIRFOIL_DATA_DIR` set to absolute path | All derived paths become absolute (via `filepath.Join`). |
| `AIRFOIL_DATA_DIR` set to relative path | Derived paths are relative to working directory at runtime. |
| `AIRFOIL_STORIES_DIR` overridden | Only affects Markdown output location (`StoriesDir`); `StoriesPath()` (JSON aggregate) remains under `DataDir`. |
| `DataDir` = `StoriesDir` (misconfiguration) | Possible but not prevented; would mix generated JSON with site content. Validation does not check for this. |
| `CacheDir` missing at runtime | Embedders create it on first write (`os.MkdirAll` in store/embedder code). |
| Concurrent runs with same `DataDir` | **Not safe** — `state.json` and `embeddings.json` are not locked. Run sequentially or use separate `DataDir`. |

---

## Referenced Files

- `agent/internal/config/config.go` — All derived path methods, `Load`, `Validate`, `firstNonEmpty`

<!-- kaioken:files agent/internal/config/config.go -->
