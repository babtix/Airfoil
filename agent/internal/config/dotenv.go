package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

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

// parseDotEnvLine splits one line into a key and value. ok is false for blank
// lines and comments; ok is true with an empty key for malformed lines.
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

// SaveDotEnv updates or creates a .env file with the given key-value updates.
// Existing comments, non-modified keys, and overall formatting are preserved.
func SaveDotEnv(path string, updates map[string]string) error {
	var lines []string
	updatedKeys := make(map[string]bool)

	if f, err := os.Open(path); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			raw := sc.Text()
			key, _, ok := parseDotEnvLine(raw)
			if ok && key != "" {
				if newVal, exists := updates[key]; exists {
					lines = append(lines, fmt.Sprintf("%s=%s", key, newVal))
					updatedKeys[key] = true
					continue
				}
			}
			lines = append(lines, raw)
		}
		_ = sc.Err()
		_ = f.Close()
	}

	// Append any new keys that were not already in the file
	var appended []string
	for k, v := range updates {
		if !updatedKeys[k] && v != "" {
			appended = append(appended, fmt.Sprintf("%s=%s", k, v))
		}
	}
	if len(appended) > 0 {
		if len(lines) > 0 && lines[len(lines)-1] != "" {
			lines = append(lines, "")
		}
		lines = append(lines, appended...)
	}

	content := strings.Join(lines, "\n")
	if len(lines) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("config: mkdir %s: %w", dir, err)
		}
	}
	tmp := filepath.Join(dir, fmt.Sprintf(".env.tmp.%d", os.Getpid()))
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		if err2 := os.WriteFile(path, []byte(content), 0o600); err2 != nil {
			return fmt.Errorf("config: save %s: %w", path, err2)
		}
	}
	return nil
}

// SyncEnv applies the key-value updates to the active Config and process environment.
func (c *Config) SyncEnv(updates map[string]string) {
	for k, v := range updates {
		_ = os.Setenv(k, v)
		switch k {
		case "NVIDIA_NIM_API_KEY":
			c.LLM.NvidiaNIM.APIKey = v
			c.Embed.NIMKey = v
		case "NVIDIA_NIM_MODEL":
			c.LLM.NvidiaNIM.Model = v
		case "OPENROUTER_API_KEY":
			c.LLM.OpenRouter.APIKey = v
		case "OPENROUTER_MODEL":
			c.LLM.OpenRouter.Model = v
		case "AIRFOIL_EMBEDDER":
			c.Embed.Provider = v
		case "NVIDIA_NIM_EMBED_MODEL":
			c.Embed.NIMModel = v
		case "NVIDIA_NIM_BASE_URL":
			c.Embed.NIMBaseURL = v
		case "GITHUB_TOKEN":
			c.Ingest.GitHubToken = v
		case "REDDIT_USER_AGENT":
			c.Ingest.RedditUserAgent = v
		case "SITE_URL":
			c.SiteURL = v
		case "AIRFOIL_LOG_LEVEL":
			c.LogLevel = v
		}
	}
}

// AsEnvMap returns a map of all configurable environment keys from the current Config.
func (c *Config) AsEnvMap() map[string]string {
	return map[string]string{
		"NVIDIA_NIM_API_KEY":     c.LLM.NvidiaNIM.APIKey,
		"NVIDIA_NIM_MODEL":       c.LLM.NvidiaNIM.Model,
		"OPENROUTER_API_KEY":     c.LLM.OpenRouter.APIKey,
		"OPENROUTER_MODEL":       c.LLM.OpenRouter.Model,
		"AIRFOIL_EMBEDDER":       c.Embed.Provider,
		"NVIDIA_NIM_EMBED_MODEL": c.Embed.NIMModel,
		"NVIDIA_NIM_BASE_URL":    c.Embed.NIMBaseURL,
		"GITHUB_TOKEN":           c.Ingest.GitHubToken,
		"REDDIT_USER_AGENT":      c.Ingest.RedditUserAgent,
		"SITE_URL":               c.SiteURL,
		"AIRFOIL_LOG_LEVEL":      c.LogLevel,
	}
}
