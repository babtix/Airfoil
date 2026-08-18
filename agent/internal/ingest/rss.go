package ingest

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/normalize"
)

// rssAdapter handles lab blogs, press feeds, and arXiv.
type rssAdapter struct {
	fetch *fetcher
}

func newRSSAdapter(f *fetcher) *rssAdapter { return &rssAdapter{fetch: f} }

func (a *rssAdapter) Type() string { return config.SourceRSS }

func (a *rssAdapter) Fetch(ctx context.Context, src config.Source) ([]normalize.Raw, error) {
	body, err := a.fetch.get(ctx, src.URL, map[string]string{
		"Accept": "application/rss+xml, application/atom+xml, application/xml;q=0.9, */*;q=0.8",
	})
	if err != nil {
		return nil, err
	}

	// gofeed.Parser is not safe for concurrent use, so each fetch gets its own.
	feed, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", src.URL, err)
	}

	out := make([]normalize.Raw, 0, len(feed.Items))
	for _, item := range feed.Items {
		if item == nil || item.Link == "" {
			continue
		}
		out = append(out, normalize.Raw{
			SourceID:    src.ID,
			SourceName:  src.Name,
			SourceTier:  src.Tier,
			URL:         item.Link,
			Title:       item.Title,
			Body:        feedBody(item),
			Author:      feedAuthor(item),
			PublishedAt: feedTime(item),
		})
	}
	return out, nil
}

// feedBody prefers the full content element when a feed provides one, since
// arXiv puts the abstract there and only a stub in the description.
//
// Whatever is returned here is capped at 300 characters by normalize (R1).
func feedBody(item *gofeed.Item) string {
	if item.Content != "" {
		return item.Content
	}
	return item.Description
}

func feedAuthor(item *gofeed.Item) string {
	if len(item.Authors) > 0 && item.Authors[0] != nil {
		return item.Authors[0].Name
	}
	if item.Author != nil {
		return item.Author.Name
	}
	return ""
}

// feedTime falls back through the date fields feeds actually populate. A zero
// result makes normalize substitute the fetch time.
func feedTime(item *gofeed.Item) time.Time {
	if item.PublishedParsed != nil {
		return *item.PublishedParsed
	}
	if item.UpdatedParsed != nil {
		return *item.UpdatedParsed
	}
	return time.Time{}
}
