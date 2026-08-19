package normalize

import (
	"strings"
	"testing"
	"time"

	"github.com/papitsho/airfoil/internal/model"
)

var (
	testNow  = time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	testRaw  = Raw{SourceID: "openai", SourceName: "OpenAI", SourceTier: 1, URL: "https://openai.com/news/gpt-5", Title: "GPT-5 ships"}
	maxChars = 300
)

func TestBuild(t *testing.T) {
	r := testRaw
	r.Body = "<p>OpenAI released GPT-5 today.</p>"
	r.Author = "  OpenAI  "
	r.PublishedAt = time.Date(2026, 8, 17, 9, 30, 0, 0, time.UTC)

	got, err := Build(r, testNow, maxChars)
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}

	if want := "https://openai.com/news/gpt-5"; got.URL != want {
		t.Errorf("URL = %q, want %q", got.URL, want)
	}
	if want := ItemID(got.URL); got.ID != want {
		t.Errorf("ID = %q, want %q", got.ID, want)
	}
	if want := "OpenAI released GPT-5 today."; got.Excerpt != want {
		t.Errorf("Excerpt = %q, want %q", got.Excerpt, want)
	}
	if want := "OpenAI"; got.Author != want {
		t.Errorf("Author = %q, want %q", got.Author, want)
	}
	if !got.FetchedAt.Equal(testNow) {
		t.Errorf("FetchedAt = %v, want %v", got.FetchedAt, testNow)
	}
	if !got.PublishedAt.Equal(r.PublishedAt) {
		t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, r.PublishedAt)
	}
	if got.SourceTier != 1 {
		t.Errorf("SourceTier = %d, want 1", got.SourceTier)
	}
}

// R1: Build is the only constructor of an Item, so the cap it applies is the
// cap that reaches disk.
func TestBuildEnforcesExcerptCap(t *testing.T) {
	r := testRaw
	r.Body = strings.Repeat("<p>Long body text that keeps going. </p>", 200)

	got, err := Build(r, testNow, maxChars)
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}
	if n := len([]rune(got.Excerpt)); n > maxChars {
		t.Errorf("Excerpt is %d runes, want <= %d", n, maxChars)
	}
}

func TestBuildTimeHandling(t *testing.T) {
	tests := []struct {
		name      string
		published time.Time
		want      time.Time
	}{
		{"kept as given", testNow.Add(-3 * time.Hour), testNow.Add(-3 * time.Hour)},
		{"missing date falls back to now", time.Time{}, testNow},
		{"future date is clamped to now", testNow.Add(48 * time.Hour), testNow},
		{"converted to utc", time.Date(2026, 8, 17, 9, 0, 0, 0, time.FixedZone("CEST", 2*3600)), testNow.Add(-5 * time.Hour)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRaw
			r.PublishedAt = tt.published

			got, err := Build(r, testNow, maxChars)
			if err != nil {
				t.Fatalf("Build() = %v", err)
			}
			if !got.PublishedAt.Equal(tt.want) {
				t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, tt.want)
			}
			if got.PublishedAt.Location() != time.UTC {
				t.Errorf("PublishedAt location = %v, want UTC", got.PublishedAt.Location())
			}
		})
	}
}

func TestBuildExtractsLinks(t *testing.T) {
	r := testRaw
	r.Body = "Weights at https://github.com/openai/gpt-5 and the paper is arXiv:2401.12345"

	got, err := Build(r, testNow, maxChars)
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}
	if want := "https://github.com/openai/gpt-5"; got.RepoURL != want {
		t.Errorf("RepoURL = %q, want %q", got.RepoURL, want)
	}
	if want := "https://arxiv.org/abs/2401.12345"; got.PaperURL != want {
		t.Errorf("PaperURL = %q, want %q", got.PaperURL, want)
	}
}

// An adapter that already knows the repo or paper must not have it overwritten
// by whatever the body happens to mention.
func TestBuildPrefersAdapterSuppliedLinks(t *testing.T) {
	r := testRaw
	r.RepoURL = "https://github.com/known/repo"
	r.PaperURL = "https://arxiv.org/abs/2000.00001"
	r.Body = "unrelated https://github.com/other/repo and arXiv:2401.12345"

	got, err := Build(r, testNow, maxChars)
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}
	if got.RepoURL != r.RepoURL {
		t.Errorf("RepoURL = %q, want %q", got.RepoURL, r.RepoURL)
	}
	if got.PaperURL != r.PaperURL {
		t.Errorf("PaperURL = %q, want %q", got.PaperURL, r.PaperURL)
	}
}

func TestBuildErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Raw)
	}{
		{"empty url", func(r *Raw) { r.URL = "" }},
		{"unparseable url", func(r *Raw) { r.URL = "ftp://example.com" }},
		{"empty title", func(r *Raw) { r.Title = "" }},
		{"title is only whitespace", func(r *Raw) { r.Title = "   " }},
		{"title is only a separator", func(r *Raw) { r.Title = " | " }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := testRaw
			tt.mutate(&r)

			if _, err := Build(r, testNow, maxChars); err == nil {
				t.Error("Build() = nil, want an error")
			}
		})
	}
}

func TestDedupe(t *testing.T) {
	item := func(id string, tier int) model.Item {
		return model.Item{ID: id, SourceTier: tier, PublishedAt: testNow}
	}

	t.Run("keeps distinct items in order", func(t *testing.T) {
		in := []model.Item{item("a", 1), item("b", 4), item("c", 5)}

		got := Dedupe(in, nil)
		if len(got) != 3 {
			t.Fatalf("got %d items, want 3", len(got))
		}
		for i, want := range []string{"a", "b", "c"} {
			if got[i].ID != want {
				t.Errorf("item %d = %q, want %q", i, got[i].ID, want)
			}
		}
	})

	t.Run("collapses duplicates within a run", func(t *testing.T) {
		got := Dedupe([]model.Item{item("a", 4), item("a", 4), item("b", 4)}, nil)
		if len(got) != 2 {
			t.Fatalf("got %d items, want 2", len(got))
		}
	})

	t.Run("the higher tier source wins a collision", func(t *testing.T) {
		got := Dedupe([]model.Item{item("a", 4), item("a", 1)}, nil)
		if len(got) != 1 {
			t.Fatalf("got %d items, want 1", len(got))
		}
		if got[0].SourceTier != 1 {
			t.Errorf("kept tier %d, want the tier 1 source", got[0].SourceTier)
		}
	})

	t.Run("earlier publication wins within a tier", func(t *testing.T) {
		late, early := item("a", 4), item("a", 4)
		early.PublishedAt = testNow.Add(-2 * time.Hour)

		got := Dedupe([]model.Item{late, early}, nil)
		if len(got) != 1 {
			t.Fatalf("got %d items, want 1", len(got))
		}
		if !got[0].PublishedAt.Equal(early.PublishedAt) {
			t.Errorf("kept %v, want the earlier %v", got[0].PublishedAt, early.PublishedAt)
		}
	})

	// R7: a second run sees everything as already seen and adds nothing.
	t.Run("drops items the state has already seen", func(t *testing.T) {
		in := []model.Item{item("a", 1), item("b", 1)}
		seen := map[string]bool{"a": true, "b": true}

		got := Dedupe(in, func(id string) bool { return seen[id] })
		if len(got) != 0 {
			t.Errorf("got %d items, want 0 on a repeat run", len(got))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		if got := Dedupe(nil, nil); len(got) != 0 {
			t.Errorf("got %d items, want 0", len(got))
		}
	})
}
