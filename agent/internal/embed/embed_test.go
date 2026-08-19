package embed

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestText(t *testing.T) {
	tests := []struct {
		name    string
		title   string
		excerpt string
		want    string
	}{
		{"title and excerpt", "GPT-5 ships", "OpenAI released it today.", "GPT-5 ships OpenAI released it today."},
		{"empty excerpt", "GPT-5 ships", "", "GPT-5 ships"},
		{"excerpt is trimmed", "T", strings.Repeat("a", 300), "T " + strings.Repeat("a", 200)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Text(tt.title, tt.excerpt); got != tt.want {
				t.Errorf("Text() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTextCountsRunesNotBytes(t *testing.T) {
	got := Text("T", strings.Repeat("é", 300))
	if n := len([]rune(got)); n != 202 { // "T" + " " + 200 runes
		t.Errorf("got %d runes, want 202", n)
	}
}

// --- batching ---------------------------------------------------------------

func TestEmbedBatches(t *testing.T) {
	tests := []struct {
		name        string
		texts       int
		size        int
		wantBatches int
	}{
		{"one full batch", 4, 4, 1},
		{"splits evenly", 8, 4, 2},
		{"trailing partial batch", 9, 4, 3},
		{"batch larger than input", 3, 100, 1},
		{"size zero means one batch", 5, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			texts := make([]string, tt.texts)
			for i := range texts {
				texts[i] = string(rune('a' + i))
			}

			batches := 0
			got, err := embedBatches(context.Background(), texts, tt.size,
				func(_ context.Context, batch []string) ([][]float32, error) {
					batches++
					out := make([][]float32, len(batch))
					for i := range batch {
						out[i] = []float32{float32(i)}
					}
					return out, nil
				})
			if err != nil {
				t.Fatalf("embedBatches() = %v", err)
			}
			if batches != tt.wantBatches {
				t.Errorf("made %d batches, want %d", batches, tt.wantBatches)
			}
			if len(got) != tt.texts {
				t.Errorf("got %d vectors, want %d", len(got), tt.texts)
			}
		})
	}
}

func TestEmbedBatchesPreservesOrder(t *testing.T) {
	texts := []string{"a", "b", "c", "d", "e"}

	got, err := embedBatches(context.Background(), texts, 2,
		func(_ context.Context, batch []string) ([][]float32, error) {
			out := make([][]float32, len(batch))
			for i, s := range batch {
				out[i] = []float32{float32(s[0])}
			}
			return out, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range texts {
		if got[i][0] != float32(s[0]) {
			t.Errorf("vector %d = %v, want the one for %q", i, got[i], s)
		}
	}
}

// A provider returning the wrong number of vectors would silently misalign
// every item with someone else's embedding.
func TestEmbedBatchesRejectsCountMismatch(t *testing.T) {
	_, err := embedBatches(context.Background(), []string{"a", "b"}, 2,
		func(_ context.Context, batch []string) ([][]float32, error) {
			return [][]float32{{1}}, nil // one vector for two texts
		})
	if err == nil {
		t.Fatal("embedBatches() = nil, want a count mismatch error")
	}
}

func TestEmbedBatchesPropagatesError(t *testing.T) {
	want := errors.New("provider down")
	_, err := embedBatches(context.Background(), []string{"a"}, 1,
		func(_ context.Context, _ []string) ([][]float32, error) { return nil, want })
	if !errors.Is(err, want) {
		t.Errorf("embedBatches() = %v, want %v", err, want)
	}
}

func TestEmbedBatchesEmptyInput(t *testing.T) {
	got, err := embedBatches(context.Background(), nil, 4,
		func(_ context.Context, _ []string) ([][]float32, error) {
			t.Fatal("provider called for empty input")
			return nil, nil
		})
	if err != nil || got != nil {
		t.Errorf("embedBatches(nil) = %v, %v; want nil, nil", got, err)
	}
}

// --- providers --------------------------------------------------------------

func TestNIMEmbedder(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer nim-key" {
			t.Errorf("Authorization = %q, want the bearer token", got)
		}
		decodeJSON(t, r, &body)
		w.Write([]byte(`{"data":[
			{"index":0,"embedding":[0.1,0.2]},
			{"index":1,"embedding":[0.3,0.4]}
		]}`))
	}))
	defer srv.Close()

	e := newNIM(srv.Client(), srv.URL, "nim-key", "nvidia/nv-embedqa-e5-v5")
	got, err := e.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("Embed() = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d vectors, want 2", len(got))
	}

	// The retrieval models are asymmetric; every item must go in as a passage
	// or the vectors land in a different part of the space.
	if body["input_type"] != "passage" {
		t.Errorf("input_type = %v, want passage", body["input_type"])
	}
	if body["model"] != "nvidia/nv-embedqa-e5-v5" {
		t.Errorf("model = %v", body["model"])
	}
}

// The API documents an index field, so out-of-order responses must be honoured
// rather than assumed away.
func TestNIMEmbedderReordersByIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[
			{"index":1,"embedding":[9,9]},
			{"index":0,"embedding":[1,1]}
		]}`))
	}))
	defer srv.Close()

	got, err := newNIM(srv.Client(), srv.URL, "k", "m").Embed(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	if got[0][0] != 1 || got[1][0] != 9 {
		t.Errorf("vectors = %v, want them reordered to match the input", got)
	}
}

func TestNIMEmbedderRejectsMissingVector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"index":0,"embedding":[1,1]}]}`))
	}))
	defer srv.Close()

	_, err := newNIM(srv.Client(), srv.URL, "k", "m").Embed(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("Embed() = nil, want an error for the missing vector")
	}
}

func TestProviderSurfacesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid api key"}`))
	}))
	defer srv.Close()

	_, err := newNIM(srv.Client(), srv.URL, "bad", "m").Embed(context.Background(), []string{"a"})
	if err == nil {
		t.Fatal("Embed() = nil, want an auth error")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %v, want it to name the status", err)
	}
}

// --- cache ------------------------------------------------------------------

// fakeEmbedder counts how many texts it was actually asked to embed.
type fakeEmbedder struct {
	calls int
	texts int
	err   error
}

func (f *fakeEmbedder) Name() string    { return "fake:v1" }
func (f *fakeEmbedder) Dimensions() int { return 2 }

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.calls++
	f.texts += len(texts)

	out := make([][]float32, len(texts))
	for i, s := range texts {
		out[i] = []float32{float32(len(s)), float32(s[0])}
	}
	return out, nil
}

func TestCachedAvoidsRepeatWork(t *testing.T) {
	path := filepath.Join(t.TempDir(), "embeddings.json")
	inner := &fakeEmbedder{}
	c := NewCached(inner, path, 7)

	texts := []string{"alpha", "bravo"}
	first, err := c.Embed(context.Background(), texts)
	if err != nil {
		t.Fatal(err)
	}
	if inner.texts != 2 {
		t.Fatalf("provider embedded %d texts, want 2", inner.texts)
	}

	second, err := c.Embed(context.Background(), texts)
	if err != nil {
		t.Fatal(err)
	}
	if inner.texts != 2 {
		t.Errorf("provider embedded %d texts in total, want the second call served from cache", inner.texts)
	}
	for i := range first {
		if first[i][0] != second[i][0] {
			t.Errorf("cached vector %d differs from the original", i)
		}
	}
}

func TestCachedDeduplicatesWithinOneCall(t *testing.T) {
	inner := &fakeEmbedder{}
	c := NewCached(inner, filepath.Join(t.TempDir(), "e.json"), 7)

	got, err := c.Embed(context.Background(), []string{"same", "same", "other"})
	if err != nil {
		t.Fatal(err)
	}
	if inner.texts != 2 {
		t.Errorf("provider embedded %d texts, want 2 distinct", inner.texts)
	}
	if got[0][0] != got[1][0] {
		t.Error("duplicate inputs got different vectors")
	}
}

func TestCachedPersistsAcrossRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "embeddings.json")

	first := &fakeEmbedder{}
	c1 := NewCached(first, path, 7)
	if _, err := c1.Embed(context.Background(), []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if err := c1.Save(); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	second := &fakeEmbedder{}
	c2 := NewCached(second, path, 7)
	if _, err := c2.Embed(context.Background(), []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if second.texts != 0 {
		t.Errorf("second run embedded %d texts, want 0 from the persisted cache", second.texts)
	}
}

func TestCachedExpiresOldEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "embeddings.json")
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)

	old := &fakeEmbedder{}
	c1 := NewCached(old, path, 7)
	c1.now = func() time.Time { return now.AddDate(0, 0, -30) }
	if _, err := c1.Embed(context.Background(), []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	// Save with the original clock so the stale entry survives to disk.
	c1.now = func() time.Time { return now.AddDate(0, 0, -30) }
	if err := c1.Save(); err != nil {
		t.Fatal(err)
	}

	fresh := &fakeEmbedder{}
	c2 := NewCached(fresh, path, 7)
	c2.now = func() time.Time { return now }
	if _, err := c2.Embed(context.Background(), []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if fresh.texts != 1 {
		t.Errorf("embedded %d texts, want the expired entry re-embedded", fresh.texts)
	}
}

// Vectors from two models live in different spaces and must never be compared.
func TestCachedKeysByModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "embeddings.json")

	c1 := NewCached(&fakeEmbedder{}, path, 7)
	if _, err := c1.Embed(context.Background(), []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if err := c1.Save(); err != nil {
		t.Fatal(err)
	}

	other := &renamedEmbedder{fakeEmbedder: &fakeEmbedder{}, name: "fake:v2"}
	c2 := NewCached(other, path, 7)
	if _, err := c2.Embed(context.Background(), []string{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if other.texts != 1 {
		t.Error("a different model reused the previous model's cached vector")
	}
}

type renamedEmbedder struct {
	*fakeEmbedder
	name string
}

func (r *renamedEmbedder) Name() string { return r.name }

func TestCachedMissingFileIsNotAnError(t *testing.T) {
	c := NewCached(&fakeEmbedder{}, filepath.Join(t.TempDir(), "nope", "e.json"), 7)
	if _, err := c.Embed(context.Background(), []string{"alpha"}); err != nil {
		t.Errorf("Embed() = %v, want a missing cache to be a miss, not an error", err)
	}
}

func TestCachedPropagatesProviderError(t *testing.T) {
	want := errors.New("provider down")
	c := NewCached(&fakeEmbedder{err: want}, filepath.Join(t.TempDir(), "e.json"), 7)

	if _, err := c.Embed(context.Background(), []string{"alpha"}); !errors.Is(err, want) {
		t.Errorf("Embed() = %v, want %v", err, want)
	}
}

// decodeJSON reads a request body into out.
func decodeJSON(t *testing.T, r *http.Request, out any) {
	t.Helper()
	if err := json.NewDecoder(r.Body).Decode(out); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
}
