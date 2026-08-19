package cluster

import (
	"math"
	"testing"
	"time"

	"github.com/papitsho/airfoil/internal/model"
)

var base = time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

func item(id string, tier int, ageHours int) model.Item {
	return model.Item{
		ID:          id,
		Title:       id,
		URL:         "https://example.com/" + id,
		SourceTier:  tier,
		PublishedAt: base.Add(-time.Duration(ageHours) * time.Hour),
	}
}

// vec returns a unit-ish vector pointing mostly along axis, so that items with
// the same axis are similar and items on different axes are not.
func vec(axis int, jitter float32) []float32 {
	v := make([]float32, 4)
	v[axis] = 1
	v[(axis+1)%4] = jitter
	return v
}

func defaultOpts() Options {
	return Options{Threshold: 0.82, Window: 48 * time.Hour, Now: base}
}

var noDebug = [2]float64{0, 0}

func TestClusterGroupsSimilarItems(t *testing.T) {
	items := []model.Item{item("a", 1, 1), item("b", 4, 2), item("c", 5, 3)}
	vectors := [][]float32{vec(0, 0.05), vec(0, 0.02), vec(0, 0.03)}

	got := Cluster(items, vectors, defaultOpts(), noDebug)
	if len(got.Clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(got.Clusters))
	}
	if n := len(got.Clusters[0].Items); n != 3 {
		t.Errorf("cluster holds %d items, want 3", n)
	}
}

func TestClusterSeparatesDissimilarItems(t *testing.T) {
	items := []model.Item{item("a", 1, 1), item("b", 1, 2), item("c", 1, 3)}
	vectors := [][]float32{vec(0, 0), vec(1, 0), vec(2, 0)}

	got := Cluster(items, vectors, defaultOpts(), noDebug)
	if len(got.Clusters) != 3 {
		t.Fatalf("got %d clusters, want 3", len(got.Clusters))
	}
}

func TestClusterThresholdIsRespected(t *testing.T) {
	items := []model.Item{item("a", 1, 1), item("b", 1, 2)}
	vectors := [][]float32{{1, 0}, {0.9, 0.436}} // cosine ≈ 0.9

	tests := []struct {
		name      string
		threshold float64
		want      int
	}{
		{"below the similarity groups them", 0.82, 1},
		{"above the similarity splits them", 0.95, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := defaultOpts()
			opts.Threshold = tt.threshold

			got := Cluster(items, vectors, opts, noDebug)
			if len(got.Clusters) != tt.want {
				t.Errorf("got %d clusters, want %d", len(got.Clusters), tt.want)
			}
		})
	}
}

// Two items covering the same event may be worded differently enough to miss
// the threshold, so a shared identifier has to override the vectors entirely.
func TestClusterForcesItemsSharingALink(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(a, b *model.Item)
	}{
		{"same canonical url", func(a, b *model.Item) { b.URL = a.URL }},
		{"same repo", func(a, b *model.Item) {
			a.RepoURL, b.RepoURL = "https://github.com/x/y", "https://github.com/x/y"
		}},
		{"same paper", func(a, b *model.Item) {
			a.PaperURL, b.PaperURL = "https://arxiv.org/abs/2401.12345", "https://arxiv.org/abs/2401.12345"
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := item("a", 1, 1), item("b", 4, 2)
			tt.mutate(&a, &b)

			// Deliberately orthogonal vectors: similarity alone would split these.
			got := Cluster([]model.Item{a, b}, [][]float32{vec(0, 0), vec(2, 0)}, defaultOpts(), noDebug)
			if len(got.Clusters) != 1 {
				t.Fatalf("got %d clusters, want 1 forced by the shared link", len(got.Clusters))
			}
		})
	}
}

func TestClusterDoesNotForceDifferentLinks(t *testing.T) {
	a, b := item("a", 1, 1), item("b", 1, 2)
	a.RepoURL = "https://github.com/x/y"
	b.RepoURL = "https://github.com/other/repo"

	got := Cluster([]model.Item{a, b}, [][]float32{vec(0, 0), vec(2, 0)}, defaultOpts(), noDebug)
	if len(got.Clusters) != 2 {
		t.Errorf("got %d clusters, want 2", len(got.Clusters))
	}
}

func TestClusterRespectsWindow(t *testing.T) {
	// Identical vectors, but published five days apart.
	items := []model.Item{item("recent", 1, 1), item("old", 1, 120)}
	vectors := [][]float32{vec(0, 0), vec(0, 0)}

	got := Cluster(items, vectors, defaultOpts(), noDebug)
	if len(got.Clusters) != 2 {
		t.Errorf("got %d clusters, want 2 — the old item is outside the window", len(got.Clusters))
	}
}

func TestClusterWithoutWindowIgnoresAge(t *testing.T) {
	items := []model.Item{item("recent", 1, 1), item("old", 1, 120)}
	vectors := [][]float32{vec(0, 0), vec(0, 0)}

	opts := defaultOpts()
	opts.Window = 0

	got := Cluster(items, vectors, opts, noDebug)
	if len(got.Clusters) != 1 {
		t.Errorf("got %d clusters, want 1 when the window is disabled", len(got.Clusters))
	}
}

// A failed embedding must cost grouping, not coverage.
func TestClusterKeepsItemsWithNoVector(t *testing.T) {
	items := []model.Item{item("a", 1, 1), item("b", 1, 2)}
	vectors := [][]float32{vec(0, 0), nil}

	got := Cluster(items, vectors, defaultOpts(), noDebug)
	if len(got.Clusters) != 2 {
		t.Fatalf("got %d clusters, want 2", len(got.Clusters))
	}

	total := 0
	for _, c := range got.Clusters {
		total += len(c.Items)
	}
	if total != 2 {
		t.Errorf("clusters hold %d items in total, want both", total)
	}
}

// The lead item supplies the cluster title, so it must be the most
// authoritative, earliest source regardless of input order.
func TestClusterLeadItemIsHighestTierThenEarliest(t *testing.T) {
	press := item("press", 4, 3)
	lab := item("lab", 1, 1)
	labEarlier := item("lab-earlier", 1, 2)
	community := item("community", 5, 4)

	items := []model.Item{press, lab, community, labEarlier}
	vectors := [][]float32{vec(0, 0.01), vec(0, 0.02), vec(0, 0.03), vec(0, 0.04)}

	got := Cluster(items, vectors, defaultOpts(), noDebug)
	if len(got.Clusters) != 1 {
		t.Fatalf("got %d clusters, want 1", len(got.Clusters))
	}
	if lead := got.Clusters[0].Items[0].ID; lead != "lab-earlier" {
		t.Errorf("lead item = %q, want the earliest tier 1 item", lead)
	}
}

func TestClusterOrdersBiggestFirst(t *testing.T) {
	items := []model.Item{
		item("solo", 1, 1),
		item("big-a", 1, 2), item("big-b", 1, 3), item("big-c", 1, 4),
	}
	vectors := [][]float32{vec(2, 0), vec(0, 0.01), vec(0, 0.02), vec(0, 0.03)}

	got := Cluster(items, vectors, defaultOpts(), noDebug)
	if len(got.Clusters) != 2 {
		t.Fatalf("got %d clusters, want 2", len(got.Clusters))
	}
	if n := len(got.Clusters[0].Items); n != 3 {
		t.Errorf("first cluster holds %d items, want the largest with 3", n)
	}
}

func TestClusterBorderlinePairs(t *testing.T) {
	items := []model.Item{item("a", 1, 1), item("b", 1, 2)}
	vectors := [][]float32{{1, 0}, {0.9, 0.436}} // cosine ≈ 0.9

	got := Cluster(items, vectors, defaultOpts(), [2]float64{0.75, 0.95})
	if len(got.Borderline) == 0 {
		t.Fatal("got no borderline pairs, want the 0.9 pair reported")
	}
	if s := got.Borderline[0].Similarity; s < 0.75 || s > 0.95 {
		t.Errorf("similarity %v is outside the debug range", s)
	}
}

func TestClusterBorderlineIsOffByDefault(t *testing.T) {
	items := []model.Item{item("a", 1, 1), item("b", 1, 2)}
	vectors := [][]float32{{1, 0}, {0.9, 0.436}}

	if got := Cluster(items, vectors, defaultOpts(), noDebug); len(got.Borderline) != 0 {
		t.Errorf("got %d borderline pairs, want none without a debug range", len(got.Borderline))
	}
}

func TestClusterEmptyInput(t *testing.T) {
	if got := Cluster(nil, nil, defaultOpts(), noDebug); len(got.Clusters) != 0 {
		t.Errorf("got %d clusters, want 0", len(got.Clusters))
	}
}

// R10: the same input must always produce the same clusters, or the site churns
// on every run.
func TestClusterIsDeterministic(t *testing.T) {
	items := []model.Item{
		item("a", 1, 1), item("b", 4, 2), item("c", 5, 3),
		item("d", 1, 4), item("e", 2, 5), item("f", 3, 6),
	}
	vectors := [][]float32{
		vec(0, 0.01), vec(0, 0.02), vec(1, 0.01),
		vec(1, 0.02), vec(2, 0.01), vec(0, 0.03),
	}

	first := Cluster(items, vectors, defaultOpts(), noDebug)
	for run := range 10 {
		got := Cluster(items, vectors, defaultOpts(), noDebug)

		if len(got.Clusters) != len(first.Clusters) {
			t.Fatalf("run %d produced %d clusters, want %d", run, len(got.Clusters), len(first.Clusters))
		}
		for i := range got.Clusters {
			if got.Clusters[i].ID != first.Clusters[i].ID {
				t.Fatalf("run %d cluster %d has ID %q, want %q",
					run, i, got.Clusters[i].ID, first.Clusters[i].ID)
			}
		}
	}
}

// Every input item must land in exactly one cluster.
func TestClusterPartitionsInput(t *testing.T) {
	items := []model.Item{
		item("a", 1, 1), item("b", 4, 2), item("c", 5, 3),
		item("d", 1, 60), item("e", 2, 5),
	}
	vectors := [][]float32{vec(0, 0.01), vec(0, 0.02), vec(1, 0), vec(0, 0), nil}

	got := Cluster(items, vectors, defaultOpts(), noDebug)

	seen := map[string]int{}
	for _, c := range got.Clusters {
		for _, it := range c.Items {
			seen[it.ID]++
		}
	}
	if len(seen) != len(items) {
		t.Errorf("clusters cover %d items, want %d", len(seen), len(items))
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("item %q appears in %d clusters, want 1", id, n)
		}
	}
}

func TestCosine(t *testing.T) {
	tests := []struct {
		name string
		a, b []float32
		want float64
	}{
		{"identical", []float32{1, 0, 0}, []float32{1, 0, 0}, 1},
		{"orthogonal", []float32{1, 0}, []float32{0, 1}, 0},
		{"opposite", []float32{1, 0}, []float32{-1, 0}, -1},
		{"scale invariant", []float32{1, 2, 3}, []float32{2, 4, 6}, 1},
		{"zero vector", []float32{0, 0}, []float32{1, 1}, 0},
		{"length mismatch", []float32{1, 0}, []float32{1, 0, 0}, 0},
		{"both empty", nil, nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Cosine(tt.a, tt.b); math.Abs(got-tt.want) > 1e-6 {
				t.Errorf("Cosine(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCentroid(t *testing.T) {
	t.Run("mean of vectors", func(t *testing.T) {
		got := Centroid([][]float32{{0, 0}, {2, 4}})
		want := []float32{1, 2}

		if len(got) != len(want) {
			t.Fatalf("got width %d, want %d", len(got), len(want))
		}
		for i := range want {
			if math.Abs(float64(got[i]-want[i])) > 1e-6 {
				t.Errorf("component %d = %v, want %v", i, got[i], want[i])
			}
		}
	})

	t.Run("single vector is itself", func(t *testing.T) {
		if got := Centroid([][]float32{{1, 2, 3}}); len(got) != 3 || got[0] != 1 {
			t.Errorf("Centroid() = %v, want {1 2 3}", got)
		}
	})

	// One malformed response must not corrupt a cluster's centre.
	t.Run("skips vectors of the wrong width", func(t *testing.T) {
		got := Centroid([][]float32{{0, 0}, {2, 4}, {9, 9, 9}})
		if len(got) != 2 || math.Abs(float64(got[0]-1)) > 1e-6 {
			t.Errorf("Centroid() = %v, want the mean of the two valid vectors", got)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		if got := Centroid(nil); got != nil {
			t.Errorf("Centroid(nil) = %v, want nil", got)
		}
	})
}
