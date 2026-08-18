package score

import (
	"math"
	"testing"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
)

var now = time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

// testScoring mirrors config/scoring.json. Tests assert against these numbers,
// so a weight change in the real file cannot silently break them.
func testScoring() config.Scoring {
	var s config.Scoring
	s.Clustering.SimilarityThreshold = 0.82
	s.Clustering.WindowHours = 48

	s.Weights.Sources = 22
	s.Weights.Tier = 25
	s.Weights.HN = 6
	s.Weights.Reddit = 5
	s.Weights.Builder = 4
	s.Weights.Hype = 6

	s.TierWeights = map[string]float64{"1": 1.00, "2": 0.85, "3": 0.80, "4": 0.50, "5": 0.30}
	s.Recency.HalfLifeHours = 48

	s.Caps.BuilderSignalMax = 5
	s.Caps.HypeSignalMax = 5
	s.Caps.LLMCallsPerRun = 15

	s.Tiers.MajorMinScore = 70
	s.Tiers.NotableMinScore = 40
	s.BuilderRelevantMinSignals = 2

	return s
}

func testKeywords() config.Keywords {
	return config.Keywords{
		BuilderSignals:           []string{"release", "open source", "api", "sdk", "benchmark", "pricing"},
		BuilderStructuralSignals: []string{"has_repo_url", "has_paper_url", "source_tier_1", "source_tier_3"},
		HypeSignals:              []string{"shocking", "game-changer", "insane", "the end of"},
		TopicTags: map[string][]string{
			"models":      {"model", "gpt", "claude"},
			"agents":      {"agent", "mcp"},
			"open-source": {"open source", "apache 2.0"},
			"research":    {"paper", "arxiv"},
		},
	}
}

// item builds a minimal Item. Fields the test does not care about stay zero.
func item(id, sourceID string, tier int, published time.Time, title, excerpt string) model.Item {
	return model.Item{
		ID:          id,
		SourceID:    sourceID,
		SourceName:  sourceID,
		SourceTier:  tier,
		URL:         "https://example.com/" + id,
		Title:       title,
		Excerpt:     excerpt,
		PublishedAt: published,
	}
}

func TestScoreEmptyCluster(t *testing.T) {
	got := Score(model.Cluster{ID: "empty"}, testScoring(), testKeywords(), now)

	if got.Score != 0 {
		t.Errorf("Score = %d, want 0", got.Score)
	}
	if got.Tier != model.TierMinor {
		t.Errorf("Tier = %q, want %q", got.Tier, model.TierMinor)
	}
}

func TestScoreFormula(t *testing.T) {
	// One tier-1 source, no community metrics, no text signals, published now.
	// Being tier 1 is itself a structural builder signal, so the builder term
	// contributes one unit:
	//
	//   raw   = 22*ln(2) + 25*1.00 + 4*1 = 15.2493 + 25 + 4 = 44.2493
	//   decay = 1
	c := model.Cluster{ID: "c1", Items: []model.Item{
		item("a", "openai", 1, now, "Something happened", "A neutral sentence."),
	}}

	got := Score(c, testScoring(), testKeywords(), now)

	wantRaw := 22*math.Log(2) + 25*1.00 + 4*1
	if math.Abs(got.Raw-wantRaw) > 1e-9 {
		t.Errorf("Raw = %v, want %v", got.Raw, wantRaw)
	}
	if got.Decay != 1 {
		t.Errorf("Decay = %v, want 1", got.Decay)
	}
	if got.Score != 44 {
		t.Errorf("Score = %d, want 44", got.Score)
	}
	if got.Tier != model.TierNotable {
		t.Errorf("Tier = %q, want notable", got.Tier)
	}
	if !equal(got.BuilderSignals, []string{"source_tier_1"}) {
		t.Errorf("BuilderSignals = %v, want [source_tier_1]", got.BuilderSignals)
	}
}

func TestScoreDistinctSourcesRaisesScore(t *testing.T) {
	published := now
	one := model.Cluster{ID: "one", Items: []model.Item{
		item("a", "techcrunch", 4, published, "Lab ships thing", ""),
	}}
	three := model.Cluster{ID: "three", Items: []model.Item{
		item("a", "techcrunch", 4, published, "Lab ships thing", ""),
		item("b", "verge", 4, published, "Lab ships thing", ""),
		item("c", "venturebeat", 4, published, "Lab ships thing", ""),
	}}

	sc, kw := testScoring(), testKeywords()
	lo := Score(one, sc, kw, now)
	hi := Score(three, sc, kw, now)

	if hi.DistinctSources != 3 {
		t.Errorf("DistinctSources = %d, want 3", hi.DistinctSources)
	}
	if hi.Score <= lo.Score {
		t.Errorf("three sources scored %d, not above one source at %d", hi.Score, lo.Score)
	}
}

func TestScoreCountsDistinctSourcesNotItems(t *testing.T) {
	// Two items from the same feed are one source. Otherwise a chatty feed
	// could manufacture the appearance of broad coverage.
	c := model.Cluster{ID: "c", Items: []model.Item{
		item("a", "arxiv", 2, now, "A paper", ""),
		item("b", "arxiv", 2, now, "A paper, again", ""),
		item("c", "arxiv", 2, now, "A paper, once more", ""),
	}}

	if got := Score(c, testScoring(), testKeywords(), now); got.DistinctSources != 1 {
		t.Errorf("DistinctSources = %d, want 1", got.DistinctSources)
	}
}

func TestScoreUsesBestTierInCluster(t *testing.T) {
	c := model.Cluster{ID: "c", Items: []model.Item{
		item("a", "hn", 5, now, "Discussion", ""),
		item("b", "openai", 1, now, "Official post", ""),
	}}

	got := Score(c, testScoring(), testKeywords(), now)

	if got.MaxTierWeight != 1.00 {
		t.Errorf("MaxTierWeight = %v, want 1.00", got.MaxTierWeight)
	}
	if got.BestTier != 1 {
		t.Errorf("BestTier = %d, want 1", got.BestTier)
	}
}

func TestScoreCommunityMetricsUseMaxNotSum(t *testing.T) {
	c := model.Cluster{ID: "c", Items: []model.Item{
		func() model.Item {
			i := item("a", "hn", 5, now, "Thing", "")
			i.Metrics.HNPoints = 400
			return i
		}(),
		func() model.Item {
			i := item("b", "hn-mirror", 5, now, "Thing", "")
			i.Metrics.HNPoints = 380
			return i
		}(),
	}}

	if got := Score(c, testScoring(), testKeywords(), now); got.HNPoints != 400 {
		t.Errorf("HNPoints = %d, want 400 (max, not sum)", got.HNPoints)
	}
}

func TestScoreBuilderSignals(t *testing.T) {
	c := model.Cluster{ID: "c", Items: []model.Item{
		item("a", "openai", 4, now,
			"New SDK release", "The API is now open source with a public benchmark."),
	}}

	got := Score(c, testScoring(), testKeywords(), now)

	want := map[string]bool{"sdk": true, "release": true, "api": true, "open source": true, "benchmark": true}
	for _, s := range got.BuilderSignals {
		delete(want, s)
	}
	if len(want) != 0 {
		t.Errorf("missing builder signals %v, got %v", want, got.BuilderSignals)
	}
	if !got.BuilderRelevant {
		t.Error("BuilderRelevant = false, want true")
	}
}

func TestScoreBuilderRelevantThreshold(t *testing.T) {
	tests := []struct {
		name    string
		excerpt string
		want    bool
	}{
		{"no signals", "A general discussion of the field.", false},
		{"one signal", "The api changed.", false},
		{"two signals", "The api and the sdk changed.", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := model.Cluster{ID: "c", Items: []model.Item{
				item("a", "press", 4, now, "Title", tt.excerpt),
			}}
			if got := Score(c, testScoring(), testKeywords(), now); got.BuilderRelevant != tt.want {
				t.Errorf("BuilderRelevant = %v, want %v (signals %v)",
					got.BuilderRelevant, tt.want, got.BuilderSignals)
			}
		})
	}
}

func TestScoreStructuralSignals(t *testing.T) {
	withRepo := item("a", "github", 3, now, "A tool", "")
	withRepo.RepoURL = "https://github.com/acme/tool"
	withRepo.PaperURL = "https://arxiv.org/abs/2501.00001"

	c := model.Cluster{ID: "c", Items: []model.Item{withRepo}}
	got := Score(c, testScoring(), testKeywords(), now)

	for _, want := range []string{"has_paper_url", "has_repo_url", "source_tier_3"} {
		if !contains(got.BuilderSignals, want) {
			t.Errorf("missing structural signal %q in %v", want, got.BuilderSignals)
		}
	}
	if contains(got.BuilderSignals, "source_tier_1") {
		t.Errorf("source_tier_1 fired on a tier-3 item: %v", got.BuilderSignals)
	}
}

func TestScoreUnknownStructuralSignalIsIgnored(t *testing.T) {
	kw := testKeywords()
	kw.BuilderStructuralSignals = []string{"has_repo_url", "source_tier_bogus", "not_a_signal"}

	c := model.Cluster{ID: "c", Items: []model.Item{item("a", "s", 1, now, "T", "")}}

	// The point is that this does not panic and does not invent a signal.
	if got := Score(c, testScoring(), kw, now); len(got.BuilderSignals) != 0 {
		t.Errorf("BuilderSignals = %v, want none", got.BuilderSignals)
	}
}

func TestScoreHypePenalty(t *testing.T) {
	plain := model.Cluster{ID: "a", Items: []model.Item{
		item("a", "press", 4, now, "Model released", "Details of the release."),
	}}
	hyped := model.Cluster{ID: "b", Items: []model.Item{
		item("b", "press", 4, now, "This shocking model is a game-changer", "Details of the release."),
	}}

	sc, kw := testScoring(), testKeywords()
	clean := Score(plain, sc, kw, now)
	loud := Score(hyped, sc, kw, now)

	if len(loud.HypeSignals) != 2 {
		t.Errorf("HypeSignals = %v, want 2", loud.HypeSignals)
	}
	if loud.Score >= clean.Score {
		t.Errorf("hyped scored %d, not below plain at %d", loud.Score, clean.Score)
	}
}

func TestScoreSignalCaps(t *testing.T) {
	sc := testScoring()
	sc.Caps.BuilderSignalMax = 2
	sc.Weights.Tier = 0 // isolate the builder term

	few := model.Cluster{ID: "a", Items: []model.Item{
		item("a", "s", 4, now, "T", "api sdk"),
	}}
	many := model.Cluster{ID: "b", Items: []model.Item{
		item("b", "s", 4, now, "T", "api sdk release benchmark pricing open source"),
	}}

	kw := testKeywords()
	kw.BuilderStructuralSignals = nil

	lo := Score(few, sc, kw, now)
	hi := Score(many, sc, kw, now)

	if lo.Score != hi.Score {
		t.Errorf("cap not applied: 2 signals scored %d, 6 signals scored %d", lo.Score, hi.Score)
	}
	if len(hi.BuilderSignals) != 6 {
		t.Errorf("BuilderSignals = %v, want all 6 reported even though scoring caps at 2", hi.BuilderSignals)
	}
}

func TestScoreRecencyDecay(t *testing.T) {
	// One half-life of age must halve the raw score.
	fresh := model.Cluster{ID: "a", Items: []model.Item{
		item("a", "openai", 1, now, "Thing", ""),
	}}
	old := model.Cluster{ID: "b", Items: []model.Item{
		item("b", "openai", 1, now.Add(-48*time.Hour), "Thing", ""),
	}}

	sc, kw := testScoring(), testKeywords()
	f := Score(fresh, sc, kw, now)
	o := Score(old, sc, kw, now)

	if math.Abs(o.Decay-0.5) > 1e-9 {
		t.Errorf("Decay at one half-life = %v, want 0.5", o.Decay)
	}
	if math.Abs(o.AgeHours-48) > 1e-9 {
		t.Errorf("AgeHours = %v, want 48", o.AgeHours)
	}
	if o.Score >= f.Score {
		t.Errorf("48h-old story scored %d, not below fresh at %d", o.Score, f.Score)
	}
}

func TestScoreUsesEarliestItemAsStoryDate(t *testing.T) {
	broke := now.Add(-40 * time.Hour)
	c := model.Cluster{ID: "c", Items: []model.Item{
		item("a", "openai", 1, now, "Follow-up coverage", ""),
		item("b", "press", 4, broke, "Original report", ""),
	}}

	// Continuing coverage must not reset the clock: the story is 40h old.
	if got := Score(c, testScoring(), testKeywords(), now); !got.Date.Equal(broke) {
		t.Errorf("Date = %v, want %v", got.Date, broke)
	}
}

func TestScoreFutureDateDoesNotBoost(t *testing.T) {
	// Feeds do publish timestamps slightly in the future. Negative age would
	// make decay exceed 1 and inflate the score.
	c := model.Cluster{ID: "c", Items: []model.Item{
		item("a", "openai", 1, now.Add(6*time.Hour), "Thing", ""),
	}}

	got := Score(c, testScoring(), testKeywords(), now)

	if got.AgeHours != 0 {
		t.Errorf("AgeHours = %v, want 0", got.AgeHours)
	}
	if got.Decay != 1 {
		t.Errorf("Decay = %v, want 1", got.Decay)
	}
}

func TestScoreClampedToRange(t *testing.T) {
	sc := testScoring()
	sc.Weights.Sources = 10000 // force an overflow

	c := model.Cluster{ID: "c", Items: []model.Item{
		item("a", "a", 1, now, "T", ""),
		item("b", "b", 1, now, "T", ""),
	}}
	if got := Score(c, sc, testKeywords(), now); got.Score != 100 {
		t.Errorf("Score = %d, want clamped to 100", got.Score)
	}

	sc = testScoring()
	sc.Weights.Hype = 10000 // force a negative raw
	hyped := model.Cluster{ID: "c", Items: []model.Item{
		item("a", "a", 4, now, "This shocking insane thing", ""),
	}}
	if got := Score(hyped, sc, testKeywords(), now); got.Score != 0 {
		t.Errorf("Score = %d, want clamped to 0", got.Score)
	}
}

func TestTierBoundaries(t *testing.T) {
	sc := testScoring()
	tests := []struct {
		score int
		want  string
	}{
		{100, model.TierMajor},
		{70, model.TierMajor},
		{69, model.TierNotable},
		{40, model.TierNotable},
		{39, model.TierMinor},
		{0, model.TierMinor},
	}
	for _, tt := range tests {
		if got := tierFor(tt.score, sc); got != tt.want {
			t.Errorf("tierFor(%d) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestScoreTopicTags(t *testing.T) {
	c := model.Cluster{ID: "c", Items: []model.Item{
		item("a", "s", 1, now, "New model from the lab",
			"An agent framework built on mcp, with gpt and claude support."),
	}}

	got := Score(c, testScoring(), testKeywords(), now)

	// "agents" matches agent+mcp (2), "models" matches model+gpt+claude (3).
	if len(got.Tags) == 0 {
		t.Fatal("Tags is empty")
	}
	if got.Tags[0] != "models" {
		t.Errorf("Tags[0] = %q, want %q (most matches first)", got.Tags[0], "models")
	}
	if !contains(got.Tags, "agents") {
		t.Errorf("Tags = %v, want to include agents", got.Tags)
	}
	if contains(got.Tags, "research") {
		t.Errorf("Tags = %v, want no research tag", got.Tags)
	}
}

func TestScoreTagsAreCapped(t *testing.T) {
	kw := testKeywords()
	kw.TopicTags = map[string][]string{
		"a": {"alpha"}, "b": {"bravo"}, "c": {"charlie"},
		"d": {"delta"}, "e": {"echo"}, "f": {"foxtrot"},
	}
	c := model.Cluster{ID: "c", Items: []model.Item{
		item("a", "s", 1, now, "alpha bravo charlie", "delta echo foxtrot"),
	}}

	if got := Score(c, testScoring(), kw, now); len(got.Tags) != maxTags {
		t.Errorf("len(Tags) = %d, want %d", len(got.Tags), maxTags)
	}
}

func TestScoreIsDeterministic(t *testing.T) {
	c := model.Cluster{ID: "c", Items: []model.Item{
		item("a", "openai", 1, now, "New model release",
			"An open source agent with an api, an sdk, a paper and a benchmark."),
		item("b", "hn", 5, now.Add(-time.Hour), "Discussion", "Comments about the model."),
	}}
	sc, kw := testScoring(), testKeywords()

	first := Score(c, sc, kw, now)
	for i := 0; i < 20; i++ {
		got := Score(c, sc, kw, now)
		if got.Score != first.Score {
			t.Fatalf("run %d: Score = %d, want %d", i, got.Score, first.Score)
		}
		if !equal(got.Tags, first.Tags) {
			t.Fatalf("run %d: Tags = %v, want %v", i, got.Tags, first.Tags)
		}
		if !equal(got.BuilderSignals, first.BuilderSignals) {
			t.Fatalf("run %d: BuilderSignals = %v, want %v", i, got.BuilderSignals, first.BuilderSignals)
		}
	}
}

func TestRankOrdersByScoreThenDateThenID(t *testing.T) {
	sc, kw := testScoring(), testKeywords()

	high := model.Cluster{ID: "high", Items: []model.Item{
		item("a", "openai", 1, now, "Major release with api and sdk", "open source benchmark"),
		item("b", "verge", 4, now, "Major release", ""),
		item("c", "hn", 5, now, "Discussion", ""),
	}}
	low := model.Cluster{ID: "low", Items: []model.Item{
		item("d", "hn", 5, now.Add(-72*time.Hour), "Old chatter", ""),
	}}

	got := Rank([]model.Cluster{low, high}, sc, kw, now)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ClusterID != "high" {
		t.Errorf("first = %q (score %d), want high", got[0].ClusterID, got[0].Score)
	}
	if got[0].Score < got[1].Score {
		t.Errorf("not sorted descending: %d then %d", got[0].Score, got[1].Score)
	}
}

func TestRankTieBreaksDeterministically(t *testing.T) {
	sc, kw := testScoring(), testKeywords()

	// Identical in every respect except ID.
	mk := func(id string) model.Cluster {
		return model.Cluster{ID: id, Items: []model.Item{
			item(id, "press", 4, now, "Same title", "Same excerpt."),
		}}
	}
	in := []model.Cluster{mk("zulu"), mk("alpha"), mk("mike")}

	got := Rank(in, sc, kw, now)
	want := []string{"alpha", "mike", "zulu"}
	for i, id := range want {
		if got[i].ClusterID != id {
			t.Fatalf("order = %v..., want %v", got[i].ClusterID, want)
		}
	}
}

func TestRankEmpty(t *testing.T) {
	if got := Rank(nil, testScoring(), testKeywords(), now); len(got) != 0 {
		t.Errorf("Rank(nil) = %v, want empty", got)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
