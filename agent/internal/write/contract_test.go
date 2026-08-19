package write

import (
	"encoding/json"
	"testing"

	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/store"
	"github.com/papitsho/airfoil/internal/summarize"
)

// TestStoriesJSONMatchesSiteContract pins the wire shape of data/stories.json.
//
// The site imports that file directly at build time and types it as
// site/src/types/story.ts. A renamed JSON tag does not fail any Go test on its
// own — it fails silently in the browser, where `ts` and `cluster` arrive
// undefined and the date filters quietly stop working. This test is the only
// thing standing between a field rename and that.
func TestStoriesJSONMatchesSiteContract(t *testing.T) {
	w := testWriter(t)
	res := result("c1", 80, true, item("i1", "openai", "https://a.test/1", "Raw", 1))

	if _, err := w.Run([]summarize.Result{res}, now, false); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	raw, err := store.ReadJSON[[]map[string]any](w.StoriesPath())
	if err != nil {
		t.Fatalf("reading stories.json: %v", err)
	}
	if len(raw) != 1 {
		t.Fatalf("stories = %d, want 1", len(raw))
	}
	got := raw[0]

	// Exactly the field names site/src/types/story.ts declares.
	required := []string{
		"id", "slug", "title", "score", "cluster", "ts",
		"summary", "body", "tags", "sources",
	}
	for _, key := range required {
		if _, ok := got[key]; !ok {
			t.Errorf("stories.json is missing %q — the site types this field", key)
		}
	}

	// The Go field names must not leak through as JSON keys.
	for _, forbidden := range []string{"cluster_size", "date", "Date", "ClusterSize"} {
		if _, ok := got[forbidden]; ok {
			t.Errorf("stories.json exposes %q; the site expects the mapped name", forbidden)
		}
	}

	// body must be an array, never null: the site maps over it.
	if body, ok := got["body"]; !ok || body == nil {
		t.Errorf("body = %v, want an array", body)
	} else if _, ok := body.([]any); !ok {
		t.Errorf("body is %T, want an array", body)
	}

	// Sources carry the discriminator the site switches on.
	sources, ok := got["sources"].([]any)
	if !ok || len(sources) == 0 {
		t.Fatalf("sources = %v, want a non-empty array", got["sources"])
	}
	src, ok := sources[0].(map[string]any)
	if !ok {
		t.Fatalf("source is %T, want an object", sources[0])
	}
	for _, key := range []string{"name", "url", "type"} {
		if _, ok := src[key]; !ok {
			t.Errorf("source is missing %q", key)
		}
	}

	// type must be one of the site's SourceType union members.
	switch src["type"] {
	case "lab", "press", "community":
	default:
		t.Errorf("source type = %v, want lab | press | community", src["type"])
	}
}

// TestStoryRoundTripsThroughStoriesJSON guards the readers.
//
// publish and digest both read stories.json back into model.Story. If the tags
// ever drift from the struct, those reads would silently produce zero dates and
// zero cluster sizes rather than failing.
func TestStoryRoundTripsThroughStoriesJSON(t *testing.T) {
	original := model.Story{
		ID: "abc", Slug: "2026-08-17-x", Title: "T", Summary: "S",
		Score: 42, Tier: model.TierNotable, Date: now, ClusterSize: 3,
		Sources: []model.StorySource{{Name: "N", URL: "https://a.test", Tier: 1, Type: "lab"}},
		Body:    []string{},
	}

	encoded, err := json.Marshal([]model.Story{original})
	if err != nil {
		t.Fatal(err)
	}

	var back []model.Story
	if err := json.Unmarshal(encoded, &back); err != nil {
		t.Fatal(err)
	}

	if !back[0].Date.Equal(original.Date) {
		t.Errorf("Date = %v, want %v — publish prunes on this field", back[0].Date, original.Date)
	}
	if back[0].ClusterSize != original.ClusterSize {
		t.Errorf("ClusterSize = %d, want %d", back[0].ClusterSize, original.ClusterSize)
	}
}
