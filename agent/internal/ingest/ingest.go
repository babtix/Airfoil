// Package ingest fetches items from every configured source.
//
// Each adapter runs in its own goroutine with its own timeout. A source that
// fails logs a warning and contributes nothing — one broken feed must never
// fail a run. If more than half of all sources fail, the run aborts before
// anything is written (R9).
package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/normalize"
)

const (
	// sourceTimeout bounds one source. A slow feed must not hold up the run.
	sourceTimeout = 15 * time.Second

	// maxConcurrent keeps us from opening seventeen sockets at once, which
	// looks like abuse to the smaller hosts.
	maxConcurrent = 6
)

// Adapter fetches raw items for one configured source.
type Adapter interface {
	// Type is the source type in sources.json that this adapter handles.
	Type() string
	// Fetch returns raw items. Returning an empty slice is not an error.
	Fetch(ctx context.Context, src config.Source) ([]normalize.Raw, error)
}

// Runner executes every enabled source and normalizes the results.
type Runner struct {
	cfg      *config.Config
	log      *slog.Logger
	adapters map[string]Adapter
	now      func() time.Time
}

// New builds a Runner with the standard adapter set.
func New(cfg *config.Config, log *slog.Logger) *Runner {
	f := newFetcher(sourceTimeout, defaultUserAgent)

	adapters := []Adapter{
		newRSSAdapter(f),
		newHNAdapter(f),
		newRedditAdapter(f, cfg.Ingest.RedditUserAgent),
		newHFAdapter(f),
		newHFModelsAdapter(f),
		newGitHubAdapter(f, cfg.Ingest.GitHubToken),
	}

	byType := make(map[string]Adapter, len(adapters))
	for _, a := range adapters {
		byType[a.Type()] = a
	}

	return &Runner{cfg: cfg, log: log, adapters: byType, now: time.Now}
}

// defaultUserAgent identifies the agent to every host we contact (R6). Reddit
// wants its own, which the Reddit adapter sets per request.
const defaultUserAgent = "airfoil/0.1 (+https://github.com/papitsho/airfoil)"

// SourceResult records what one source contributed.
type SourceResult struct {
	SourceID string
	Fetched  int // raw items returned
	Kept     int // items that survived normalization
	Duration time.Duration
	Err      error
}

// Result is the outcome of one ingest pass.
type Result struct {
	// Items are new, normalized, and deduplicated against state (R7).
	Items   []model.Item
	Sources []SourceResult
	Failed  int
}

// Run fetches every enabled source concurrently and returns the new items.
//
// state is read, not written: Run reports what is new, and the caller decides
// whether to commit that to disk.
func (r *Runner) Run(ctx context.Context, state *model.State, since time.Duration) (Result, error) {
	sources := r.cfg.EnabledSources()
	if len(sources) == 0 {
		return Result{}, fmt.Errorf("ingest: no enabled sources")
	}

	now := r.now().UTC()
	results := make([]SourceResult, len(sources))
	raws := make([][]normalize.Raw, len(sources))

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)

	for i, src := range sources {
		g.Go(func() error {
			started := time.Now()
			out, err := r.fetchOne(gctx, src)

			results[i] = SourceResult{
				SourceID: src.ID,
				Fetched:  len(out),
				Duration: time.Since(started),
				Err:      err,
			}
			raws[i] = out

			// The error is recorded, not returned: one bad source must not
			// cancel the group.
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return Result{}, fmt.Errorf("ingest: %w", err)
	}

	// Normalize in source order so that the result is deterministic regardless
	// of which goroutine finished first.
	var all []model.Item
	failed := 0
	for i, src := range sources {
		if results[i].Err != nil {
			failed++
			r.log.Warn("source failed", "source", src.ID, "error", results[i].Err)
			continue
		}

		kept := 0
		for _, raw := range raws[i] {
			item, err := normalize.Build(raw, now, r.cfg.Scoring.Limits.ExcerptMaxChars)
			if err != nil {
				r.log.Debug("skipped item", "source", src.ID, "error", err)
				continue
			}
			if since > 0 && item.PublishedAt.Before(now.Add(-since)) {
				continue
			}
			all = append(all, item)
			kept++
		}
		results[i].Kept = kept
		r.log.Info("source ok", "source", src.ID,
			"fetched", results[i].Fetched, "kept", kept,
			"ms", results[i].Duration.Milliseconds())
	}

	// R9: a run where most sources are down would produce a misleading digest.
	// Better to write nothing than to publish a half-empty day as if it were
	// the whole picture.
	if failed*2 > len(sources) {
		return Result{}, fmt.Errorf("ingest: %d of %d sources failed; aborting before write",
			failed, len(sources))
	}

	// Highest-tier source first, so that a duplicate resolves to the most
	// authoritative version of the story.
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].SourceTier < all[j].SourceTier
	})
	items := normalize.Dedupe(all, state.Seen)

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})

	return Result{Items: items, Sources: results, Failed: failed}, nil
}

// fetchOne runs a single source under its own timeout.
func (r *Runner) fetchOne(ctx context.Context, src config.Source) ([]normalize.Raw, error) {
	adapter, ok := r.adapters[src.Type]
	if !ok {
		return nil, fmt.Errorf("no adapter for type %q", src.Type)
	}

	ctx, cancel := context.WithTimeout(ctx, sourceTimeout)
	defer cancel()

	out, err := adapter.Fetch(ctx, src)
	if err != nil {
		return nil, err
	}
	return out, nil
}
