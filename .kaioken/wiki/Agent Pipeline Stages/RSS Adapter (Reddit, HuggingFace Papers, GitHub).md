# RSS Adapter (Reddit, HuggingFace Papers, GitHub)

This chapter documents the `rssAdapter` implementation in `agent/internal/ingest/rss.go`. The adapter uses the `gofeed` library to parse RSS, Atom, and RDF feeds, then maps each feed item to a `normalize.Raw` struct for downstream normalization. It handles content extraction from multiple fields, author resolution with fallbacks, and published-time parsing with a cascading fallback chain.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Adapter Structure](#adapter-structure)
- [Fetch Flow](#fetch-flow)
- [Feed Parsing with gofeed](#feed-parsing-with-gofeed)
- [Item-to-Raw Mapping](#item-to-raw-mapping)
- [Content Extraction](#content-extraction)
- [Author Handling](#author-handling)
- [Published Time Parsing with Fallbacks](#published-time-parsing-with-fallbacks)
- [Error Handling](#error-handling)
- [Referenced Files](#referenced-files)

---

## Architecture Overview

The `rssAdapter` implements the `ingest.Adapter` interface and is used by three configured source types in `sources.json`:

| Source Type | Description | Example Feeds |
|-------------|-------------|---------------|
| `Reddit` | Subreddit RSS feeds (e.g., `r/MachineLearning/.rss`) | Reddit community discussions |
| `HFPapers` | Hugging Face Daily Papers RSS | `https://huggingface.co/papers/rss` |
| `GitHub` | GitHub repository releases/commits RSS | `https://github.com/owner/repo/releases.atom` |

All three use the same adapter because they expose standard RSS/Atom feeds. The adapter is instantiated once per `ingest.Runner` and reused across all RSS-type sources.

```mermaid
graph TD
    A[ingest.Runner.Run] --> B[Adapter.Fetch per source]
    B --> C[rssAdapter.Fetch]
    C --> D[fetcher.get HTTP request]
    D --> E[gofeed.NewParser.Parse]
    E --> F[Iterate feed.Items]
    F --> G[feedBody item]
    F --> H[feedAuthor item]
    F --> I[feedTime item]
    G --> J[normalize.Raw]
    H --> J
    I --> J
    J --> K[Return []normalize.Raw]
```

---

## Adapter Structure

The adapter holds a reference to the shared HTTP `fetcher` (which provides timeout, retry, and User-Agent handling).

`agent/internal/ingest/rss.go:16-20`

```go
// rssAdapter handles lab blogs, press feeds, and arXiv.
type rssAdapter struct {
	fetch *fetcher
}

func newRSSAdapter(f *fetcher) *rssAdapter { return &rssAdapter{fetch: f} }
```

The `Type()` method returns the constant `config.SourceRSS` (defined in `internal/config/sources.go`), which the runner uses for logging and metrics.

`agent/internal/ingest/rss.go:22`

```go
func (a *rssAdapter) Type() string { return config.SourceRSS }
```

---

## Fetch Flow

The `Fetch` method performs the following steps:

1. **HTTP GET** with an `Accept` header preferring RSS/Atom/XML.
2. **Parse** the response body with a fresh `gofeed.Parser` instance (not concurrency-safe).
3. **Iterate** over `feed.Items`, skipping nil items or items without a `Link`.
4. **Map** each item to `normalize.Raw` using helper functions for body, author, and time.
5. **Return** the slice of `Raw` items.

`agent/internal/ingest/rss.go:24-55`

```go
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
```

### HTTP Request Details

| Aspect | Value |
|--------|-------|
| Method | `GET` (via `fetcher.get`) |
| Accept Header | `application/rss+xml, application/atom+xml, application/xml;q=0.9, */*;q=0.8` |
| Timeout | Inherited from `fetcher` (default 10s, configurable via `config.Source.Timeout`) |
| Retries | Inherited from `fetcher` (default 3 with exponential backoff) |
| User-Agent | Set by `fetcher` (default `AirfoilBot/1.0`; Reddit sources should override via `src.Options.UserAgent`) |

### Concurrency Note

The comment in the code highlights that `gofeed.Parser` is **not safe for concurrent use**. Therefore, a new parser is created per `Fetch` call. This is acceptable because `ingest.Runner` processes sources sequentially (or with a semaphore), and the parser allocation is lightweight.

---

## Feed Parsing with gofeed

The adapter uses `github.com/mmcdole/gofeed` v1.x, which supports:

- RSS 0.91, 0.92, 1.0, 2.0
- Atom 0.3, 1.0
- RDF/RSS 1.0
- JSON Feed (v1)

The parser normalizes all feed types into a common `*gofeed.Feed` with a slice of `*gofeed.Item`. Each `Item` contains the fields used by the adapter:

| gofeed.Item Field | Used By | Notes |
|-------------------|---------|-------|
| `Link` | `Raw.URL` | Required; items without a link are skipped |
| `Title` | `Raw.Title` | May be empty; normalization handles missing titles |
| `Content` | `feedBody` | Full content (e.g., arXiv abstract); preferred over `Description` |
| `Description` | `feedBody` | Fallback summary/description |
| `Authors` | `feedAuthor` | Slice of `*gofeed.Person` with `Name`, `Email` |
| `Author` | `feedAuthor` | Single `*gofeed.Person` (legacy RSS) |
| `PublishedParsed` | `feedTime` | `*time.Time` from `pubDate` / `published` |
| `UpdatedParsed` | `feedTime` | `*time.Time` from `updated` / `modified` |

---

## Item-to-Raw Mapping

Each valid feed item becomes a `normalize.Raw` struct (defined in `internal/normalize/raw.go`):

```go
type Raw struct {
	SourceID    string
	SourceName  string
	SourceTier  int
	URL         string
	Title       string
	Body        string
	Author      string
	PublishedAt time.Time
}
```

The mapping is direct for most fields. The three computed fields (`Body`, `Author`, `PublishedAt`) are produced by the helper functions documented below.

---

## Content Extraction

`feedBody` prefers the full `Content` field when present, falling back to `Description`. This is critical for arXiv feeds (used by HFPapers), where the abstract appears only in `Content` and `Description` contains a stub.

`agent/internal/ingest/rss.go:61-66`

```go
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
```

### Content Field Precedence

| Priority | Field | Typical Source |
|----------|-------|----------------|
| 1 | `item.Content` | Atom `<content>`, RSS 2.0 `<content:encoded>`, JSON Feed `content_html` |
| 2 | `item.Description` | RSS `<description>`, Atom `<summary>`, JSON Feed `summary` |

**Important**: The returned body is **not truncated here**. The 300-character excerpt cap is enforced later in `normalize.Build` via `normalize.Excerpt` (see *Normalization & Deduplication* chapter). This allows the embedder to receive full content for vector generation.

---

## Author Handling

`feedAuthor` resolves the author name through a two-level fallback:

1. First author in the `Authors` slice (Atom, JSON Feed, RSS 2.0 with `dc:creator`).
2. The singular `Author` field (legacy RSS `<author>` or `<managingEditor>`).

`agent/internal/ingest/rss.go:68-76`

```go
func feedAuthor(item *gofeed.Item) string {
	if len(item.Authors) > 0 && item.Authors[0] != nil {
		return item.Authors[0].Name
	}
	if item.Author != nil {
		return item.Author.Name
	}
	return ""
}
```

### Author Field Sources by Feed Type

| Feed Type | Primary Field | Fallback Field |
|-----------|---------------|----------------|
| Atom 1.0 | `Authors` (from `<author><name>`) | `Author` (deprecated) |
| RSS 2.0 | `Authors` (from `<dc:creator>`) | `Author` (from `<author>` or `<managingEditor>`) |
| JSON Feed | `Authors` (from `authors` array) | `Author` (from `author` object) |
| RSS 1.0 / RDF | `Authors` (from `<dc:creator>`) | `Author` |

If neither field is present, an empty string is returned. The downstream `normalize.Build` does not require an author.

---

## Published Time Parsing with Fallbacks

`feedTime` implements a cascading fallback through the date fields that feeds actually populate. A zero `time.Time` result signals to `normalize.Build` to substitute the fetch time (i.e., `now`).

`agent/internal/ingest/rss.go:80-88`

```go
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
```

### Time Field Precedence

| Priority | gofeed Field | Source Element | Notes |
|----------|--------------|----------------|-------|
| 1 | `PublishedParsed` | RSS `<pubDate>`, Atom `<published>`, JSON Feed `date_published` | Parsed by gofeed into `*time.Time` |
| 2 | `UpdatedParsed` | Atom `<updated>`, RSS `<lastBuildDate>`, JSON Feed `date_modified` | Used when publication date absent |
| 3 | (none) | — | Returns zero time; `normalize.Build` uses fetch time |

### gofeed Parsing Behavior

- gofeed uses a comprehensive date parser supporting RFC822, RFC3339, ISO8601, and common variants.
- If a date string is unparseable, the corresponding `*Parsed` field is `nil` (not a zero time).
- The adapter does **not** attempt custom date parsing; it relies entirely on gofeed.

### Downstream Handling in normalize.Build

When `PublishedAt` is zero, `normalize.Build` substitutes the `now` parameter passed to it (the pipeline run timestamp). This ensures every `Item` has a valid `PublishedAt` for scoring and sorting.

---

## Error Handling

| Error Point | Behavior |
|-------------|----------|
| HTTP fetch failure (`fetcher.get`) | Returns error wrapped by caller; source marked as failed in metrics |
| Feed parse failure (`gofeed.Parse`) | Returns `fmt.Errorf("parse feed %s: %w", src.URL, err)`; source marked as failed |
| Nil item or empty `Link` | Item silently skipped (continue loop) |
| Missing `Title` | Empty string passed to `Raw`; `normalize.Build` generates ID from URL |
| Missing `Body` (both Content and Description empty) | Empty string passed; `normalize.Excerpt` returns empty string |
| Missing `Author` | Empty string passed; not required |
| Zero `PublishedAt` | `normalize.Build` substitutes fetch time |

The adapter does **not** retry individual items — the entire source fetch either succeeds or fails. Partial results are not returned on parse error.

---

## Referenced Files

| File | Role |
|------|------|
| `agent/internal/ingest/rss.go` | Adapter implementation (this chapter) |
| `agent/internal/ingest/fetcher.go` | HTTP fetcher with timeout/retry/User-Agent |
| `agent/internal/ingest/runner.go` | Orchestrates adapters, calls `Fetch` |
| `agent/internal/normalize/raw.go` | `Raw` struct definition |
| `agent/internal/normalize/build.go` | `Build(Raw, now, maxExcerpt) → Item` (consumes `Raw`) |
| `agent/internal/config/sources.go` | `Source` struct, `SourceRSS` constant |
| `agent/internal/config/config.go` | `Config` loading, source enablement |
| `github.com/mmcdole/gofeed` | Feed parsing library (external dependency) |

<!-- kaioken:files agent/internal/ingest/rss.go -->
