package config

import (
	"strings"
	"testing"
)

// validConfig returns a Config that passes Validate, so each test can break
// exactly one thing.
func validConfig() *Config {
	return &Config{
		ConfigDir: "./config",
		DataDir:   "./data",
		Sources: []Source{
			{ID: "anthropic", Name: "Anthropic", Type: SourceRSS, URL: "https://x/rss", Tier: 1, Enabled: true},
			{ID: "hn", Name: "Hacker News", Type: SourceHN, URL: "https://y", Tier: 5, Enabled: true},
		},
		Scoring: validScoring(),
		Keywords: Keywords{
			BuilderSignals: []string{"release"},
			HypeSignals:    []string{"insane"},
		},
		Embed: EmbedConfig{Provider: EmbedderNvidiaNIM, NIMKey: "test-key"},
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid config passes", func(t *testing.T) {
		if err := validConfig().Validate(); err != nil {
			t.Fatalf("Validate() = %v, want nil", err)
		}
	})

	tests := []struct {
		name    string
		mutate  func(*Config)
		wantSub string
	}{
		{
			"all sources disabled",
			func(c *Config) {
				for i := range c.Sources {
					c.Sources[i].Enabled = false
				}
			},
			"no enabled sources",
		},
		{"missing id", func(c *Config) { c.Sources[0].ID = "" }, "missing id"},
		{"duplicate id", func(c *Config) { c.Sources[1].ID = c.Sources[0].ID }, "duplicate id"},
		{"missing name", func(c *Config) { c.Sources[0].Name = "" }, "missing name"},
		{"missing url", func(c *Config) { c.Sources[0].URL = "" }, "missing url"},
		{"unknown type", func(c *Config) { c.Sources[0].Type = "gopher" }, `unknown type "gopher"`},
		{"tier too low", func(c *Config) { c.Sources[0].Tier = 0 }, "out of range"},
		{"tier too high", func(c *Config) { c.Sources[0].Tier = 6 }, "out of range"},
		{"scoring problems surface", func(c *Config) { c.Scoring.Clustering.WindowHours = 0 }, "window_hours"},
		{"no builder signals", func(c *Config) { c.Keywords.BuilderSignals = nil }, "builder_signals is empty"},
		{"no hype signals", func(c *Config) { c.Keywords.HypeSignals = nil }, "hype_signals is empty"},
		{"unknown embedder", func(c *Config) { c.Embed.Provider = "word2vec" }, "AIRFOIL_EMBEDDER"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validConfig()
			tt.mutate(c)

			err := c.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error mentioning %q", tt.wantSub)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("Validate() = %v, want it to mention %q", err, tt.wantSub)
			}
		})
	}
}

func TestEnabledSources(t *testing.T) {
	c := validConfig()
	c.Sources = append(c.Sources, Source{
		ID: "off", Name: "Disabled", Type: SourceRSS, URL: "https://z", Tier: 4, Enabled: false,
	})

	got := c.EnabledSources()
	if len(got) != 2 {
		t.Fatalf("EnabledSources() returned %d sources, want 2", len(got))
	}
	for _, s := range got {
		if s.ID == "off" {
			t.Error("EnabledSources() included a disabled source")
		}
	}
}

func TestProviderCredsConfigured(t *testing.T) {
	tests := []struct {
		name  string
		creds ProviderCreds
		want  bool
	}{
		{"key and model", ProviderCreds{APIKey: "k", Model: "m"}, true},
		{"model without key", ProviderCreds{Model: "m"}, false},
		{"key without model", ProviderCreds{APIKey: "k"}, false},
		{"neither", ProviderCreds{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.creds.Configured(); got != tt.want {
				t.Errorf("Configured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFirstNonEmpty(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"flag wins over env and default", []string{"flag", "env", "def"}, "flag"},
		{"env wins over default", []string{"", "env", "def"}, "env"},
		{"falls through to default", []string{"", "", "def"}, "def"},
		{"all empty", []string{"", ""}, ""},
		{"no values", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := firstNonEmpty(tt.in...); got != tt.want {
				t.Errorf("firstNonEmpty(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
