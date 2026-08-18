package config

import (
	"strings"
	"testing"
)

// validScoring returns a Scoring value that passes every check, so each test
// can break exactly one thing.
func validScoring() Scoring {
	var s Scoring
	s.Clustering.SimilarityThreshold = 0.82
	s.Clustering.WindowHours = 48
	s.Weights.Sources = 22
	s.Weights.Tier = 25
	s.TierWeights = map[string]float64{"1": 1, "2": 0.85, "3": 0.8, "4": 0.5, "5": 0.3}
	s.Recency.HalfLifeHours = 48
	s.Caps.LLMCallsPerRun = 15
	s.Tiers.MajorMinScore = 70
	s.Tiers.NotableMinScore = 40
	s.Limits.ExcerptMaxChars = 300
	s.Limits.SummaryMaxWords = 80
	s.Limits.VerbatimOverlapMaxWords = 12
	s.Limits.QuoteMaxWords = 15
	s.Retention.ItemsDays = 30
	s.Retention.SeenURLsDays = 30
	s.Retention.EmbeddingCacheDays = 7
	return s
}

func TestScoringProblems(t *testing.T) {
	t.Run("valid config has no problems", func(t *testing.T) {
		if got := validScoring().problems(); len(got) != 0 {
			t.Fatalf("got %d problems, want 0: %v", len(got), got)
		}
	})

	tests := []struct {
		name    string
		mutate  func(*Scoring)
		wantSub string
	}{
		{"threshold zero", func(s *Scoring) { s.Clustering.SimilarityThreshold = 0 }, "similarity_threshold"},
		{"threshold above one", func(s *Scoring) { s.Clustering.SimilarityThreshold = 1.5 }, "similarity_threshold"},
		{"window not positive", func(s *Scoring) { s.Clustering.WindowHours = 0 }, "window_hours"},
		{"half life not positive", func(s *Scoring) { s.Recency.HalfLifeHours = 0 }, "half_life_hours"},
		{"missing tier weight", func(s *Scoring) { delete(s.TierWeights, "3") }, "missing tier 3"},
		{"tiers inverted", func(s *Scoring) { s.Tiers.MajorMinScore = 30 }, "must exceed"},
		{"tiers equal", func(s *Scoring) { s.Tiers.MajorMinScore = s.Tiers.NotableMinScore }, "must exceed"},
		{"no llm budget", func(s *Scoring) { s.Caps.LLMCallsPerRun = 0 }, "llm_calls_per_run"},
		{"excerpt cap missing breaks R1", func(s *Scoring) { s.Limits.ExcerptMaxChars = 0 }, "excerpt_max_chars"},
		{"summary cap missing breaks R1", func(s *Scoring) { s.Limits.SummaryMaxWords = 0 }, "summary_max_words"},
		{"overlap cap missing breaks R1", func(s *Scoring) { s.Limits.VerbatimOverlapMaxWords = 0 }, "verbatim_overlap_max_words"},
		{"quote cap missing breaks R2", func(s *Scoring) { s.Limits.QuoteMaxWords = 0 }, "quote_max_words"},
		{"items retention", func(s *Scoring) { s.Retention.ItemsDays = 0 }, "retention.items_days"},
		{"seen url retention", func(s *Scoring) { s.Retention.SeenURLsDays = 0 }, "retention.seen_urls_days"},
		{"cache retention", func(s *Scoring) { s.Retention.EmbeddingCacheDays = 0 }, "retention.embedding_cache_days"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validScoring()
			tt.mutate(&s)

			got := s.problems()
			if len(got) == 0 {
				t.Fatalf("expected a problem mentioning %q, got none", tt.wantSub)
			}
			if !strings.Contains(strings.Join(got, "\n"), tt.wantSub) {
				t.Errorf("problems %v, want one mentioning %q", got, tt.wantSub)
			}
		})
	}
}

func TestScoringTierWeight(t *testing.T) {
	s := validScoring()

	for tier, want := range map[int]float64{1: 1, 2: 0.85, 3: 0.8, 4: 0.5, 5: 0.3} {
		if got := s.TierWeight(tier); got != want {
			t.Errorf("TierWeight(%d) = %v, want %v", tier, got, want)
		}
	}
	// An unconfigured tier contributes nothing rather than panicking.
	if got := s.TierWeight(9); got != 0 {
		t.Errorf("TierWeight(9) = %v, want 0", got)
	}
}
