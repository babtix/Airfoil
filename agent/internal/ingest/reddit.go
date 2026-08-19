package ingest

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/normalize"
)

// redditAdapter reads a subreddit's public JSON listing.
//
// Reddit returns 429 to generic User-Agent strings, so this adapter always
// sends the configured one explicitly rather than relying on the default.
type redditAdapter struct {
	fetch     *fetcher
	userAgent string
}

func newRedditAdapter(f *fetcher, userAgent string) *redditAdapter {
	// Reddit answers 403 to short or generic agent strings.
	if userAgent == "" || len(userAgent) < 16 {
		userAgent = defaultUserAgent
	}
	return &redditAdapter{fetch: f, userAgent: userAgent}
}

func (a *redditAdapter) Type() string { return config.SourceReddit }

type redditListing struct {
	Data struct {
		After    string `json:"after"`
		Children []struct {
			Data redditPost `json:"data"`
		} `json:"children"`
	} `json:"data"`
}

type redditPost struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Permalink   string  `json:"permalink"`
	Author      string  `json:"author"`
	Selftext    string  `json:"selftext"`
	Score       int     `json:"score"`
	NumComments int     `json:"num_comments"`
	CreatedUTC  float64 `json:"created_utc"`
	IsSelf      bool    `json:"is_self"`
	Stickied    bool    `json:"stickied"`
	Over18      bool    `json:"over_18"`
}

// Fetch prefers the JSON listing, which carries the score, and falls back to
// the public RSS listing when Reddit refuses it.
//
// Reddit now answers 403 to unauthenticated JSON from many networks. The RSS
// listing stays public but has no score, so the subreddit still contributes as
// a distinct source even though its community-validation signal is lost.
func (a *redditAdapter) Fetch(ctx context.Context, src config.Source) ([]normalize.Raw, error) {
	raws, jsonErr := a.fetchJSON(ctx, src)
	if jsonErr == nil {
		return raws, nil
	}

	raws, rssErr := a.fetchRSS(ctx, src)
	if rssErr != nil {
		return nil, fmt.Errorf("reddit %s: json: %v; rss: %w", src.ID, jsonErr, rssErr)
	}
	return raws, nil
}

func (a *redditAdapter) fetchJSON(ctx context.Context, src config.Source) ([]normalize.Raw, error) {
	limit := src.Options.Limit
	if limit <= 0 {
		limit = 50
	}

	endpoint := src.URL
	if !strings.Contains(endpoint, "?") {
		endpoint += "?" + url.Values{"limit": {strconv.Itoa(limit)}}.Encode()
	}

	var listing redditListing
	err := a.fetch.getJSON(ctx, endpoint, map[string]string{"User-Agent": a.userAgent}, &listing)
	if err != nil {
		return nil, err
	}

	out := make([]normalize.Raw, 0, len(listing.Data.Children))
	for _, child := range listing.Data.Children {
		post := child.Data

		// Pinned mod posts are subreddit furniture, not news.
		if post.Stickied || post.Over18 || post.Title == "" {
			continue
		}
		if post.Score < src.Options.MinScore {
			continue
		}

		// A link post points outward; a text post points at the discussion.
		link := post.URL
		if post.IsSelf || link == "" {
			if post.Permalink == "" {
				continue
			}
			link = "https://www.reddit.com" + post.Permalink
		}

		out = append(out, normalize.Raw{
			SourceID:    src.ID,
			SourceName:  src.Name,
			SourceTier:  src.Tier,
			URL:         link,
			Title:       post.Title,
			Body:        post.Selftext,
			Author:      post.Author,
			PublishedAt: time.Unix(int64(post.CreatedUTC), 0).UTC(),
			Metrics: model.Metrics{
				RedditScore: post.Score,
			},
		})
	}
	return out, nil
}

// fetchRSS reads the public listing feed. Scores are unavailable here, so the
// min_score option cannot be applied.
func (a *redditAdapter) fetchRSS(ctx context.Context, src config.Source) ([]normalize.Raw, error) {
	endpoint := strings.TrimSuffix(src.URL, ".json") + "/.rss"

	body, err := a.fetch.get(ctx, endpoint, map[string]string{
		"User-Agent": a.userAgent,
		"Accept":     "application/rss+xml, application/atom+xml, application/xml;q=0.9",
	})
	if err != nil {
		return nil, err
	}

	feed, err := gofeed.NewParser().Parse(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse feed %s: %w", endpoint, err)
	}

	out := make([]normalize.Raw, 0, len(feed.Items))
	for _, item := range feed.Items {
		if item == nil || item.Link == "" || item.Title == "" {
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
