package write

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/score"
	"github.com/papitsho/airfoil/internal/store"
	"github.com/papitsho/airfoil/internal/summarize"
)

var now = time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

func testWriter(t *testing.T) *Writer {
	t.Helper()
	dir := t.TempDir()
	cfg := &config.Config{
		DataDir:    dir,
		StoriesDir: filepath.Join(dir, "stories"),
	}
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func item(id, source, url, title string, tier int) model.Item {
	return model.Item{
		ID: id, SourceID: source, SourceName: source, SourceTier: tier,
		URL: url, Title: title, PublishedAt: now,
	}
}

func result(clusterID string, sc int, summarized bool, items ...model.Item) summarize.Result {
	r := summarize.Result{
		Cluster: model.Cluster{ID: clusterID, Items: items},
		Score: score.Result{
			ClusterID: clusterID, Score: sc, Tier: model.TierMajor,
			Date: now, Tags: []string{"models"},
		},
		Summarized: summarized,
	}
	if summarized {
		r.Summary = summarize.Response{
			Title:      "Summarized title",
			Summary:    "An original summary of the event.",
			Takeaways:  []string{"one", "two"},
			Tags:       []string{"models"},
			Confidence: summarize.ConfidenceHigh,
		}
	}
	return r
}

func TestRunCreatesStoryAndMarkdown(t *testing.T) {
	w := testWriter(t)
	res := result("c1", 80, true, item("i1", "openai", "https://a.test/1", "Raw title", 1))

	stats, err := w.Run([]summarize.Result{res}, now, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stats.Created != 1 {
		t.Errorf("Created = %d, want 1", stats.Created)
	}
	if len(stats.Files) != 1 {
		t.Fatalf("Files = %v, want 1", stats.Files)
	}

	body, err := os.ReadFile(stats.Files[0])
	if err != nil {
		t.Fatalf("reading markdown: %v", err)
	}
	md := string(body)
	for _, want := range []string{
		`title: "Summarized title"`,
		`summary: "An original summary of the event."`,
		"score: 80",
		`url: "https://a.test/1"`,
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n%s", want, md)
		}
	}
}

func TestRunSkipsMarkdownForUnsummarizedStory(t *testing.T) {
	w := testWriter(t)
	res := result("c1", 20, false, item("i1", "hn", "https://a.test/1", "Raw title", 5))

	stats, err := w.Run([]summarize.Result{res}, now, false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(stats.Files) != 0 {
		t.Errorf("Files = %v, want none — an index entry has no prose to render", stats.Files)
	}
	if stats.Indexed != 1 {
		t.Errorf("Indexed = %d, want 1 — it still belongs in the index", stats.Indexed)
	}
}

func TestRunNeverRewritesTitleOrSummary(t *testing.T) {
	w := testWriter(t)
	first := result("c1", 80, true, item("i1", "openai", "https://a.test/1", "Raw", 1))
	if _, err := w.Run([]summarize.Result{first}, now, false); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}

	before, err := store.ReadJSONOr(w.StoriesPath(), []model.Story(nil))
	if err != nil {
		t.Fatal(err)
	}

	// The same story on a later run, with a different summary and a new source.
	second := result("c2", 95, true,
		item("i1", "openai", "https://a.test/1", "Raw", 1),
		item("i2", "verge", "https://b.test/2", "Coverage", 4))
	second.Summary.Title = "A COMPLETELY DIFFERENT TITLE"
	second.Summary.Summary = "A completely different summary."

	stats, err := w.Run([]summarize.Result{second}, now, false)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if stats.Updated != 1 {
		t.Errorf("Updated = %d, want 1", stats.Updated)
	}

	after, err := store.ReadJSONOr(w.StoriesPath(), []model.Story(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 {
		t.Fatalf("stories = %d, want 1 — the second run must merge, not duplicate", len(after))
	}

	got := after[0]
	if got.Title != before[0].Title {
		t.Errorf("Title changed to %q, want frozen at %q", got.Title, before[0].Title)
	}
	if got.Summary != before[0].Summary {
		t.Errorf("Summary changed to %q, want frozen", got.Summary)
	}
	if got.Slug != before[0].Slug {
		t.Errorf("Slug changed to %q, want frozen at %q — URLs must stay stable",
			got.Slug, before[0].Slug)
	}

	// Score and sources are the fields that are allowed to move.
	if got.Score != 95 {
		t.Errorf("Score = %d, want 95", got.Score)
	}
	if len(got.Sources) != 2 {
		t.Errorf("Sources = %d, want 2 (R3: new coverage is linked)", len(got.Sources))
	}
}

func TestRunMatchesExistingStoryDespiteChangedClusterID(t *testing.T) {
	// Cluster IDs derive from the lead item, so a higher-tier source joining
	// changes the ID. Matching must survive that.
	w := testWriter(t)

	first := result("old-id", 50, true, item("i1", "hn", "https://a.test/1", "HN thread", 5))
	if _, err := w.Run([]summarize.Result{first}, now, false); err != nil {
		t.Fatal(err)
	}

	second := result("brand-new-id", 70, true,
		item("i2", "openai", "https://b.test/2", "Official post", 1),
		item("i1", "hn", "https://a.test/1", "HN thread", 5))

	if _, err := w.Run([]summarize.Result{second}, now, false); err != nil {
		t.Fatal(err)
	}

	stories, err := store.ReadJSONOr(w.StoriesPath(), []model.Story(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(stories) != 1 {
		t.Fatalf("stories = %d, want 1 — a changed cluster ID must not fork the story", len(stories))
	}
}

func TestRunGivesSummaryToPreviouslyIndexOnlyStory(t *testing.T) {
	w := testWriter(t)

	// First run: too minor to summarize.
	first := result("c1", 20, false, item("i1", "hn", "https://a.test/1", "Thread", 5))
	if _, err := w.Run([]summarize.Result{first}, now, false); err != nil {
		t.Fatal(err)
	}

	// Second run: it picked up coverage and now earns prose.
	second := result("c1", 75, true, item("i1", "hn", "https://a.test/1", "Thread", 5))
	if _, err := w.Run([]summarize.Result{second}, now, false); err != nil {
		t.Fatal(err)
	}

	stories, _ := store.ReadJSONOr(w.StoriesPath(), []model.Story(nil))
	if len(stories) != 1 {
		t.Fatalf("stories = %d, want 1", len(stories))
	}
	if stories[0].Summary == "" {
		t.Error("story never gained a summary despite being summarized on the second run")
	}
}

func TestRunDryWritesNothing(t *testing.T) {
	w := testWriter(t)
	res := result("c1", 80, true, item("i1", "openai", "https://a.test/1", "Raw", 1))

	if _, err := w.Run([]summarize.Result{res}, now, true); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if store.Exists(w.StoriesPath()) {
		t.Error("stories.json was written during a dry run")
	}
	if store.Exists(w.IndexPath()) {
		t.Error("index.json was written during a dry run")
	}
}

func TestRunDemotesLowConfidence(t *testing.T) {
	w := testWriter(t)
	res := result("c1", 95, true, item("i1", "openai", "https://a.test/1", "Raw", 1))
	res.Score.Tier = model.TierMajor
	res.Summary.Confidence = summarize.ConfidenceLow

	if _, err := w.Run([]summarize.Result{res}, now, false); err != nil {
		t.Fatal(err)
	}

	stories, _ := store.ReadJSONOr(w.StoriesPath(), []model.Story(nil))
	if stories[0].Tier != model.TierNotable {
		t.Errorf("Tier = %q, want %q — low confidence demotes one level",
			stories[0].Tier, model.TierNotable)
	}
}

func TestRunDedupesSlugsAcrossDifferentStories(t *testing.T) {
	w := testWriter(t)

	a := result("c1", 80, true, item("i1", "openai", "https://a.test/1", "Same Title", 1))
	b := result("c2", 70, true, item("i2", "verge", "https://b.test/2", "Same Title", 4))
	a.Summary.Title = "Same Title"
	b.Summary.Title = "Same Title"

	if _, err := w.Run([]summarize.Result{a, b}, now, false); err != nil {
		t.Fatal(err)
	}

	stories, _ := store.ReadJSONOr(w.StoriesPath(), []model.Story(nil))
	if len(stories) != 2 {
		t.Fatalf("stories = %d, want 2", len(stories))
	}
	if stories[0].Slug == stories[1].Slug {
		t.Errorf("both stories got slug %q — they would overwrite each other", stories[0].Slug)
	}
}

func TestRunWritesIndexEntriesForEveryStory(t *testing.T) {
	w := testWriter(t)
	results := []summarize.Result{
		result("c1", 80, true, item("i1", "openai", "https://a.test/1", "Major", 1)),
		result("c2", 15, false, item("i2", "hn", "https://b.test/2", "Minor", 5)),
	}
	if _, err := w.Run(results, now, false); err != nil {
		t.Fatal(err)
	}

	idx, err := store.ReadJSON[model.Index](w.IndexPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Stories) != 2 {
		t.Errorf("index has %d stories, want 2", len(idx.Stories))
	}
	if idx.GeneratedAt.IsZero() {
		t.Error("index.generated_at is zero")
	}
}
