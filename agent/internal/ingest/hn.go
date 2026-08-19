package ingest

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/normalize"
)

// hnAdapter queries the Hacker News Algolia API, once per configured keyword.
//
// Hacker News is a tier 5 signal: it rarely breaks a story, but the points on a
// submission are the best available proxy for whether engineers care.
type hnAdapter struct {
	fetch *fetcher
}

func newHNAdapter(f *fetcher) *hnAdapter { return &hnAdapter{fetch: f} }

func (a *hnAdapter) Type() string { return config.SourceHN }

type hnResponse struct {
	Hits []hnHit `json:"hits"`
}

type hnHit struct {
	ObjectID    string `json:"objectID"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	StoryText   string `json:"story_text"`
	Points      int    `json:"points"`
	NumComments int    `json:"num_comments"`
	CreatedAtI  int64  `json:"created_at_i"`
}

func (a *hnAdapter) Fetch(ctx context.Context, src config.Source) ([]normalize.Raw, error) {
	opts := src.Options
	if len(opts.Queries) == 0 {
		return nil, fmt.Errorf("hn: no queries configured for %s", src.ID)
	}

	hoursBack := opts.HoursBack
	if hoursBack <= 0 {
		hoursBack = 48
	}
	after := time.Now().Add(-time.Duration(hoursBack) * time.Hour).Unix()

	// One submission often matches several keywords.
	seen := make(map[string]bool)
	var out []normalize.Raw

	for _, query := range opts.Queries {
		hits, err := a.search(ctx, src.URL, query, opts.MinPoints, after)
		if err != nil {
			return nil, err
		}
		for _, hit := range hits {
			if seen[hit.ObjectID] {
				continue
			}
			seen[hit.ObjectID] = true

			raw, ok := hnRaw(hit, src)
			if !ok {
				continue
			}
			out = append(out, raw)
		}
	}
	return out, nil
}

func (a *hnAdapter) search(ctx context.Context, endpoint, query string, minPoints int, after int64) ([]hnHit, error) {
	filters := fmt.Sprintf("created_at_i>%d", after)
	if minPoints > 0 {
		filters += ",points>" + strconv.Itoa(minPoints)
	}

	q := url.Values{}
	q.Set("query", query)
	q.Set("tags", "story")
	q.Set("numericFilters", filters)
	q.Set("hitsPerPage", "50")

	var resp hnResponse
	if err := a.fetch.getJSON(ctx, endpoint+"?"+q.Encode(), nil, &resp); err != nil {
		return nil, fmt.Errorf("hn %q: %w", query, err)
	}
	return resp.Hits, nil
}

// hnRaw converts a hit. Submissions with no outbound link (Ask HN, and text
// posts) point at the discussion itself.
func hnRaw(hit hnHit, src config.Source) (normalize.Raw, bool) {
	if hit.Title == "" {
		return normalize.Raw{}, false
	}

	link := hit.URL
	if link == "" {
		if hit.ObjectID == "" {
			return normalize.Raw{}, false
		}
		link = "https://news.ycombinator.com/item?id=" + hit.ObjectID
	}

	return normalize.Raw{
		SourceID:    src.ID,
		SourceName:  src.Name,
		SourceTier:  src.Tier,
		URL:         link,
		Title:       hit.Title,
		Body:        hit.StoryText,
		Author:      hit.Author,
		PublishedAt: time.Unix(hit.CreatedAtI, 0).UTC(),
		Metrics: model.Metrics{
			HNPoints:   hit.Points,
			HNComments: hit.NumComments,
		},
	}, true
}
