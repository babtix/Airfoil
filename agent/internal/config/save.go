package config

import (
	"fmt"
	"path/filepath"

	"github.com/papitsho/airfoil/internal/store"
)

// Paths of the three editable config files.
func (c *Config) SourcesPath() string  { return filepath.Join(c.ConfigDir, "sources.json") }
func (c *Config) ScoringPath() string  { return filepath.Join(c.ConfigDir, "scoring.json") }
func (c *Config) KeywordsPath() string { return filepath.Join(c.ConfigDir, "keywords.json") }

// sourcesComment is restored on save so that operator notes at the top of
// sources.json survive an edit made through the TUI.
const sourcesComment = "Tier 1 = official lab. 2 = research. 3 = dev/tools. 4 = press. 5 = community. " +
	"Verify every feed URL with `airfoil doctor` before trusting it — feed paths change."

// SaveSources validates and writes sources.json atomically.
//
// Validation runs against a copy of the whole config, so a source that would
// break a run is rejected before it reaches disk rather than after.
func (c *Config) SaveSources(sources []Source) error {
	candidate := *c
	candidate.Sources = sources
	if err := candidate.Validate(); err != nil {
		return err
	}

	file := sourcesFile{Comment: sourcesComment, Sources: sources}
	if err := store.WriteJSON(c.SourcesPath(), file); err != nil {
		return fmt.Errorf("config: save sources: %w", err)
	}
	c.Sources = sources
	return nil
}

// SaveScoring validates and writes scoring.json atomically.
func (c *Config) SaveScoring(scoring Scoring) error {
	candidate := *c
	candidate.Scoring = scoring
	if err := candidate.Validate(); err != nil {
		return err
	}

	if err := store.WriteJSON(c.ScoringPath(), scoring); err != nil {
		return fmt.Errorf("config: save scoring: %w", err)
	}
	c.Scoring = scoring
	return nil
}

// SaveKeywords validates and writes keywords.json atomically.
func (c *Config) SaveKeywords(keywords Keywords) error {
	candidate := *c
	candidate.Keywords = keywords
	if err := candidate.Validate(); err != nil {
		return err
	}

	if err := store.WriteJSON(c.KeywordsPath(), keywords); err != nil {
		return fmt.Errorf("config: save keywords: %w", err)
	}
	c.Keywords = keywords
	return nil
}

// Reload re-reads the three config files from disk, discarding unsaved edits.
func (c *Config) Reload() error {
	if err := c.loadFiles(); err != nil {
		return err
	}
	return c.Validate()
}

// ValidSourceTypes lists the source types the ingest layer understands, for
// the type picker in the editor.
func ValidSourceTypes() []string {
	return []string{SourceRSS, SourceHN, SourceReddit, SourceHFPapers, SourceGitHub}
}

// Embedders lists the supported embedding providers.
func Embedders() []string {
	return []string{EmbedderNvidiaNIM, EmbedderGemini}
}
