// Package digest generates the daily newsletter, X thread, and LinkedIn draft.
//
// Nothing here sends anything. Every output is written to data/digest/ for a
// human to review and post — the commentary is the reason people follow, so it
// stays human (docs/PROMPTS.md §4).
package digest

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/store"
	"github.com/papitsho/airfoil/internal/summarize"
)

// topStories is how many stories the digest covers, from PROMPTS.md §2.
const topStories = 5

// Generator builds the daily outputs.
type Generator struct {
	llm summarize.Completer
	cfg *config.Config
	log *slog.Logger
}

func New(llm summarize.Completer, cfg *config.Config, log *slog.Logger) *Generator {
	return &Generator{llm: llm, cfg: cfg, log: log}
}

// Intro is the newsletter opening from PROMPTS.md §2.
type Intro struct {
	Intro    string `json:"intro"`
	Theme    string `json:"theme"`
	QuietDay bool   `json:"quiet_day"`
}

// Thread is the X thread from PROMPTS.md §3.
type Thread struct {
	Posts []Post `json:"posts"`
}

// Post is one entry in an X thread.
type Post struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// LinkedIn is the draft from PROMPTS.md §4.
type LinkedIn struct {
	Body     string   `json:"body"`
	Hashtags []string `json:"hashtags"`
	StoryURL string   `json:"story_url"`
}

// Result is everything one digest run produced.
type Result struct {
	Date     time.Time
	Stories  []model.Story
	Intro    Intro
	Thread   Thread
	LinkedIn LinkedIn
	Files    []string
	// Failures names the parts that could not be generated. A digest is a
	// convenience, so a failed part is reported rather than fatal.
	Failures []string
}

// Dir is where digest output lands.
func (g *Generator) Dir() string {
	return filepath.Join(g.cfg.DataDir, "digest")
}

// Run generates all three outputs for the given day.
func (g *Generator) Run(ctx context.Context, day time.Time, dry bool) (Result, error) {
	res := Result{Date: day}

	stories, err := store.ReadJSONOr(filepath.Join(g.cfg.DataDir, "stories.json"), []model.Story(nil))
	if err != nil {
		return res, fmt.Errorf("digest: %w", err)
	}

	res.Stories = topFor(stories, day, topStories)
	if len(res.Stories) == 0 {
		return res, fmt.Errorf("digest: no stories for %s — run write first",
			day.Format(time.DateOnly))
	}

	// Each part is generated independently so one failure does not lose the
	// others.
	if intro, err := g.intro(ctx, res.Stories, day); err != nil {
		res.Failures = append(res.Failures, "intro: "+err.Error())
		g.log.Warn("digest intro failed", "error", err)
	} else {
		res.Intro = intro
	}

	if thread, err := g.thread(ctx, res.Stories); err != nil {
		res.Failures = append(res.Failures, "thread: "+err.Error())
		g.log.Warn("digest thread failed", "error", err)
	} else {
		res.Thread = thread
	}

	if li, err := g.linkedIn(ctx, res.Stories[0]); err != nil {
		res.Failures = append(res.Failures, "linkedin: "+err.Error())
		g.log.Warn("digest linkedin failed", "error", err)
	} else {
		res.LinkedIn = li
	}

	if dry {
		g.log.Info("dry run: digest not written", "stories", len(res.Stories))
		return res, nil
	}

	files, err := g.write(day, res)
	if err != nil {
		return res, err
	}
	res.Files = files

	g.log.Info("wrote digest", "day", day.Format(time.DateOnly),
		"files", len(files), "failures", len(res.Failures))
	return res, nil
}

// topFor returns the day's highest-scoring stories that carry prose. A digest
// entry needs a summary to be worth reading.
func topFor(stories []model.Story, day time.Time, n int) []model.Story {
	target := day.UTC().Format(time.DateOnly)

	var out []model.Story
	for _, s := range stories {
		if s.Summary == "" {
			continue
		}
		if s.Date.UTC().Format(time.DateOnly) != target {
			continue
		}
		out = append(out, s)
		if len(out) == n {
			break
		}
	}
	return out
}

// storyLines renders the shared story block used by several prompts.
func storyLines(stories []model.Story) string {
	var b strings.Builder
	for i, s := range stories {
		fmt.Fprintf(&b, "%d. [%d] %s\n   %s\n", i+1, s.Score, s.Title, s.Summary)
	}
	return b.String()
}

// primaryURL is the highest-tier source on a story — the one worth linking.
func primaryURL(s model.Story) string {
	best := ""
	bestTier := 99
	for _, src := range s.Sources {
		if src.Tier < bestTier {
			bestTier, best = src.Tier, src.URL
		}
	}
	return best
}
