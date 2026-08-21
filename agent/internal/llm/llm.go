// Package llm is the summarization transport: a chain of chat providers tried
// in order until one answers.
//
// It knows nothing about stories or prompts. It sends a system and user message
// and returns raw text. Prompt construction and schema validation live in
// internal/summarize, so both stay testable without a network.
package llm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/papitsho/airfoil/internal/config"
)

// Provider is one chat backend.
type Provider interface {
	Complete(ctx context.Context, sys, user string) (string, error)
	Name() string
}

const (
	// requestTimeout bounds one provider attempt. BUILD_SPEC suggests 30s,
	// which is right for a plain instruct model but far too short here: the
	// configured Nemotron models reason before answering, and the digest's
	// thread prompt spends ~10,000 tokens doing it. At 90s that call was
	// killed mid-generation every time.
	//
	// The cost of a generous timeout is a slow failure; the cost of a tight
	// one is losing work that was about to succeed. Measured: summaries return
	// in ~20s, the digest thread needs 5-7 minutes.
	//
	// This is a ceiling, not a wait — a fast call still returns fast.
	requestTimeout = 10 * time.Minute

	// maxTokens must clear the reasoning pass plus the JSON answer, and the
	// reasoning dominates. Measured against nemotron-3-super: a story summary
	// spends ~1,400 completion tokens, but the digest's X thread — seven posts
	// over five stories in one object — spends ~10,000, almost all of it
	// reasoning that is emitted before any JSON.
	//
	// At 2,048 and again at 4,096 the thread came back as finish_reason
	// "length" with ten thousand characters of reasoning and no JSON at all.
	// This is a ceiling, not a spend: a typical call still uses a fraction.
	maxTokens = 16384

	// temperature is low but not zero: on a retry after a validation failure a
	// fully deterministic model would return the identical invalid answer.
	temperature = 0.3
)

// Chain tries each provider in order and returns the first success.
//
// R8: a provider that errors, times out, or returns empty is logged and the
// chain falls through. Only an exhausted chain is an error.
type Chain struct {
	providers []Provider
	log       *slog.Logger
}

// New builds the chain in the fallback order:
// openrouter → nvidia_nim. Unconfigured providers are skipped rather
// than attempted and failed.
//
// Every provider is a hosted API. There is no local fallback: the pipeline's
// real home is CI, where no daemon is listening, and a local provider would
// make the chain look healthier on a laptop than it is in production.
func New(cfg *config.Config, log *slog.Logger) *Chain {
	var ps []Provider

	if cfg.LLM.OpenRouter.Configured() {
		ps = append(ps, newOpenAICompatible(
			"openrouter", "https://openrouter.ai/api/v1",
			cfg.LLM.OpenRouter.APIKey, cfg.LLM.OpenRouter.Model,
			// OpenRouter attributes traffic by these headers.
			map[string]string{
				"HTTP-Referer": "https://github.com/papitsho/airfoil",
				"X-Title":      "Airfoil",
			}))
	}
	if cfg.LLM.NvidiaNIM.Configured() {
		ps = append(ps, newOpenAICompatible(
			"nvidia_nim", "https://integrate.api.nvidia.com/v1",
			cfg.LLM.NvidiaNIM.APIKey, cfg.LLM.NvidiaNIM.Model, nil))
	}
	return &Chain{providers: ps, log: log}
}

// Providers returns the configured chain in order, for doctor and logging.
func (c *Chain) Providers() []Provider { return c.providers }

// ErrNoProvider means nothing in the chain answered.
var ErrNoProvider = errors.New("llm: every provider failed")

// Complete tries each provider until one returns non-empty text.
func (c *Chain) Complete(ctx context.Context, sys, user string) (string, error) {
	if len(c.providers) == 0 {
		return "", fmt.Errorf("%w: none configured", ErrNoProvider)
	}

	var problems []string
	for _, p := range c.providers {
		attempt, cancel := context.WithTimeout(ctx, requestTimeout)
		out, err := p.Complete(attempt, sys, user)
		cancel()

		switch {
		case err != nil:
			c.log.Warn("llm provider failed", "provider", p.Name(), "error", err)
			problems = append(problems, p.Name()+": "+err.Error())
		case strings.TrimSpace(out) == "":
			// A reasoning model that spends its whole budget thinking returns
			// an empty content field with no error. Treat it as a failure so
			// the chain falls through instead of surfacing "".
			c.log.Warn("llm provider returned empty content", "provider", p.Name())
			problems = append(problems, p.Name()+": empty response")
		default:
			return out, nil
		}

		// A cancelled parent context means the run is shutting down; trying
		// the next provider would just produce another cancellation.
		if ctx.Err() != nil {
			break
		}
	}
	return "", fmt.Errorf("%w: %s", ErrNoProvider, strings.Join(problems, "; "))
}
