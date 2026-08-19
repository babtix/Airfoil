// Package cluster groups items that cover the same event.
//
// This is what makes Airfoil not an RSS reader: twenty outlets covering one
// release become one story. The algorithm is a pure function over items and
// their vectors, and is table-driven tested (R10).
package cluster

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"time"

	"github.com/papitsho/airfoil/internal/model"
)

// Options are the tunables from config/scoring.json.
type Options struct {
	// Threshold is the minimum cosine similarity for an item to join a cluster.
	Threshold float64
	// Window bounds how far apart two items may be published and still group.
	Window time.Duration
	// Now anchors the window. Zero means "use the newest item".
	Now time.Time
}

// Pair is one similarity measurement, used by `airfoil cluster --debug` to tune
// the threshold against real data instead of guesses.
type Pair struct {
	A, B       model.Item
	Similarity float64
}

// Result carries the clusters plus the diagnostics the debug flag prints.
type Result struct {
	Clusters []model.Cluster
	// Borderline holds pairs whose similarity landed in the debug range —
	// the ones that reveal whether the threshold is set right.
	Borderline []Pair
}

// Cluster groups items by embedding similarity, single-link and greedy.
//
// vectors is indexed in parallel with items. An item with no vector still gets
// a cluster of its own rather than being dropped, so a failed embedding costs
// grouping, not coverage.
//
// Items sharing a canonical URL, repository, or paper are forced together
// regardless of similarity: those are the same event by definition, and
// embeddings of two differently-worded headlines can easily fall short of the
// threshold.
//
// The comparison is O(n²) over a few hundred items. That is fine. Do not
// optimize it.
func Cluster(items []model.Item, vectors [][]float32, opts Options, debugRange [2]float64) Result {
	if len(items) == 0 {
		return Result{}
	}

	// Newest first, so a cluster is anchored by the freshest coverage and the
	// result does not depend on the caller's ordering.
	order := make([]int, len(items))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return items[order[a]].PublishedAt.After(items[order[b]].PublishedAt)
	})

	anchor := opts.Now
	if anchor.IsZero() && len(order) > 0 {
		anchor = items[order[0]].PublishedAt
	}

	var (
		groups   []*group
		byLink   = map[string]*group{} // canonical URL, repo, or paper -> cluster
		result   Result
		hasDebug = debugRange[1] > debugRange[0]
	)

	for _, idx := range order {
		item := items[idx]

		// Outside the window an item can only start a cluster of its own.
		if opts.Window > 0 && anchor.Sub(item.PublishedAt) > opts.Window {
			groups = append(groups, newGroup(item, vectors[idx]))
			continue
		}

		// A shared identifier is decisive; no vector comparison can override it.
		if g := findLinked(byLink, item); g != nil {
			g.add(item, vectors[idx])
			indexLinks(byLink, item, g)
			continue
		}

		best, bestSim := (*group)(nil), 0.0
		for _, g := range groups {
			if opts.Window > 0 && absDuration(g.newest.Sub(item.PublishedAt)) > opts.Window {
				continue
			}
			sim := Cosine(g.centroid, vectors[idx])
			if hasDebug && sim >= debugRange[0] && sim <= debugRange[1] {
				result.Borderline = append(result.Borderline, Pair{A: g.items[0], B: item, Similarity: sim})
			}
			if sim > bestSim {
				best, bestSim = g, sim
			}
		}

		if best != nil && bestSim >= opts.Threshold {
			best.add(item, vectors[idx])
			indexLinks(byLink, item, best)
			continue
		}

		g := newGroup(item, vectors[idx])
		groups = append(groups, g)
		indexLinks(byLink, item, g)
	}

	result.Clusters = make([]model.Cluster, len(groups))
	for i, g := range groups {
		result.Clusters[i] = g.finish()
	}

	// Largest cluster first — the most-covered event is the biggest story.
	sort.SliceStable(result.Clusters, func(a, b int) bool {
		ca, cb := result.Clusters[a], result.Clusters[b]
		if len(ca.Items) != len(cb.Items) {
			return len(ca.Items) > len(cb.Items)
		}
		return ca.Items[0].PublishedAt.After(cb.Items[0].PublishedAt)
	})

	sort.SliceStable(result.Borderline, func(a, b int) bool {
		return result.Borderline[a].Similarity > result.Borderline[b].Similarity
	})

	return result
}

// group is a cluster under construction.
type group struct {
	items    []model.Item
	vectors  [][]float32
	centroid []float32
	newest   time.Time
}

func newGroup(item model.Item, vector []float32) *group {
	g := &group{newest: item.PublishedAt}
	g.add(item, vector)
	return g
}

func (g *group) add(item model.Item, vector []float32) {
	g.items = append(g.items, item)
	if len(vector) > 0 {
		g.vectors = append(g.vectors, vector)
		g.centroid = Centroid(g.vectors)
	}
	if item.PublishedAt.After(g.newest) {
		g.newest = item.PublishedAt
	}
}

// finish orders a cluster's items so that the most authoritative, earliest
// source leads. That item supplies the cluster's title.
func (g *group) finish() model.Cluster {
	items := make([]model.Item, len(g.items))
	copy(items, g.items)

	sort.SliceStable(items, func(a, b int) bool {
		if items[a].SourceTier != items[b].SourceTier {
			return items[a].SourceTier < items[b].SourceTier
		}
		return items[a].PublishedAt.Before(items[b].PublishedAt)
	})

	return model.Cluster{
		ID:       clusterID(items),
		Items:    items,
		Centroid: g.centroid,
	}
}

// clusterID is derived from the lead item, so that a cluster keeps its identity
// across runs as long as its lead does.
func clusterID(items []model.Item) string {
	if len(items) == 0 {
		return ""
	}
	sum := sha256.Sum256([]byte(items[0].ID))
	return hex.EncodeToString(sum[:])[:16]
}

// linkKeys returns the identifiers that make two items the same event.
func linkKeys(item model.Item) []string {
	var keys []string
	if item.URL != "" {
		keys = append(keys, "u:"+item.URL)
	}
	if item.RepoURL != "" {
		keys = append(keys, "r:"+item.RepoURL)
	}
	if item.PaperURL != "" {
		keys = append(keys, "p:"+item.PaperURL)
	}
	return keys
}

func findLinked(byLink map[string]*group, item model.Item) *group {
	for _, key := range linkKeys(item) {
		if g, ok := byLink[key]; ok {
			return g
		}
	}
	return nil
}

func indexLinks(byLink map[string]*group, item model.Item, g *group) {
	for _, key := range linkKeys(item) {
		if _, taken := byLink[key]; !taken {
			byLink[key] = g
		}
	}
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
