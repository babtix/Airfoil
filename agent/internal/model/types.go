// Package model holds the data types that move between pipeline stages.
// These types are the contract between ingest, cluster, score, and write.
package model

import "time"

// Item is one article from one source, after normalization.
//
// R1: Excerpt is capped at 300 characters by the normalize package. No other
// code path constructs an Item, so nothing longer can reach disk.
type Item struct {
	ID          string    `json:"id"` // sha256(canonicalURL)[:16]
	SourceID    string    `json:"source_id"`
	SourceName  string    `json:"source_name"`
	SourceTier  int       `json:"source_tier"`
	URL         string    `json:"url"` // canonical
	Title       string    `json:"title"`
	Excerpt     string    `json:"excerpt"`
	Author      string    `json:"author,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	FetchedAt   time.Time `json:"fetched_at"`
	Metrics     Metrics   `json:"metrics"`
	RepoURL     string    `json:"repo_url,omitempty"`
	PaperURL    string    `json:"paper_url,omitempty"`
}

// Metrics carries community-validation signals used by scoring.
type Metrics struct {
	HNPoints    int `json:"hn_points,omitempty"`
	HNComments  int `json:"hn_comments,omitempty"`
	RedditScore int `json:"reddit_score,omitempty"`
	HFUpvotes   int `json:"hf_upvotes,omitempty"`
	GitHubStars int `json:"github_stars,omitempty"`
}

// Cluster is a group of Items covering the same event.
type Cluster struct {
	ID       string    `json:"id"`
	Items    []Item    `json:"items"`
	Centroid []float32 `json:"-"`
}

// Story tiers.
const (
	TierMajor   = "major"
	TierNotable = "notable"
	TierMinor   = "minor"
)

// Story is a cluster after scoring and summarization.
//
// data/stories.json is the authoritative record of every published story.
// The markdown files under site/src/content/stories/ are rendered from it, so
// the writer never needs to parse frontmatter back.
//
// The JSON tags deliberately match site/src/types/story.ts rather than the Go
// field names: the site imports data/stories.json directly at build time, so
// this struct *is* the render contract. `ts` and `cluster` look wrong next to
// the field names, and renaming them would silently break the site's date
// filters and cluster badges — the fields would simply arrive undefined.
type Story struct {
	ID              string        `json:"id"`
	Slug            string        `json:"slug"`
	Title           string        `json:"title"`
	Summary         string        `json:"summary"`
	Score           int           `json:"score"`
	Tier            string        `json:"tier"`
	Tags            []string      `json:"tags"`
	BuilderRelevant bool          `json:"builder_relevant"`
	Date            time.Time     `json:"ts"`
	ClusterSize     int           `json:"cluster"`
	Sources         []StorySource `json:"sources"`
	Takeaways       []string      `json:"takeaways,omitempty"`
	// Body is never omitted: the site types it as string[] and maps over it,
	// so it must serialize as [] rather than null.
	Body []string `json:"body"`
}

// StorySource is one outbound link on a story page. R3: every source a cluster
// drew from appears here.
type StorySource struct {
	Name    string         `json:"name"`
	URL     string         `json:"url"`
	Tier    int            `json:"tier"`
	Type    string         `json:"type"` // lab | press | community
	Metrics *SourceMetrics `json:"metrics,omitempty"`
}

// SourceMetrics mirrors the metrics block the site renders per source.
type SourceMetrics struct {
	Points   int `json:"points,omitempty"`
	Comments int `json:"comments,omitempty"`
	Score    int `json:"score,omitempty"`
	Upvotes  int `json:"upvotes,omitempty"`
	Stars    int `json:"stars,omitempty"`
}

// SourceType maps a source tier to the coarse category the site renders.
func SourceType(tier int) string {
	switch {
	case tier <= 3:
		return "lab"
	case tier == 4:
		return "press"
	default:
		return "community"
	}
}

// IndexEntry is the slim record in data/index.json used by feed/top/ship.
type IndexEntry struct {
	ID              string    `json:"id"`
	Slug            string    `json:"slug"`
	Title           string    `json:"title"`
	Summary         string    `json:"summary"`
	Score           int       `json:"score"`
	Tier            string    `json:"tier"`
	Tags            []string  `json:"tags"`
	BuilderRelevant bool      `json:"builder_relevant"`
	Date            time.Time `json:"date"`
	ClusterSize     int       `json:"cluster_size"`
}

// Index is the whole of data/index.json.
type Index struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Stories     []IndexEntry `json:"stories"`
}

// State is data/state.json — what makes the agent idempotent (R7).
type State struct {
	// SeenURLs maps an item ID to the date it was first seen, as YYYY-MM-DD.
	SeenURLs map[string]string `json:"seen_urls"`
	// Cursors holds per-source high-water marks.
	Cursors map[string]string `json:"cursors"`
	LastRun *time.Time        `json:"last_run"`
}

// NewState returns an empty, non-nil State.
func NewState() *State {
	return &State{
		SeenURLs: map[string]string{},
		Cursors:  map[string]string{},
	}
}

// Seen reports whether an item ID has already been ingested.
func (s *State) Seen(id string) bool {
	_, ok := s.SeenURLs[id]
	return ok
}

// MarkSeen records an item ID as ingested on the given day.
func (s *State) MarkSeen(id string, day time.Time) {
	if s.SeenURLs == nil {
		s.SeenURLs = map[string]string{}
	}
	s.SeenURLs[id] = day.UTC().Format(time.DateOnly)
}
