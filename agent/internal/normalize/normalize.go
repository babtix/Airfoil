// Package normalize turns raw feed output into canonical Items.
//
// Every function here is pure and table-driven tested (R10). Build is the only
// path that constructs a model.Item, which is what makes R1 structural: the
// excerpt cap is applied here, before anything reaches disk.
package normalize

import (
	"fmt"
	"time"

	"github.com/papitsho/airfoil/internal/model"
)

// Raw is what an ingest adapter produces before normalization. Fields are
// whatever the source gave us — unescaped HTML, tracking URLs, missing dates.
type Raw struct {
	SourceID   string
	SourceName string
	SourceTier int

	URL         string
	Title       string
	Body        string // summary or content HTML from the feed
	Author      string
	PublishedAt time.Time

	Metrics model.Metrics

	// RepoURL and PaperURL may be set by adapters that already know them, such
	// as the GitHub and HuggingFace papers adapters. Otherwise they are
	// extracted from URL, title, and body.
	RepoURL  string
	PaperURL string
}

// Build normalizes a Raw into an Item.
//
// now is the fetch time, used both as FetchedAt and as the fallback publication
// date for feeds that omit one. maxExcerpt is the R1 cap in characters.
func Build(r Raw, now time.Time, maxExcerpt int) (model.Item, error) {
	canonical, err := CanonicalURL(r.URL)
	if err != nil {
		return model.Item{}, fmt.Errorf("normalize: %s: %w", r.SourceID, err)
	}

	title := CleanTitle(r.Title, r.SourceName)
	if title == "" {
		return model.Item{}, fmt.Errorf("normalize: %s: empty title for %s", r.SourceID, canonical)
	}

	// A publication date in the future is a feed bug, and it would win every
	// recency comparison. Clamp it to now.
	published := r.PublishedAt.UTC()
	if published.IsZero() || published.After(now.UTC()) {
		published = now.UTC()
	}

	repo, paper := r.RepoURL, r.PaperURL
	if repo == "" {
		repo = ExtractRepoURL(canonical, r.Title, r.Body)
	}
	if paper == "" {
		paper = ExtractPaperURL(canonical, r.Title, r.Body)
	}

	return model.Item{
		ID:          ItemID(canonical),
		SourceID:    r.SourceID,
		SourceName:  r.SourceName,
		SourceTier:  r.SourceTier,
		URL:         canonical,
		Title:       title,
		Excerpt:     Excerpt(r.Body, maxExcerpt), // R1
		Author:      CollapseSpace(r.Author),
		PublishedAt: published,
		FetchedAt:   now.UTC(),
		Metrics:     r.Metrics,
		RepoURL:     repo,
		PaperURL:    paper,
	}, nil
}

// Dedupe removes items whose ID has already been seen, keeping the first
// occurrence. Within a single run, two feeds carrying the same article collapse
// to one item; across runs, state.SeenURLs does the same job (R7).
//
// The higher-tier item wins a collision, so a lab blog post beats the press
// rewrite of it that happens to share a canonical URL.
func Dedupe(items []model.Item, seen func(id string) bool) []model.Item {
	byID := make(map[string]int, len(items))
	out := make([]model.Item, 0, len(items))

	for _, item := range items {
		if seen != nil && seen(item.ID) {
			continue
		}
		if at, dup := byID[item.ID]; dup {
			if betterDuplicate(item, out[at]) {
				out[at] = item
			}
			continue
		}
		byID[item.ID] = len(out)
		out = append(out, item)
	}
	return out
}

// betterDuplicate reports whether a should replace b. Lower tier number is a
// more authoritative source; ties go to the earlier publication.
func betterDuplicate(a, b model.Item) bool {
	if a.SourceTier != b.SourceTier {
		return a.SourceTier < b.SourceTier
	}
	return a.PublishedAt.Before(b.PublishedAt)
}
