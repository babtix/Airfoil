// Package score ranks clusters for builder relevance.
//
// Every function here is pure: same cluster, same config, same clock reading,
// same result. That is what lets the weights in config/scoring.json be tuned
// against real output instead of guesses (R10).
package score

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
)

// Structural builder signals. These are named in keywords.json rather than
// matched as text, because they describe the item, not its prose.
const (
	signalRepoURL    = "has_repo_url"
	signalPaperURL   = "has_paper_url"
	signalTierPrefix = "source_tier_"
)

// Result is one scored cluster. The breakdown fields exist so `airfoil rank
// --explain` can show why a story placed where it did — the phase-3 gate is a
// human reading the top 20, and that needs evidence.
type Result struct {
	ClusterID string    `json:"cluster_id"`
	Title     string    `json:"title"`
	Date      time.Time `json:"date"`

	Score           int      `json:"score"`
	Tier            string   `json:"tier"`
	Tags            []string `json:"tags"`
	BuilderRelevant bool     `json:"builder_relevant"`

	DistinctSources int      `json:"distinct_sources"`
	MaxTierWeight   float64  `json:"max_tier_weight"`
	BestTier        int      `json:"best_tier"`
	HNPoints        int      `json:"hn_points,omitempty"`
	RedditScore     int      `json:"reddit_score,omitempty"`
	BuilderSignals  []string `json:"builder_signals,omitempty"`
	HypeSignals     []string `json:"hype_signals,omitempty"`

	AgeHours float64 `json:"age_hours"`
	Decay    float64 `json:"decay"`
	Raw      float64 `json:"raw"`
}

// maxTags caps how many topic tags a story carries. The site renders them in a
// single row; beyond four they wrap and stop being scannable.
const maxTags = 4

// Score ranks one cluster. now is passed in rather than read from the clock so
// the function stays pure and testable.
func Score(c model.Cluster, sc config.Scoring, kw config.Keywords, now time.Time) Result {
	r := Result{ClusterID: c.ID}
	if len(c.Items) == 0 {
		r.Tier = model.TierMinor
		return r
	}

	// The cluster lead is set by the cluster package: highest tier, earliest
	// published. Its title names the story.
	r.Title = c.Items[0].Title

	corpus := corpusOf(c.Items)

	sources := map[string]bool{}
	for _, item := range c.Items {
		sources[item.SourceID] = true

		if w := sc.TierWeight(item.SourceTier); w > r.MaxTierWeight {
			r.MaxTierWeight = w
			r.BestTier = item.SourceTier
		}
		// Max, not sum: two feeds carrying the same HN thread would otherwise
		// double-count one community signal.
		r.HNPoints = max(r.HNPoints, item.Metrics.HNPoints)
		r.RedditScore = max(r.RedditScore, item.Metrics.RedditScore)

		// A story's age is when the event broke, not when the latest outlet
		// got to it. Continuing coverage should decay, not stay fresh.
		if r.Date.IsZero() || item.PublishedAt.Before(r.Date) {
			r.Date = item.PublishedAt
		}
	}
	r.DistinctSources = len(sources)

	r.BuilderSignals = append(
		matchAll(corpus, kw.BuilderSignals),
		structuralSignals(c.Items, kw.BuilderStructuralSignals)...,
	)
	sortStrings(r.BuilderSignals)
	r.HypeSignals = matchAll(corpus, kw.HypeSignals)
	r.Tags = topicTags(corpus, kw.TopicTags)

	builder := capped(len(r.BuilderSignals), sc.Caps.BuilderSignalMax)
	hype := capped(len(r.HypeSignals), sc.Caps.HypeSignalMax)

	w := sc.Weights
	r.Raw = w.Sources*math.Log(1+float64(r.DistinctSources)) +
		w.Tier*r.MaxTierWeight +
		w.HN*math.Log(1+float64(r.HNPoints)) +
		w.Reddit*math.Log(1+float64(r.RedditScore)) +
		w.Builder*float64(builder) -
		w.Hype*float64(hype)

	r.AgeHours = math.Max(0, now.Sub(r.Date).Hours())
	r.Decay = decay(r.AgeHours, sc.Recency.HalfLifeHours)

	r.Score = int(math.Round(clamp(r.Raw*r.Decay, 0, 100)))
	r.Tier = tierFor(r.Score, sc)
	r.BuilderRelevant = len(r.BuilderSignals) >= sc.BuilderRelevantMinSignals

	return r
}

// Rank scores every cluster and orders them best first. Ties break on date then
// cluster ID so a rerun over unchanged data produces an identical file.
func Rank(clusters []model.Cluster, sc config.Scoring, kw config.Keywords, now time.Time) []Result {
	out := make([]Result, 0, len(clusters))
	for _, c := range clusters {
		out = append(out, Score(c, sc, kw, now))
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if !a.Date.Equal(b.Date) {
			return a.Date.After(b.Date)
		}
		return a.ClusterID < b.ClusterID
	})
	return out
}

// corpusOf builds the lowercased text every keyword is matched against.
func corpusOf(items []model.Item) string {
	var b strings.Builder
	for _, item := range items {
		b.WriteString(item.Title)
		b.WriteByte(' ')
		b.WriteString(item.Excerpt)
		// A separator that is not a word character, so a keyword cannot match
		// across the seam between two items.
		b.WriteString(" | ")
	}
	return strings.ToLower(b.String())
}

// structuralSignals resolves the named signals in keywords.json that describe
// the item rather than its text.
func structuralSignals(items []model.Item, names []string) []string {
	var out []string
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		var hit bool
		switch {
		case name == signalRepoURL:
			hit = anyItem(items, func(i model.Item) bool { return i.RepoURL != "" })
		case name == signalPaperURL:
			hit = anyItem(items, func(i model.Item) bool { return i.PaperURL != "" })
		case strings.HasPrefix(name, signalTierPrefix):
			tier, err := strconv.Atoi(strings.TrimPrefix(name, signalTierPrefix))
			if err != nil {
				continue // an unrecognised signal is config noise, not a crash
			}
			hit = anyItem(items, func(i model.Item) bool { return i.SourceTier == tier })
		default:
			continue
		}
		if hit {
			out = append(out, name)
		}
	}
	return out
}

func anyItem(items []model.Item, pred func(model.Item) bool) bool {
	for _, item := range items {
		if pred(item) {
			return true
		}
	}
	return false
}

// topicTags returns the tags whose keywords appear in the corpus, strongest
// first, capped at maxTags.
func topicTags(corpus string, groups map[string][]string) []string {
	type scored struct {
		tag string
		n   int
	}
	var hits []scored
	for tag, phrases := range groups {
		if n := len(matchAll(corpus, phrases)); n > 0 {
			hits = append(hits, scored{tag, n})
		}
	}
	// Map iteration is random, so the tiebreak on name is what keeps the tag
	// list stable between runs.
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].n != hits[j].n {
			return hits[i].n > hits[j].n
		}
		return hits[i].tag < hits[j].tag
	})

	out := make([]string, 0, maxTags)
	for _, h := range hits {
		if len(out) == maxTags {
			break
		}
		out = append(out, h.tag)
	}
	return out
}

// decay is the recency half-life curve. A zero or negative half-life would make
// the exponent explode, so it is treated as "no decay".
func decay(ageHours, halfLifeHours float64) float64 {
	if halfLifeHours <= 0 {
		return 1
	}
	return math.Exp(-math.Ln2 * ageHours / halfLifeHours)
}

func tierFor(score int, sc config.Scoring) string {
	switch {
	case score >= sc.Tiers.MajorMinScore:
		return model.TierMajor
	case score >= sc.Tiers.NotableMinScore:
		return model.TierNotable
	default:
		return model.TierMinor
	}
}

// capped applies a signal cap. A limit of zero or less means uncapped.
func capped(n, limit int) int {
	if limit > 0 && n > limit {
		return limit
	}
	return n
}

func clamp(v, lo, hi float64) float64 {
	return math.Min(hi, math.Max(lo, v))
}
