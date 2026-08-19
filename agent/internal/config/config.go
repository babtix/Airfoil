// Package config loads environment variables and the JSON files under config/
// into one explicit struct. Nothing in the agent reads os.Getenv directly, and
// there is no global config value — it is passed down from the root command.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the fully resolved configuration for a run.
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

// LLMConfig holds credentials and model IDs for the summarization chain.
// Model IDs live in env because free models rotate out.
//
// Every provider is a hosted API. Local models are deliberately not supported:
// the pipeline runs in CI, where no daemon is listening, so a local fallback
// would pass on a developer machine and fail in the only place that matters.
type LLMConfig struct {
	Gemini     ProviderCreds
	NvidiaNIM  ProviderCreds
	OpenRouter ProviderCreds
}

// ProviderCreds is an API key plus the model ID to call.
type ProviderCreds struct {
	APIKey string
	Model  string
}

// Configured reports whether this provider has enough to be attempted.
func (p ProviderCreds) Configured() bool {
	return p.APIKey != "" && p.Model != ""
}

// Embedder providers.
const (
	EmbedderGemini    = "gemini"
	EmbedderNvidiaNIM = "nvidia_nim"
)

// EmbedConfig selects and configures the embedding provider.
//
// Dimensionality differs per provider, and the clustering threshold is tuned
// against one of them, so switching providers means retuning. The embedding
// cache is keyed by model to keep vectors from two providers from mixing.
type EmbedConfig struct {
	Provider    string // gemini | nvidia_nim
	GeminiKey   string
	GeminiModel string
	NIMKey      string
	NIMModel    string
	NIMBaseURL  string
}

// Model returns the model ID for the selected provider.
func (e EmbedConfig) Model() string {
	switch e.Provider {
	case EmbedderGemini:
		return e.GeminiModel
	default:
		return e.NIMModel
	}
}

// IngestConfig holds credentials the ingest adapters need.
type IngestConfig struct {
	GitHubToken     string
	RedditUserAgent string
}

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

// Options carries the values set by global CLI flags. Empty fields fall back to
// environment variables, then to defaults.
type Options struct {
	ConfigDir string
	DataDir   string
	Verbose   bool
}

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
		},

		Embed: EmbedConfig{
			Provider:    firstNonEmpty(os.Getenv("AIRFOIL_EMBEDDER"), EmbedderNvidiaNIM),
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

	// Only the provider name is checked here. A missing API key is not a
	// config error: `airfoil version`, config editing, and the TUI all load
	// config without ever embedding. The key is required at the point of use,
	// where embed.New reports it precisely, and doctor --ping proves it works.
	switch c.Embed.Provider {
	case EmbedderGemini, EmbedderNvidiaNIM:
	default:
		problems = append(problems, fmt.Sprintf("AIRFOIL_EMBEDDER=%q: want one of %q, %q",
			c.Embed.Provider, EmbedderGemini, EmbedderNvidiaNIM))
	}

	if len(problems) > 0 {
		return fmt.Errorf("config: invalid:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
