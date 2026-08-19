package config

import (
	"fmt"

	"github.com/papitsho/airfoil/internal/store"
)

// Source types recognised by the ingest layer.
const (
	SourceRSS      = "rss"
	SourceHN       = "hn"
	SourceReddit   = "reddit"
	SourceHFPapers = "hf_papers"
	SourceHFModels = "hf_models"
	SourceGitHub   = "github"
)

var validSourceTypes = map[string]bool{
	SourceRSS:      true,
	SourceHN:       true,
	SourceReddit:   true,
	SourceHFPapers: true,
	SourceHFModels: true,
	SourceGitHub:   true,
}

// Source is one entry in config/sources.json.
type Source struct {
	Comment string        `json:"$comment,omitempty"`
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	Type    string        `json:"type"`
	URL     string        `json:"url"`
	Tier    int           `json:"tier"`
	Tags    []string      `json:"tags"`
	Enabled bool          `json:"enabled"`
	Options SourceOptions `json:"options"`
}

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

	// hf_models
	MinLikes int    `json:"min_likes"`
	Sort     string `json:"sort"`

	// github
	CreatedWithinDays int `json:"created_within_days"`
	MinStars          int `json:"min_stars"`
	MaxPerQuery       int `json:"max_per_query"`
}

type sourcesFile struct {
	Comment string   `json:"$comment,omitempty"`
	Sources []Source `json:"sources"`
}

func loadSources(path string) ([]Source, error) {
	f, err := store.ReadJSON[sourcesFile](path)
	if err != nil {
		return nil, fmt.Errorf("config: sources: %w", err)
	}
	return f.Sources, nil
}

// Scoring is config/scoring.json. Every knob the ranking depends on lives here
// so it can be tuned without a rebuild.
type Scoring struct {
	Comment string `json:"$comment,omitempty"`

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

// TierWeight returns the scoring weight for a source tier, or 0 if the tier is
// not configured.
func (s Scoring) TierWeight(tier int) float64 {
	return s.TierWeights[fmt.Sprint(tier)]
}

func loadScoring(path string) (Scoring, error) {
	s, err := store.ReadJSON[Scoring](path)
	if err != nil {
		return Scoring{}, fmt.Errorf("config: scoring: %w", err)
	}
	return s, nil
}

// problems reports scoring values that would produce nonsense rankings.
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
	if s.Limits.ExcerptMaxChars <= 0 {
		add("limits.excerpt_max_chars must be > 0 (R1)")
	}
	if s.Limits.SummaryMaxWords <= 0 {
		add("limits.summary_max_words must be > 0 (R1)")
	}
	if s.Limits.VerbatimOverlapMaxWords <= 0 {
		add("limits.verbatim_overlap_max_words must be > 0 (R1)")
	}
	if s.Limits.QuoteMaxWords <= 0 {
		add("limits.quote_max_words must be > 0 (R2)")
	}

	for name, days := range map[string]int{
		"retention.items_days":           s.Retention.ItemsDays,
		"retention.seen_urls_days":       s.Retention.SeenURLsDays,
		"retention.embedding_cache_days": s.Retention.EmbeddingCacheDays,
	} {
		if days <= 0 {
			add("%s must be > 0", name)
		}
	}

	return out
}

// Keywords is config/keywords.json. Matched case-insensitively against
// title + excerpt.
type Keywords struct {
	Comment string `json:"$comment,omitempty"`

	BuilderSignals           []string            `json:"builder_signals"`
	BuilderStructuralSignals []string            `json:"builder_structural_signals"`
	HypeSignals              []string            `json:"hype_signals"`
	TopicTags                map[string][]string `json:"topic_tags"`
}

func loadKeywords(path string) (Keywords, error) {
	k, err := store.ReadJSON[Keywords](path)
	if err != nil {
		return Keywords{}, fmt.Errorf("config: keywords: %w", err)
	}
	return k, nil
}
