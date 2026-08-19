package ingest

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/papitsho/airfoil/internal/config"
)

const hfModelsJSON = `[
  {"id":"Qwen/Qwen3.8-27B","author":"Qwen","likes":11409,"downloads":1006235,
   "createdAt":"2026-08-05T08:22:59.000Z","pipeline_tag":"image-text-to-text",
   "library_name":"transformers",
   "tags":["transformers","safetensors","qwen3_5","image-text-to-text","license:apache-2.0","endpoints_compatible"]},
  {"id":"someone/my-lora-checkpoint","author":"someone","likes":2,"downloads":11,
   "createdAt":"2026-08-19T01:00:00.000Z","pipeline_tag":"text-generation","tags":["peft"]},
  {"id":"private/hidden","author":"private","likes":900,"private":true,
   "createdAt":"2026-08-18T01:00:00.000Z"}
]`

func TestHFModelsAdapter(t *testing.T) {
	var gotQuery url.Values
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(hfModelsJSON))
	})

	src := config.Source{
		ID: "hf-models", Name: "Hugging Face Models", Type: config.SourceHFModels,
		URL: srv.URL, Tier: 1,
		Options: config.SourceOptions{Sort: "likes7d", Limit: 60, MinLikes: 50},
	}

	got, err := newHFModelsAdapter(testFetcher()).Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch() = %v", err)
	}

	// The low-like checkpoint and the private repo are both dropped.
	if len(got) != 1 {
		t.Fatalf("got %d raws, want 1 after filtering", len(got))
	}

	raw := got[0]
	if want := "https://huggingface.co/Qwen/Qwen3.8-27B"; raw.URL != want {
		t.Errorf("URL = %q, want %q", raw.URL, want)
	}
	if !strings.Contains(raw.Title, "Qwen/Qwen3.8-27B") {
		t.Errorf("Title = %q, want it to name the model", raw.Title)
	}
	if !strings.Contains(raw.Title, "image-text-to-text") {
		t.Errorf("Title = %q, want the task in it", raw.Title)
	}
	if raw.Metrics.HFUpvotes != 11409 {
		t.Errorf("HFUpvotes = %d, want 11409", raw.Metrics.HFUpvotes)
	}
	if raw.SourceTier != 1 {
		t.Errorf("SourceTier = %d, want 1", raw.SourceTier)
	}

	// createdAt sorting returns thousands of personal fine-tunes, so the
	// configured sort has to reach the API.
	if got := gotQuery.Get("sort"); got != "likes7d" {
		t.Errorf("sort = %q, want likes7d", got)
	}
	if got := gotQuery.Get("limit"); got != "60" {
		t.Errorf("limit = %q, want 60", got)
	}
}

// The body is what carries builder signals into scoring, so it must name the
// facts a reader and the keyword matcher both care about.
func TestHFModelsBodyCarriesReleaseSignals(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(hfModelsJSON))
	})

	src := config.Source{
		ID: "hf-models", Name: "HF", URL: srv.URL, Tier: 1,
		Options: config.SourceOptions{MinLikes: 50},
	}
	got, err := newHFModelsAdapter(testFetcher()).Fetch(context.Background(), src)
	if err != nil {
		t.Fatal(err)
	}

	body := strings.ToLower(got[0].Body)
	for _, want := range []string{"open model weights", "apache-2.0", "transformers", "likes"} {
		if !strings.Contains(body, want) {
			t.Errorf("body is missing %q: %s", want, got[0].Body)
		}
	}
	// Machine-oriented tags are noise for a reader.
	if strings.Contains(body, "endpoints_compatible") {
		t.Errorf("body kept a machine tag: %s", got[0].Body)
	}
}

func TestHFModelsDefaults(t *testing.T) {
	var gotQuery url.Values
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`[]`))
	})

	src := config.Source{ID: "hf-models", Name: "HF", URL: srv.URL, Tier: 1}
	if _, err := newHFModelsAdapter(testFetcher()).Fetch(context.Background(), src); err != nil {
		t.Fatal(err)
	}

	if got := gotQuery.Get("sort"); got != "likes7d" {
		t.Errorf("default sort = %q, want likes7d", got)
	}
	if got := gotQuery.Get("direction"); got != "-1" {
		t.Errorf("direction = %q, want -1", got)
	}
}

func TestHFLicense(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want string
	}{
		{"apache", []string{"transformers", "license:apache-2.0"}, "apache-2.0"},
		{"mit", []string{"license:mit"}, "mit"},
		{"none", []string{"transformers", "safetensors"}, ""},
		{"empty", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hfLicense(tt.tags); got != tt.want {
				t.Errorf("hfLicense(%v) = %q, want %q", tt.tags, got, tt.want)
			}
		})
	}
}

func TestHFDescriptiveTags(t *testing.T) {
	got := hfDescriptiveTags([]string{
		"transformers", "safetensors", "qwen3_5", "conversational",
		"license:apache-2.0", "endpoints_compatible", "moe", "reasoning",
	})

	for _, unwanted := range []string{"transformers", "safetensors", "endpoints_compatible", "license:apache-2.0"} {
		for _, g := range got {
			if g == unwanted {
				t.Errorf("kept the noise tag %q", unwanted)
			}
		}
	}
	if len(got) == 0 {
		t.Fatal("dropped every tag")
	}
	if len(got) > 6 {
		t.Errorf("kept %d tags, want at most 6", len(got))
	}
}
