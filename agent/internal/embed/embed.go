// Package embed turns item text into vectors for clustering.
//
// Two hosted providers are supported: Gemini and NVIDIA NIM. They produce
// different dimensionalities and different similarity distributions, so the
// clustering threshold is tuned per provider and the cache is keyed by model.
//
// Local models are deliberately not supported — the pipeline runs in CI, where
// no local daemon exists.
package embed

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/papitsho/airfoil/internal/config"
)

// requestTimeout bounds a single batch. Embedding several hundred items is many
// batches, so this is per-request, not per-run.
const requestTimeout = 60 * time.Second

// Embedder turns texts into vectors.
type Embedder interface {
	// Embed returns one vector per input text, in the same order.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dimensions is the vector width, or 0 until the first response.
	Dimensions() int
	// Name identifies the provider and model, and keys the cache.
	Name() string
}

// New builds the Embedder selected by AIRFOIL_EMBEDDER.
func New(cfg *config.Config) (Embedder, error) {
	client := &http.Client{Timeout: requestTimeout}

	switch cfg.Embed.Provider {
	case config.EmbedderGemini:
		if cfg.Embed.GeminiKey == "" {
			return nil, fmt.Errorf("embed: AIRFOIL_EMBEDDER=gemini requires GEMINI_API_KEY")
		}
		return newGemini(client, cfg.Embed.GeminiKey, cfg.Embed.GeminiModel), nil

	case config.EmbedderNvidiaNIM:
		if cfg.Embed.NIMKey == "" {
			return nil, fmt.Errorf("embed: AIRFOIL_EMBEDDER=nvidia_nim requires NVIDIA_NIM_API_KEY")
		}
		return newNIM(client, cfg.Embed.NIMBaseURL, cfg.Embed.NIMKey, cfg.Embed.NIMModel), nil

	default:
		return nil, fmt.Errorf("embed: unknown provider %q", cfg.Embed.Provider)
	}
}

// Text is what gets embedded for an item: the title plus the opening of the
// excerpt. The excerpt is trimmed because the tail of a feed summary is usually
// boilerplate, and because the smaller retrieval models cap at 512 tokens.
func Text(title, excerpt string) string {
	const excerptChars = 200

	runes := []rune(excerpt)
	if len(runes) > excerptChars {
		excerpt = string(runes[:excerptChars])
	}
	if excerpt == "" {
		return title
	}
	return title + " " + excerpt
}

// embedBatches splits texts into batches of at most size and calls one at a
// time, preserving input order.
//
// Requests are sequential on purpose: these are rate-limited endpoints, and a
// run embeds a few hundred items at most.
func embedBatches(
	ctx context.Context,
	texts []string,
	size int,
	call func(context.Context, []string) ([][]float32, error),
) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if size <= 0 {
		size = len(texts)
	}

	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += size {
		end := min(start+size, len(texts))

		vectors, err := call(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		if len(vectors) != end-start {
			return nil, fmt.Errorf("embed: batch returned %d vectors for %d texts", len(vectors), end-start)
		}
		out = append(out, vectors...)
	}
	return out, nil
}
