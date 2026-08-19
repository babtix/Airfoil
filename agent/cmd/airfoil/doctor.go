package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/embed"
	"github.com/papitsho/airfoil/internal/ingest"
	"github.com/papitsho/airfoil/internal/llm"
)

// newDoctorCmd reports whether the agent is configured well enough to run.
//
// Without --ping it inspects configuration only, which is fast but cannot tell
// a valid API key from a revoked one. --ping goes to the network.
func newDoctorCmd(a *app) *cobra.Command {
	var ping bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check configuration, providers, and sources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDoctor(cmd.Context(), cmd.OutOrStdout(), a.cfg, ping)
		},
	}
	cmd.Flags().BoolVar(&ping, "ping", false,
		"probe every source URL and the embedder, instead of only reading config")

	return cmd
}

func runDoctor(ctx context.Context, w io.Writer, cfg *config.Config, ping bool) error {
	var blocking []string

	fmt.Fprintln(w, "paths")
	fmt.Fprintf(w, "  config   %s\n", cfg.ConfigDir)
	fmt.Fprintf(w, "  data     %s\n", cfg.DataDir)
	fmt.Fprintf(w, "  stories  %s\n", cfg.StoriesDir)

	// Sources, grouped by tier so a missing tier is obvious at a glance.
	enabled := cfg.EnabledSources()
	fmt.Fprintf(w, "\nsources  %d enabled / %d total\n", len(enabled), len(cfg.Sources))
	byTier := map[int][]string{}
	for _, s := range enabled {
		byTier[s.Tier] = append(byTier[s.Tier], fmt.Sprintf("%s (%s)", s.ID, s.Type))
	}
	tiers := make([]int, 0, len(byTier))
	for t := range byTier {
		tiers = append(tiers, t)
	}
	sort.Ints(tiers)
	for _, t := range tiers {
		fmt.Fprintf(w, "  tier %d   %s\n", t, strings.Join(byTier[t], ", "))
	}

	// Embedder.
	fmt.Fprintf(w, "\nembedder %s\n", cfg.Embed.Provider)
	switch cfg.Embed.Provider {
	case config.EmbedderNvidiaNIM:
		fmt.Fprintf(w, "  model    %s\n  api key  %s\n", cfg.Embed.NIMModel, present(cfg.Embed.NIMKey != ""))
		if cfg.Embed.NIMKey == "" {
			blocking = append(blocking, "AIRFOIL_EMBEDDER=nvidia_nim but NVIDIA_NIM_API_KEY is not set")
		}
	}

	// LLM chain, in fallback order.
	fmt.Fprintln(w, "\nllm chain")
	chain := []struct {
		name string
		ok   bool
		note string
	}{
		{"nvidia_nim", cfg.LLM.NvidiaNIM.Configured(), cfg.LLM.NvidiaNIM.Model},
		{"openrouter", cfg.LLM.OpenRouter.Configured(), cfg.LLM.OpenRouter.Model},
	}
	configured := 0
	for _, p := range chain {
		if p.ok {
			configured++
		}
		fmt.Fprintf(w, "  %-12s %-8s %s\n", p.name, present(p.ok), p.note)
	}
	if configured == 0 {
		blocking = append(blocking, "no LLM provider is configured; summarization will skip every cluster")
	}

	// Advisory checks — these degrade a run without breaking it.
	fmt.Fprintln(w, "\ningest")
	fmt.Fprintf(w, "  github token   %s (missing caps the API at 60 req/hr)\n",
		present(cfg.Ingest.GitHubToken != ""))
	ua := cfg.Ingest.RedditUserAgent
	fmt.Fprintf(w, "  reddit ua      %q\n", ua)
	if !strings.Contains(ua, "/u/") || strings.Contains(ua, "YOUR_USERNAME") {
		fmt.Fprintln(w, "  warning: Reddit returns 429 without a descriptive User-Agent naming a real account")
	}

	if ping {
		if err := pingEmbedder(ctx, w, cfg); err != nil {
			blocking = append(blocking, err.Error())
		}
		if err := pingLLM(ctx, w, cfg); err != nil {
			blocking = append(blocking, err.Error())
		}
		if dead := pingSources(ctx, w, cfg); dead > 0 {
			// R9 aborts a run when more than half the sources fail, so that is
			// the line at which a dead feed stops being cosmetic.
			enabled := len(cfg.EnabledSources())
			if dead*2 > enabled {
				blocking = append(blocking,
					fmt.Sprintf("%d of %d sources unreachable — ingest would abort (R9)", dead, enabled))
			}
		}
	}

	if len(blocking) > 0 {
		fmt.Fprintln(w)
		return errors.New("doctor: " + strings.Join(blocking, "; "))
	}
	fmt.Fprintln(w, "\nok")
	return nil
}

// pingEmbedder embeds one short string. This is the check that distinguishes a
// key that is present from a key that works — the failure mode that cost a
// whole debugging session when doctor only reported presence.
func pingEmbedder(ctx context.Context, w io.Writer, cfg *config.Config) error {
	fmt.Fprintln(w, "\nembedder ping")

	provider, err := embed.New(cfg)
	if err != nil {
		fmt.Fprintf(w, "  FAIL  %v\n", err)
		return fmt.Errorf("embedder %s: %w", cfg.Embed.Provider, err)
	}

	started := time.Now()
	vectors, err := provider.Embed(ctx, []string{"airfoil doctor connectivity check"})
	took := time.Since(started).Round(time.Millisecond)

	switch {
	case err != nil:
		fmt.Fprintf(w, "  FAIL  %s  %v\n", provider.Name(), err)
		return fmt.Errorf("embedder %s unreachable: %w", provider.Name(), err)
	case len(vectors) != 1 || len(vectors[0]) == 0:
		fmt.Fprintf(w, "  FAIL  %s  returned no vector\n", provider.Name())
		return fmt.Errorf("embedder %s returned no vector", provider.Name())
	default:
		fmt.Fprintf(w, "  ok    %s  %d dims in %s\n", provider.Name(), len(vectors[0]), took)
		return nil
	}
}

// pingLLM asks every configured provider for one token.
//
// Each provider is tried individually rather than through the chain, because
// the chain stops at the first success — which would hide a dead fallback
// until the day the primary goes down and the fallback is needed.
func pingLLM(ctx context.Context, w io.Writer, cfg *config.Config) error {
	fmt.Fprintln(w, "\nllm ping")

	providers := llm.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil))).Providers()
	if len(providers) == 0 {
		fmt.Fprintln(w, "  FAIL  no provider configured")
		return errors.New("no LLM provider configured")
	}

	alive := 0
	for _, p := range providers {
		attempt, cancel := context.WithTimeout(ctx, 90*time.Second)
		started := time.Now()
		out, err := p.Complete(attempt, "Reply with the single word OK.", "Reply now.")
		cancel()
		took := time.Since(started).Round(time.Millisecond)

		switch {
		case err != nil:
			fmt.Fprintf(w, "  FAIL  %-52s %v\n", p.Name(), err)
		default:
			alive++
			fmt.Fprintf(w, "  ok    %-52s %s (%q)\n", p.Name(), took, truncate(out, 24))
		}
	}

	if alive == 0 {
		return errors.New("every LLM provider failed; summarization would skip every cluster")
	}
	return nil
}

// pingSources probes every enabled feed and returns how many are unreachable.
func pingSources(ctx context.Context, w io.Writer, cfg *config.Config) int {
	enabled := cfg.EnabledSources()
	fmt.Fprintf(w, "\nsource ping  (%d enabled)\n", len(enabled))

	dead := 0
	for _, c := range ingest.CheckSources(ctx, cfg, enabled) {
		switch {
		case c.Err != nil:
			dead++
			fmt.Fprintf(w, "  FAIL  t%d %-18s %v\n", c.Source.Tier, c.Source.ID, c.Err)
		case !c.OK():
			dead++
			fmt.Fprintf(w, "  FAIL  t%d %-18s HTTP %d  %s\n",
				c.Source.Tier, c.Source.ID, c.Status, c.Source.URL)
		case c.Fallback():
			fmt.Fprintf(w, "  ok    t%d %-18s %s (via fallback URL)\n",
				c.Source.Tier, c.Source.ID, c.Took.Round(time.Millisecond))
		default:
			fmt.Fprintf(w, "  ok    t%d %-18s %s\n",
				c.Source.Tier, c.Source.ID, c.Took.Round(time.Millisecond))
		}
	}
	if dead > 0 {
		fmt.Fprintf(w, "  %d unreachable\n", dead)
	}
	return dead
}

func present(ok bool) string {
	if ok {
		return "set"
	}
	return "missing"
}
