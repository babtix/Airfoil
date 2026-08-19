package summarize

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/score"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// stubLLM returns queued responses in order, then repeats the last one.
type stubLLM struct {
	replies []string
	err     error
	calls   int
}

func (s *stubLLM) Complete(_ context.Context, _, _ string) (string, error) {
	s.calls++
	if s.err != nil {
		return "", s.err
	}
	if len(s.replies) == 0 {
		return "", nil
	}
	if s.calls-1 < len(s.replies) {
		return s.replies[s.calls-1], nil
	}
	return s.replies[len(s.replies)-1], nil
}

func testConfig() *config.Config {
	var sc config.Scoring
	sc.Limits.SummaryMaxWords = 80
	sc.Limits.VerbatimOverlapMaxWords = 12
	sc.Limits.QuotesMax = 1
	sc.Limits.QuoteMaxWords = 15
	sc.Limits.TakeawaysMax = 3
	sc.Tiers.MajorMinScore = 70
	sc.Tiers.NotableMinScore = 40
	sc.Caps.LLMCallsPerRun = 15
	return &config.Config{Scoring: sc}
}

func testCluster(id string) model.Cluster {
	return model.Cluster{ID: id, Items: []model.Item{{
		ID: id + "-item", SourceID: "src", SourceName: "Src", SourceTier: 1,
		URL: "https://example.com/" + id, Title: "A title",
		Excerpt:     "Unrelated source text that the summary will not reuse.",
		PublishedAt: time.Now().UTC(),
	}}}
}

const goodJSON = `{"title":"A title","summary":"An original summary.",` +
	`"takeaways":["one"],"tags":["models"],"builder_relevant":true,"confidence":"high"}`

func TestOneAcceptsValidResponse(t *testing.T) {
	s := New(&stubLLM{replies: []string{goodJSON}}, testConfig(), quietLogger())

	got, err := s.One(context.Background(), testCluster("c1"), score.Result{})
	if err != nil {
		t.Fatalf("One() error = %v", err)
	}
	if got.Summary != "An original summary." {
		t.Errorf("Summary = %q", got.Summary)
	}
}

func TestOneRetriesOnceThenSucceeds(t *testing.T) {
	stub := &stubLLM{replies: []string{"not json at all", goodJSON}}
	s := New(stub, testConfig(), quietLogger())

	if _, err := s.One(context.Background(), testCluster("c1"), score.Result{}); err != nil {
		t.Fatalf("One() error = %v", err)
	}
	if stub.calls != 2 {
		t.Errorf("calls = %d, want 2 (one retry)", stub.calls)
	}
}

func TestOneGivesUpAfterTwoAttempts(t *testing.T) {
	stub := &stubLLM{replies: []string{"garbage"}}
	s := New(stub, testConfig(), quietLogger())

	_, err := s.One(context.Background(), testCluster("c1"), score.Result{})
	if !errors.Is(err, ErrSkipped) {
		t.Fatalf("error = %v, want ErrSkipped", err)
	}
	if stub.calls != 2 {
		t.Errorf("calls = %d, want exactly 2 — the budget allows one retry", stub.calls)
	}
}

func TestOneDoesNotRetryTransportFailure(t *testing.T) {
	// The whole chain being down is not something a retry can fix, and each
	// attempt costs budget.
	stub := &stubLLM{err: errors.New("every provider failed")}
	s := New(stub, testConfig(), quietLogger())

	_, err := s.One(context.Background(), testCluster("c1"), score.Result{})
	if !errors.Is(err, ErrSkipped) {
		t.Fatalf("error = %v, want ErrSkipped", err)
	}
	if stub.calls != 1 {
		t.Errorf("calls = %d, want 1 — a dead chain should not be retried", stub.calls)
	}
}

func TestRunSkipsMinorClustersWithoutCallingLLM(t *testing.T) {
	stub := &stubLLM{replies: []string{goodJSON}}
	s := New(stub, testConfig(), quietLogger())

	ranked := []score.Result{
		{ClusterID: "hi", Score: 75, Tier: model.TierMajor},
		{ClusterID: "lo", Score: 12, Tier: model.TierMinor},
	}
	clusters := map[string]model.Cluster{
		"hi": testCluster("hi"),
		"lo": testCluster("lo"),
	}

	out := s.Run(context.Background(), ranked, clusters)

	if len(out) != 2 {
		t.Fatalf("len = %d, want 2", len(out))
	}
	if stub.calls != 1 {
		t.Errorf("calls = %d, want 1 — minor stories must not cost an LLM call", stub.calls)
	}
	if !out[0].Summarized {
		t.Error("major story was not summarized")
	}
	if out[1].Summarized {
		t.Error("minor story was summarized")
	}
	if out[1].Err != nil {
		t.Errorf("minor story recorded an error: %v — index-only is not a failure", out[1].Err)
	}
}

func TestRunRespectsCallBudget(t *testing.T) {
	cfg := testConfig()
	cfg.Scoring.Caps.LLMCallsPerRun = 2

	stub := &stubLLM{replies: []string{goodJSON}}
	s := New(stub, cfg, quietLogger())

	var ranked []score.Result
	clusters := map[string]model.Cluster{}
	for _, id := range []string{"a", "b", "c", "d"} {
		ranked = append(ranked, score.Result{ClusterID: id, Score: 80, Tier: model.TierMajor})
		clusters[id] = testCluster(id)
	}

	out := s.Run(context.Background(), ranked, clusters)

	if stub.calls != 2 {
		t.Errorf("calls = %d, want 2 (the configured budget)", stub.calls)
	}
	summarized := 0
	for _, r := range out {
		if r.Summarized {
			summarized++
		}
	}
	if summarized != 2 {
		t.Errorf("summarized = %d, want 2", summarized)
	}
}

func TestRunSkipsRankedClusterMissingFromClusters(t *testing.T) {
	s := New(&stubLLM{replies: []string{goodJSON}}, testConfig(), quietLogger())

	ranked := []score.Result{{ClusterID: "ghost", Score: 90, Tier: model.TierMajor}}
	out := s.Run(context.Background(), ranked, map[string]model.Cluster{})

	if len(out) != 0 {
		t.Errorf("len = %d, want 0 — a missing cluster should be dropped, not panic", len(out))
	}
}
