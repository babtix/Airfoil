package ingest

import (
	"context"
	"fmt"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/normalize"
)

// hfAdapter reads the HuggingFace daily papers list.
//
// Every entry carries an arXiv identifier, so the paper URL is known up front
// rather than extracted — which means two outlets covering the same paper are
// forced into one cluster even if their titles differ.
type hfAdapter struct {
	fetch *fetcher
}

func newHFAdapter(f *fetcher) *hfAdapter { return &hfAdapter{fetch: f} }

func (a *hfAdapter) Type() string { return config.SourceHFPapers }

type hfEntry struct {
	Paper struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Summary     string `json:"summary"`
		Upvotes     int    `json:"upvotes"`
		PublishedAt string `json:"publishedAt"`
		Authors     []struct {
			Name string `json:"name"`
		} `json:"authors"`
	} `json:"paper"`
	Title       string `json:"title"`
	PublishedAt string `json:"publishedAt"`
}

func (a *hfAdapter) Fetch(ctx context.Context, src config.Source) ([]normalize.Raw, error) {
	var entries []hfEntry
	if err := a.fetch.getJSON(ctx, src.URL, nil, &entries); err != nil {
		return nil, fmt.Errorf("hf papers %s: %w", src.ID, err)
	}

	out := make([]normalize.Raw, 0, len(entries))
	for _, e := range entries {
		if e.Paper.ID == "" {
			continue
		}
		if e.Paper.Upvotes < src.Options.MinUpvotes {
			continue
		}

		title := e.Paper.Title
		if title == "" {
			title = e.Title
		}
		if title == "" {
			continue
		}

		out = append(out, normalize.Raw{
			SourceID:    src.ID,
			SourceName:  src.Name,
			SourceTier:  src.Tier,
			URL:         "https://huggingface.co/papers/" + e.Paper.ID,
			Title:       title,
			Body:        e.Paper.Summary,
			Author:      hfFirstAuthor(e),
			PublishedAt: hfTime(e),
			PaperURL:    "https://arxiv.org/abs/" + e.Paper.ID,
			Metrics: model.Metrics{
				HFUpvotes: e.Paper.Upvotes,
			},
		})
	}
	return out, nil
}

func hfFirstAuthor(e hfEntry) string {
	if len(e.Paper.Authors) > 0 {
		return e.Paper.Authors[0].Name
	}
	return ""
}

// hfTime reads whichever timestamp the entry carries. A zero result makes
// normalize substitute the fetch time.
func hfTime(e hfEntry) time.Time {
	for _, s := range []string{e.Paper.PublishedAt, e.PublishedAt} {
		if s == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
