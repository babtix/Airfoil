package ingest

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/normalize"
)

// stubAdapter returns canned raws, or an error, without touching the network.
type stubAdapter struct {
	typ  string
	raws map[string][]normalize.Raw // by source ID
	errs map[string]error           // by source ID
}

func (s *stubAdapter) Type() string { return s.typ }

func (s *stubAdapter) Fetch(_ context.Context, src config.Source) ([]normalize.Raw, error) {
	if err := s.errs[src.ID]; err != nil {
		return nil, err
	}
	return s.raws[src.ID], nil
}

func testRunner(t *testing.T, sources []config.Source, stub *stubAdapter) *Runner {
	t.Helper()

	cfg := &config.Config{Sources: sources}
	cfg.Scoring.Limits.ExcerptMaxChars = 300

	return &Runner{
		cfg:      cfg,
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		adapters: map[string]Adapter{stub.typ: stub},
		now:      func() time.Time { return time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC) },
	}
}

func src(id string, tier int) config.Source {
	return config.Source{ID: id, Name: id, Type: "stub", URL: "https://" + id, Tier: tier, Enabled: true}
}

func raw(id, url, title string, tier int) normalize.Raw {
	return normalize.Raw{
		SourceID: id, SourceName: id, SourceTier: tier,
		URL: url, Title: title,
		PublishedAt: time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC),
	}
}

func TestRunnerCollectsFromEverySource(t *testing.T) {
	sources := []config.Source{src("a", 1), src("b", 4)}
	stub := &stubAdapter{typ: "stub", raws: map[string][]normalize.Raw{
		"a": {raw("a", "https://a.com/1", "First", 1)},
		"b": {raw("b", "https://b.com/1", "Second", 4), raw("b", "https://b.com/2", "Third", 4)},
	}}

	got, err := testRunner(t, sources, stub).Run(context.Background(), model.NewState(), 0)
	if err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if len(got.Items) != 3 {
		t.Fatalf("got %d items, want 3", len(got.Items))
	}
	if got.Failed != 0 {
		t.Errorf("Failed = %d, want 0", got.Failed)
	}
}

// One broken feed must never fail a run.
func TestRunnerContinuesPastOneFailure(t *testing.T) {
	sources := []config.Source{src("a", 1), src("b", 4), src("c", 5)}
	stub := &stubAdapter{
		typ: "stub",
		raws: map[string][]normalize.Raw{
			"a": {raw("a", "https://a.com/1", "First", 1)},
			"c": {raw("c", "https://c.com/1", "Third", 5)},
		},
		errs: map[string]error{"b": errors.New("404 Not Found")},
	}

	got, err := testRunner(t, sources, stub).Run(context.Background(), model.NewState(), 0)
	if err != nil {
		t.Fatalf("Run() = %v, want the run to survive one bad source", err)
	}
	if len(got.Items) != 2 {
		t.Errorf("got %d items, want 2", len(got.Items))
	}
	if got.Failed != 1 {
		t.Errorf("Failed = %d, want 1", got.Failed)
	}
}

// R9: a run where most sources are down would publish a half-empty day as if
// it were the whole picture, so it must write nothing at all.
func TestRunnerAbortsWhenMostSourcesFail(t *testing.T) {
	sources := []config.Source{src("a", 1), src("b", 4), src("c", 5)}
	stub := &stubAdapter{
		typ:  "stub",
		raws: map[string][]normalize.Raw{"a": {raw("a", "https://a.com/1", "First", 1)}},
		errs: map[string]error{
			"b": errors.New("timeout"),
			"c": errors.New("503"),
		},
	}

	_, err := testRunner(t, sources, stub).Run(context.Background(), model.NewState(), 0)
	if err == nil {
		t.Fatal("Run() = nil, want an abort")
	}
	if !strings.Contains(err.Error(), "aborting before write") {
		t.Errorf("error = %v, want it to say the run aborted", err)
	}
}

func TestRunnerHalfFailingIsNotAnAbort(t *testing.T) {
	// Two of four is not "more than half".
	sources := []config.Source{src("a", 1), src("b", 4), src("c", 5), src("d", 5)}
	stub := &stubAdapter{
		typ: "stub",
		raws: map[string][]normalize.Raw{
			"a": {raw("a", "https://a.com/1", "First", 1)},
			"b": {raw("b", "https://b.com/1", "Second", 4)},
		},
		errs: map[string]error{"c": errors.New("x"), "d": errors.New("y")},
	}

	if _, err := testRunner(t, sources, stub).Run(context.Background(), model.NewState(), 0); err != nil {
		t.Fatalf("Run() = %v, want the run to proceed", err)
	}
}

// R7: everything state has already seen is dropped, so a repeat run is empty.
func TestRunnerDropsSeenItems(t *testing.T) {
	sources := []config.Source{src("a", 1)}
	stub := &stubAdapter{typ: "stub", raws: map[string][]normalize.Raw{
		"a": {raw("a", "https://a.com/1", "First", 1), raw("a", "https://a.com/2", "Second", 1)},
	}}
	runner := testRunner(t, sources, stub)

	state := model.NewState()
	first, err := runner.Run(context.Background(), state, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 {
		t.Fatalf("first run got %d items, want 2", len(first.Items))
	}

	for _, item := range first.Items {
		state.MarkSeen(item.ID, time.Now())
	}

	second, err := runner.Run(context.Background(), state, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 0 {
		t.Errorf("second run got %d items, want 0", len(second.Items))
	}
}

// Two feeds carrying the same article collapse to one item, keeping the more
// authoritative source.
func TestRunnerDeduplicatesAcrossSources(t *testing.T) {
	sources := []config.Source{src("press", 4), src("lab", 1)}
	stub := &stubAdapter{typ: "stub", raws: map[string][]normalize.Raw{
		"press": {raw("press", "https://lab.com/post?utm_source=x", "Lab ships a model", 4)},
		"lab":   {raw("lab", "https://lab.com/post", "Lab ships a model", 1)},
	}}

	got, err := testRunner(t, sources, stub).Run(context.Background(), model.NewState(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(got.Items))
	}
	if got.Items[0].SourceTier != 1 {
		t.Errorf("kept the tier %d copy, want the tier 1 source", got.Items[0].SourceTier)
	}
}

func TestRunnerAppliesWindow(t *testing.T) {
	old := raw("a", "https://a.com/old", "Old news", 1)
	old.PublishedAt = time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

	sources := []config.Source{src("a", 1)}
	stub := &stubAdapter{typ: "stub", raws: map[string][]normalize.Raw{
		"a": {raw("a", "https://a.com/new", "New", 1), old},
	}}
	runner := testRunner(t, sources, stub)

	all, err := runner.Run(context.Background(), model.NewState(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Items) != 2 {
		t.Fatalf("with no window got %d items, want 2", len(all.Items))
	}

	windowed, err := runner.Run(context.Background(), model.NewState(), 48*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(windowed.Items) != 1 {
		t.Errorf("with a 48h window got %d items, want 1", len(windowed.Items))
	}
}

func TestRunnerUnknownSourceTypeFailsThatSourceOnly(t *testing.T) {
	sources := []config.Source{src("a", 1), {ID: "weird", Name: "weird", Type: "gopher", URL: "x", Tier: 4, Enabled: true}}
	stub := &stubAdapter{typ: "stub", raws: map[string][]normalize.Raw{
		"a": {raw("a", "https://a.com/1", "First", 1)},
	}}

	got, err := testRunner(t, sources, stub).Run(context.Background(), model.NewState(), 0)
	if err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if got.Failed != 1 {
		t.Errorf("Failed = %d, want 1", got.Failed)
	}
	if len(got.Items) != 1 {
		t.Errorf("got %d items, want the working source's item", len(got.Items))
	}
}

func TestRunnerNoEnabledSources(t *testing.T) {
	stub := &stubAdapter{typ: "stub"}
	if _, err := testRunner(t, nil, stub).Run(context.Background(), model.NewState(), 0); err == nil {
		t.Fatal("Run() = nil, want an error")
	}
}

// The result must not depend on which goroutine finished first.
func TestRunnerIsDeterministic(t *testing.T) {
	sources := []config.Source{src("a", 1), src("b", 4), src("c", 5)}
	stub := &stubAdapter{typ: "stub", raws: map[string][]normalize.Raw{
		"a": {raw("a", "https://a.com/1", "A", 1)},
		"b": {raw("b", "https://b.com/1", "B", 4)},
		"c": {raw("c", "https://c.com/1", "C", 5)},
	}}
	runner := testRunner(t, sources, stub)

	var first []string
	for run := range 5 {
		got, err := runner.Run(context.Background(), model.NewState(), 0)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(got.Items))
		for i, item := range got.Items {
			ids[i] = item.ID
		}
		if run == 0 {
			first = ids
			continue
		}
		if strings.Join(ids, ",") != strings.Join(first, ",") {
			t.Fatalf("run %d produced %v, want %v", run, ids, first)
		}
	}
}
