package publish

import (
	"strings"
	"testing"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
)

var now = time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

func testScoring() config.Scoring {
	var s config.Scoring
	s.Limits.SummaryMaxWords = 80
	s.Retention.ItemsDays = 30
	s.Retention.SeenURLsDays = 30
	return s
}

func goodStory() model.Story {
	return model.Story{
		ID:      "abc123",
		Slug:    "2026-08-17-a-story",
		Title:   "A story",
		Summary: "A short summary.",
		Score:   50,
		Tier:    model.TierNotable,
		Date:    now,
		Sources: []model.StorySource{{Name: "Blog", URL: "https://a.test/1", Tier: 1}},
	}
}

func indexFor(stories []model.Story) model.Index {
	idx := model.Index{GeneratedAt: now}
	for _, s := range stories {
		idx.Stories = append(idx.Stories, model.IndexEntry{ID: s.ID, Slug: s.Slug})
	}
	return idx
}

func TestValidateAcceptsGoodOutput(t *testing.T) {
	stories := []model.Story{goodStory()}
	if got := Validate(stories, indexFor(stories), testScoring()); len(got) != 0 {
		t.Errorf("Validate() = %v, want none", got)
	}
}

func TestValidateCatchesProblems(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*model.Story)
		want string
	}{
		{"no sources", func(s *model.Story) { s.Sources = nil }, "no sources"},
		{"source without url", func(s *model.Story) { s.Sources[0].URL = "" }, "no URL"},
		{"empty title", func(s *model.Story) { s.Title = "   " }, "empty title"},
		{"missing slug", func(s *model.Story) { s.Slug = "" }, "missing slug"},
		{"missing id", func(s *model.Story) { s.ID = "" }, "missing id"},
		{"zero date", func(s *model.Story) { s.Date = time.Time{} }, "zero date"},
		{"bad tier", func(s *model.Story) { s.Tier = "enormous" }, "invalid tier"},
		{"negative score", func(s *model.Story) { s.Score = -1 }, "outside 0-100"},
		{"score over 100", func(s *model.Story) { s.Score = 101 }, "outside 0-100"},
		{
			"summary over the word cap",
			func(s *model.Story) { s.Summary = strings.Repeat("word ", 81) },
			"81 words",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := goodStory()
			tt.mut(&s)
			stories := []model.Story{s}

			got := Validate(stories, indexFor(stories), testScoring())
			if len(got) == 0 {
				t.Fatalf("Validate() found nothing, want a problem mentioning %q", tt.want)
			}
			joined := ""
			for _, p := range got {
				joined += p.String() + "\n"
			}
			if !strings.Contains(joined, tt.want) {
				t.Errorf("problems = %s, want one mentioning %q", joined, tt.want)
			}
		})
	}
}

func TestValidateCatchesDuplicateSlug(t *testing.T) {
	// Two stories with one slug means one markdown file silently overwrote
	// the other — the site would be missing a story with no error anywhere.
	a, b := goodStory(), goodStory()
	b.ID = "different"
	stories := []model.Story{a, b}

	got := Validate(stories, indexFor(stories), testScoring())
	if len(got) == 0 {
		t.Fatal("duplicate slug not caught")
	}
	if !strings.Contains(got[0].String(), "duplicate slug") {
		t.Errorf("problems = %v, want a duplicate slug report", got)
	}
}

func TestValidateCatchesDuplicateID(t *testing.T) {
	a, b := goodStory(), goodStory()
	b.Slug = "2026-08-17-other"
	stories := []model.Story{a, b}

	got := Validate(stories, indexFor(stories), testScoring())
	if len(got) == 0 || !strings.Contains(got[0].String(), "duplicate id") {
		t.Errorf("problems = %v, want a duplicate id report", got)
	}
}

func TestValidateCatchesIndexDrift(t *testing.T) {
	stories := []model.Story{goodStory()}
	idx := model.Index{GeneratedAt: now} // index built with no entries

	got := Validate(stories, idx, testScoring())
	if len(got) == 0 {
		t.Fatal("index/stories length mismatch not caught")
	}
}

func TestValidateCatchesMissingGeneratedAt(t *testing.T) {
	stories := []model.Story{goodStory()}
	idx := indexFor(stories)
	idx.GeneratedAt = time.Time{}

	got := Validate(stories, idx, testScoring())
	if len(got) == 0 || !strings.Contains(got[0].String(), "generated_at") {
		t.Errorf("problems = %v, want a generated_at report", got)
	}
}

func TestValidateReportsEveryProblem(t *testing.T) {
	// One publish attempt should list everything that needs fixing.
	s := goodStory()
	s.Sources = nil
	s.Title = ""
	s.Tier = "bogus"

	stories := []model.Story{s}
	if got := Validate(stories, indexFor(stories), testScoring()); len(got) < 3 {
		t.Errorf("Validate() = %v, want at least 3 problems", got)
	}
}

func TestValidateAllowsEmptySummary(t *testing.T) {
	// Minor stories are index entries with no prose; that is not a defect.
	s := goodStory()
	s.Summary = ""
	s.Tier = model.TierMinor

	stories := []model.Story{s}
	if got := Validate(stories, indexFor(stories), testScoring()); len(got) != 0 {
		t.Errorf("Validate() = %v, want none", got)
	}
}

func TestPruneDropsOldStories(t *testing.T) {
	fresh := goodStory()
	old := goodStory()
	old.Slug = "2026-06-01-old"
	old.Date = now.AddDate(0, 0, -45)

	state := model.NewState()
	kept, dropped, _ := Prune([]model.Story{fresh, old}, state, testScoring(), now)

	if dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}
	if len(kept) != 1 || kept[0].Slug != fresh.Slug {
		t.Errorf("kept = %v, want only the fresh story", kept)
	}
}

func TestPruneDropsOldSeenURLs(t *testing.T) {
	state := model.NewState()
	state.SeenURLs["recent"] = now.Format(time.DateOnly)
	state.SeenURLs["ancient"] = now.AddDate(0, 0, -60).Format(time.DateOnly)

	_, _, dropped := Prune(nil, state, testScoring(), now)

	if dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}
	if _, ok := state.SeenURLs["recent"]; !ok {
		t.Error("the recent URL was pruned")
	}
	if _, ok := state.SeenURLs["ancient"]; ok {
		t.Error("the ancient URL survived")
	}
}

func TestPruneDropsCorruptSeenURLDates(t *testing.T) {
	// An unparseable date can never expire, so it would leak forever.
	state := model.NewState()
	state.SeenURLs["corrupt"] = "not-a-date"

	_, _, dropped := Prune(nil, state, testScoring(), now)

	if dropped != 1 {
		t.Errorf("dropped = %d, want 1", dropped)
	}
	if len(state.SeenURLs) != 0 {
		t.Errorf("SeenURLs = %v, want empty", state.SeenURLs)
	}
}
