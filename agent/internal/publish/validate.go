// Package publish validates the generated output and commits it.
//
// Validation runs before anything is staged: a run that produced broken output
// must not reach the site (R9).
package publish

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/normalize"
)

// Problem is one validation failure, named by the story it belongs to.
type Problem struct {
	Slug   string
	Detail string
}

func (p Problem) String() string {
	if p.Slug == "" {
		return p.Detail
	}
	return p.Slug + ": " + p.Detail
}

// Validate checks every published story against the rules that must hold
// before the site is allowed to change.
//
// It returns every problem rather than the first, so one publish attempt tells
// you everything that needs fixing.
func Validate(stories []model.Story, idx model.Index, sc config.Scoring) []Problem {
	var problems []Problem
	add := func(slug, format string, args ...any) {
		problems = append(problems, Problem{Slug: slug, Detail: fmt.Sprintf(format, args...)})
	}

	if idx.GeneratedAt.IsZero() {
		add("", "index.json has no generated_at timestamp")
	}
	if len(stories) != len(idx.Stories) {
		add("", "stories.json has %d entries but index.json has %d",
			len(stories), len(idx.Stories))
	}

	seenSlug := map[string]bool{}
	seenID := map[string]bool{}

	for _, s := range stories {
		switch {
		case s.Slug == "":
			add(s.ID, "missing slug")
		case seenSlug[s.Slug]:
			// Two stories with one slug means one markdown file silently
			// overwrote the other.
			add(s.Slug, "duplicate slug")
		default:
			seenSlug[s.Slug] = true
		}

		if s.ID == "" {
			add(s.Slug, "missing id")
		} else if seenID[s.ID] {
			add(s.Slug, "duplicate id %s", s.ID)
		} else {
			seenID[s.ID] = true
		}

		if strings.TrimSpace(s.Title) == "" {
			add(s.Slug, "empty title")
		}
		if s.Date.IsZero() {
			add(s.Slug, "zero date")
		}

		// R3: a story with no source is unattributable and must never ship.
		if len(s.Sources) == 0 {
			add(s.Slug, "no sources")
		}
		for _, src := range s.Sources {
			if src.URL == "" {
				add(s.Slug, "source %q has no URL", src.Name)
			}
		}

		switch s.Tier {
		case model.TierMajor, model.TierNotable, model.TierMinor:
		default:
			add(s.Slug, "invalid tier %q", s.Tier)
		}

		if s.Score < 0 || s.Score > 100 {
			add(s.Slug, "score %d outside 0-100", s.Score)
		}

		// R1: the word cap is re-checked here, not trusted from the writer.
		// This is the last gate before the text is public.
		if s.Summary != "" {
			if n := normalize.WordCount(s.Summary); n > sc.Limits.SummaryMaxWords {
				add(s.Slug, "summary is %d words, limit %d", n, sc.Limits.SummaryMaxWords)
			}
		}
	}

	return problems
}

// Prune drops stories and seen-URL entries past the retention window.
//
// Without this, state.json grows without bound and every run re-embeds a
// longer tail of dead items.
func Prune(stories []model.Story, state *model.State, sc config.Scoring, now time.Time) (kept []model.Story, droppedStories, droppedURLs int) {
	cutoff := now.AddDate(0, 0, -sc.Retention.ItemsDays)

	kept = make([]model.Story, 0, len(stories))
	for _, s := range stories {
		if s.Date.Before(cutoff) {
			droppedStories++
			continue
		}
		kept = append(kept, s)
	}

	urlCutoff := now.AddDate(0, 0, -sc.Retention.SeenURLsDays)
	for id, day := range state.SeenURLs {
		seen, err := time.Parse(time.DateOnly, day)
		if err != nil {
			// An unparseable date is corrupt state; drop it rather than keep
			// an entry that can never expire.
			delete(state.SeenURLs, id)
			droppedURLs++
			continue
		}
		if seen.Before(urlCutoff) {
			delete(state.SeenURLs, id)
			droppedURLs++
		}
	}

	return kept, droppedStories, droppedURLs
}

// ValidateStoryFiles reports markdown pages on disk that no published story
// points at.
//
// The writer never deletes files, and a story's slug is derived from its title.
// So if stories.json is ever rebuilt — or a title changes before its slug is
// frozen — the old page stays behind, referenced by nothing. Committing it
// publishes a dead URL that the site's own data does not know about.
//
// Orphans are reported rather than deleted: R9's job is to stop the run and say
// what is wrong, and silently removing files the operator did not ask about is
// not a safe thing for a scheduled job to do.
func ValidateStoryFiles(stories []model.Story, storiesDir string) []Problem {
	entries, err := os.ReadDir(storiesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing written yet is not a failure
		}
		return []Problem{{Detail: fmt.Sprintf("cannot read %s: %v", storiesDir, err)}}
	}

	// Every slug a story claims, whether or not it carries prose.
	claimed := make(map[string]bool, len(stories))
	for _, s := range stories {
		claimed[s.Slug] = true
	}

	var problems []Problem
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") {
			continue
		}
		if slug := strings.TrimSuffix(name, ".md"); !claimed[slug] {
			problems = append(problems, Problem{
				Slug:   slug,
				Detail: "orphaned page — no story in stories.json points at it",
			})
		}
	}
	return problems
}
