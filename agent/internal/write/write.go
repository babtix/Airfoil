// Package write turns summarized clusters into the three published outputs:
// the canonical markdown files, data/index.json, and data/stories.json.
package write

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/store"
	"github.com/papitsho/airfoil/internal/summarize"
)

// Writer produces the published artefacts for one run.
type Writer struct {
	cfg *config.Config
	log *slog.Logger
}

func New(cfg *config.Config, log *slog.Logger) *Writer {
	return &Writer{cfg: cfg, log: log}
}

// Stats reports what one write pass did.
type Stats struct {
	Created   int // new stories with a markdown file
	Updated   int // existing stories that gained sources or score
	Indexed   int // total entries in index.json
	Unchanged int
	Files     []string
}

// StoriesPath is the authoritative record every other output derives from.
func (w *Writer) StoriesPath() string {
	return filepath.Join(w.cfg.DataDir, "stories.json")
}

// IndexPath is the slim feed the site reads.
func (w *Writer) IndexPath() string {
	return filepath.Join(w.cfg.DataDir, "index.json")
}

// Run merges this run's results into the published set and writes all three
// outputs.
//
// Existing stories are never rewritten: a story that gains coverage keeps its
// title, summary, slug, and date, and only its score, tier, sources and
// cluster size move. That keeps published URLs and text stable for readers and
// search engines, per BUILD_SPEC §5.7.
func (w *Writer) Run(results []summarize.Result, now time.Time, dry bool) (Stats, error) {
	var stats Stats

	published, err := store.ReadJSONOr(w.StoriesPath(), []model.Story(nil))
	if err != nil {
		return stats, fmt.Errorf("write: %w", err)
	}

	// Index existing stories by every source URL they already carry, so a
	// cluster is recognised even when its lead item — and therefore its
	// cluster ID — has changed since the last run.
	byURL := map[string]int{}
	slugs := map[string]bool{}
	for i, s := range published {
		slugs[s.Slug] = true
		for _, src := range s.Sources {
			byURL[src.URL] = i
		}
	}

	for _, res := range results {
		if idx, ok := w.findExisting(res, byURL); ok {
			if w.merge(&published[idx], res) {
				stats.Updated++
			} else {
				stats.Unchanged++
			}
			continue
		}

		// A cluster with no usable summary becomes an index entry only. It is
		// still recorded so the site can list it and so a later run can pick
		// it up once it scores high enough to earn a summary.
		story := w.build(res, slugs)
		slugs[story.Slug] = true
		for _, src := range story.Sources {
			byURL[src.URL] = len(published)
		}
		published = append(published, story)

		if res.Summarized {
			stats.Created++
		}
	}

	sort.SliceStable(published, func(i, j int) bool {
		if published[i].Score != published[j].Score {
			return published[i].Score > published[j].Score
		}
		return published[i].Date.After(published[j].Date)
	})

	stats.Indexed = len(published)
	if dry {
		w.log.Info("dry run: nothing written",
			"would_create", stats.Created, "would_update", stats.Updated)
		return stats, nil
	}

	if err := store.WriteJSON(w.StoriesPath(), published); err != nil {
		return stats, fmt.Errorf("write stories: %w", err)
	}
	if err := store.WriteJSON(w.IndexPath(), buildIndex(published, now)); err != nil {
		return stats, fmt.Errorf("write index: %w", err)
	}

	files, err := w.writeMarkdown(published)
	if err != nil {
		return stats, err
	}
	stats.Files = files

	w.log.Info("wrote stories",
		"created", stats.Created, "updated", stats.Updated,
		"indexed", stats.Indexed, "markdown_files", len(files))
	return stats, nil
}

// findExisting locates a published story sharing any source URL with this
// cluster.
func (w *Writer) findExisting(res summarize.Result, byURL map[string]int) (int, bool) {
	for _, item := range res.Cluster.Items {
		if idx, ok := byURL[item.URL]; ok {
			return idx, true
		}
	}
	return 0, false
}

// merge folds new coverage into a published story. It reports whether anything
// changed. Title, summary, slug and date are deliberately untouched.
func (w *Writer) merge(s *model.Story, res summarize.Result) bool {
	changed := false

	have := map[string]bool{}
	for _, src := range s.Sources {
		have[src.URL] = true
	}
	for _, item := range res.Cluster.Items {
		if have[item.URL] {
			continue
		}
		s.Sources = append(s.Sources, storySource(item))
		have[item.URL] = true
		changed = true
	}

	if s.Score != res.Score.Score {
		s.Score = res.Score.Score
		changed = true
	}
	if tier := effectiveTier(res); s.Tier != tier {
		s.Tier = tier
		changed = true
	}
	if n := len(res.Cluster.Items); n > s.ClusterSize {
		s.ClusterSize = n
		changed = true
	}

	// A story that was an index entry can gain prose on a later run, once it
	// scores high enough to earn a summary. That is the one case where the
	// summary field is allowed to change — from empty to written.
	if res.Summarized && s.Summary == "" {
		s.Summary = res.Summary.Summary
		s.Takeaways = res.Summary.Takeaways
		if len(res.Summary.Tags) > 0 {
			s.Tags = res.Summary.Tags
		}
		changed = true
	}

	return changed
}

// build creates a new story from a fresh cluster.
func (w *Writer) build(res summarize.Result, slugs map[string]bool) model.Story {
	lead := res.Cluster.Items[0]

	title := lead.Title
	if res.Summarized && res.Summary.Title != "" {
		title = res.Summary.Title
	}

	date := res.Score.Date
	if date.IsZero() {
		date = lead.PublishedAt
	}

	story := model.Story{
		ID:              res.Cluster.ID,
		Slug:            DedupeSlug(Slug(date, title), func(s string) bool { return slugs[s] }),
		Title:           title,
		Score:           res.Score.Score,
		Tier:            effectiveTier(res),
		Tags:            res.Score.Tags,
		BuilderRelevant: res.Score.BuilderRelevant,
		Date:            date,
		ClusterSize:     len(res.Cluster.Items),
		// Non-nil so it marshals as [] rather than null — the site types Body
		// as string[] and maps over it.
		Body: []string{},
	}

	if res.Summarized {
		story.Summary = res.Summary.Summary
		story.Takeaways = res.Summary.Takeaways
		if len(res.Summary.Tags) > 0 {
			story.Tags = res.Summary.Tags
		}
		story.BuilderRelevant = res.Summary.BuilderRelevant || res.Score.BuilderRelevant
	}

	// R3: every source the cluster drew from is linked.
	for _, item := range res.Cluster.Items {
		story.Sources = append(story.Sources, storySource(item))
	}

	return story
}

// effectiveTier applies the low-confidence demotion from PROMPTS.md.
func effectiveTier(res summarize.Result) string {
	tier := res.Score.Tier
	if res.Summarized && res.Summary.Confidence == summarize.ConfidenceLow {
		return summarize.DemoteTier(tier)
	}
	return tier
}

func storySource(item model.Item) model.StorySource {
	src := model.StorySource{
		Name: item.SourceName,
		URL:  item.URL,
		Tier: item.SourceTier,
		Type: model.SourceType(item.SourceTier),
	}
	if m := item.Metrics; m != (model.Metrics{}) {
		src.Metrics = &model.SourceMetrics{
			Points:   m.HNPoints,
			Comments: m.HNComments,
			Score:    m.RedditScore,
			Upvotes:  m.HFUpvotes,
			Stars:    m.GitHubStars,
		}
	}
	return src
}

func buildIndex(stories []model.Story, now time.Time) model.Index {
	idx := model.Index{
		GeneratedAt: now.UTC(),
		Stories:     make([]model.IndexEntry, 0, len(stories)),
	}
	for _, s := range stories {
		idx.Stories = append(idx.Stories, model.IndexEntry{
			ID:              s.ID,
			Slug:            s.Slug,
			Title:           s.Title,
			Summary:         s.Summary,
			Score:           s.Score,
			Tier:            s.Tier,
			Tags:            s.Tags,
			BuilderRelevant: s.BuilderRelevant,
			Date:            s.Date,
			ClusterSize:     s.ClusterSize,
		})
	}
	return idx
}
