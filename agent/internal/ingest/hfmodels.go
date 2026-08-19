package ingest

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/normalize"
)

// hfModelsAdapter reads trending model releases from the HuggingFace hub.
//
// This is where open-weight model releases actually land. Labs like Zhipu,
// Qwen, DeepSeek, MiniMax, and Moonshot publish weights here and mostly do not
// run an RSS feed at all, so without this adapter a major release only reaches
// the corpus as second-hand chatter on Hacker News or Reddit.
//
// Sorting matters: createdAt returns thousands of personal fine-tunes and
// checkpoint dumps, while likes7d surfaces the handful of releases people
// actually reacted to.
type hfModelsAdapter struct {
	fetch *fetcher
}

func newHFModelsAdapter(f *fetcher) *hfModelsAdapter { return &hfModelsAdapter{fetch: f} }

func (a *hfModelsAdapter) Type() string { return config.SourceHFModels }

type hfModel struct {
	ID            string   `json:"id"`
	Author        string   `json:"author"`
	Likes         int      `json:"likes"`
	Downloads     int      `json:"downloads"`
	CreatedAt     string   `json:"createdAt"`
	PipelineTag   string   `json:"pipeline_tag"`
	LibraryName   string   `json:"library_name"`
	Tags          []string `json:"tags"`
	TrendingScore float64  `json:"trendingScore"`
	Private       bool     `json:"private"`
	Gated         any      `json:"gated"`
}

func (a *hfModelsAdapter) Fetch(ctx context.Context, src config.Source) ([]normalize.Raw, error) {
	opts := src.Options

	sort := opts.Sort
	if sort == "" {
		sort = "likes7d"
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 60
	}

	q := url.Values{}
	q.Set("sort", sort)
	q.Set("direction", "-1")
	q.Set("limit", strconv.Itoa(limit))

	var models []hfModel
	if err := a.fetch.getJSON(ctx, src.URL+"?"+q.Encode(), nil, &models); err != nil {
		return nil, fmt.Errorf("hf models %s: %w", src.ID, err)
	}

	out := make([]normalize.Raw, 0, len(models))
	for _, m := range models {
		if m.ID == "" || m.Private {
			continue
		}
		if m.Likes < opts.MinLikes {
			continue
		}

		out = append(out, normalize.Raw{
			SourceID:    src.ID,
			SourceName:  src.Name,
			SourceTier:  src.Tier,
			URL:         "https://huggingface.co/" + m.ID,
			Title:       hfModelTitle(m),
			Body:        hfModelBody(m),
			Author:      m.Author,
			PublishedAt: hfModelTime(m),
			Metrics: model.Metrics{
				HFUpvotes: m.Likes,
			},
		})
	}
	return out, nil
}

// hfModelTitle reads as a headline rather than a bare repository path.
func hfModelTitle(m hfModel) string {
	if m.PipelineTag == "" {
		return m.ID
	}
	return m.ID + " (" + m.PipelineTag + ")"
}

// hfModelBody is factual metadata, not article text — there is no prose on a
// model page to copy, so R1 is satisfied by construction.
func hfModelBody(m hfModel) string {
	parts := []string{"Open model weights on Hugging Face."}

	if m.PipelineTag != "" {
		parts = append(parts, "Task: "+m.PipelineTag+".")
	}
	if m.LibraryName != "" {
		parts = append(parts, "Library: "+m.LibraryName+".")
	}
	if license := hfLicense(m.Tags); license != "" {
		parts = append(parts, "License: "+license+".")
	}
	parts = append(parts, fmt.Sprintf("%d likes, %d downloads.", m.Likes, m.Downloads))

	if extra := hfDescriptiveTags(m.Tags); len(extra) > 0 {
		parts = append(parts, "Tags: "+strings.Join(extra, ", ")+".")
	}
	return strings.Join(parts, " ")
}

// hfLicense pulls the license out of the tag list, where the hub encodes it.
func hfLicense(tags []string) string {
	for _, t := range tags {
		if rest, ok := strings.CutPrefix(t, "license:"); ok {
			return rest
		}
	}
	return ""
}

// hfDescriptiveTags drops the machine-oriented tags that carry no signal for a
// reader, keeping the ones that describe the model.
func hfDescriptiveTags(tags []string) []string {
	skip := map[string]bool{
		"transformers": true, "safetensors": true, "endpoints_compatible": true,
		"autotrain_compatible": true, "region:us": true, "region:eu": true,
		"eval-results": true, "has_space": true, "text-generation-inference": true,
	}

	var out []string
	for _, t := range tags {
		if skip[t] || strings.Contains(t, ":") {
			continue
		}
		out = append(out, t)
		if len(out) == 6 {
			break
		}
	}
	return out
}

// hfModelTime uses the creation date, which is when the weights appeared.
func hfModelTime(m hfModel) time.Time {
	if t, err := time.Parse(time.RFC3339, m.CreatedAt); err == nil {
		return t
	}
	return time.Time{}
}
