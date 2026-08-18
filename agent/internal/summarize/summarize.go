package summarize

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/score"
)

// Completer is the transport the summarizer drives. internal/llm.Chain
// satisfies it; tests supply a stub.
type Completer interface {
	Complete(ctx context.Context, sys, user string) (string, error)
}

// Summarizer turns scored clusters into validated stories.
type Summarizer struct {
	llm Completer
	cfg *config.Config
	log *slog.Logger
}

func New(llm Completer, cfg *config.Config, log *slog.Logger) *Summarizer {
	return &Summarizer{llm: llm, cfg: cfg, log: log}
}

// ErrSkipped means the cluster could not be summarized and should become an
// index entry only. It is never fatal to a run.
var ErrSkipped = errors.New("summarize: cluster skipped")

// One summarizes a single cluster, retrying once on a validation failure.
//
// PROMPTS.md allows exactly one retry: a model that returns an invalid summary
// twice is unlikely to fix itself on a third attempt, and each try costs a call
// against the per-run budget.
func (s *Summarizer) One(ctx context.Context, c model.Cluster, r score.Result) (Response, error) {
	sys := SystemPrompt
	user := UserPrompt(c, r)

	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return Response{}, ctx.Err()
			case <-time.After(750 * time.Millisecond):
			}
		}

		raw, err := s.llm.Complete(ctx, sys, user)
		if err != nil {
			// A transport failure is not a validation failure: the whole chain
			// is down, so retrying this cluster will not help.
			return Response{}, fmt.Errorf("%w: %v", ErrSkipped, err)
		}

		parsed, err := Parse(raw)
		if err == nil {
			var clean Response
			clean, err = Validate(parsed, c.Items, s.cfg.Scoring)
			if err == nil {
				if attempt > 1 {
					s.log.Info("summary valid on retry", "cluster", c.ID)
				}
				return clean, nil
			}
		}

		lastErr = err
		s.log.Warn("summary rejected",
			"cluster", c.ID, "attempt", attempt, "error", err)
	}

	return Response{}, fmt.Errorf("%w: %v", ErrSkipped, lastErr)
}

// Result pairs a scored cluster with its summary outcome.
type Result struct {
	Score   score.Result
	Cluster model.Cluster
	Summary Response
	// Summarized is false when the cluster became an index entry only.
	Summarized bool
	Err        error
}

// Run summarizes the highest-scoring clusters within the per-run call budget.
//
// Clusters below the notable cutoff are never sent: PROMPTS.md reserves LLM
// calls for stories that will actually be published as prose, and BUILD_SPEC
// caps spend at caps.llm_calls_per_run.
func (s *Summarizer) Run(ctx context.Context, ranked []score.Result, clusters map[string]model.Cluster) []Result {
	sc := s.cfg.Scoring
	budget := sc.Caps.LLMCallsPerRun

	out := make([]Result, 0, len(ranked))
	used := 0

	for _, r := range ranked {
		c, ok := clusters[r.ClusterID]
		if !ok {
			s.log.Warn("ranked cluster missing from clusters.json", "cluster", r.ClusterID)
			continue
		}

		res := Result{Score: r, Cluster: c}

		switch {
		case r.Score < sc.Tiers.NotableMinScore:
			// Minor: index entry only, no call. Not an error.
		case used >= budget:
			// Over budget. Also not an error — the rest simply wait for the
			// next run, by which time their scores will have decayed.
			res.Err = fmt.Errorf("%w: over the %d-call budget", ErrSkipped, budget)
		default:
			used++
			summary, err := s.One(ctx, c, r)
			if err != nil {
				res.Err = err
			} else {
				res.Summary = summary
				res.Summarized = true
			}
		}

		out = append(out, res)
	}

	s.log.Info("summarized", "calls", used, "budget", budget, "clusters", len(out))
	return out
}
