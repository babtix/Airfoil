// Package pipeline holds the pipeline stages as callable operations.
//
// Both the CLI commands and the TUI drive the same code from here, so a stage
// behaves identically however it is invoked.
package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/papitsho/airfoil/internal/cluster"
	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/embed"
	"github.com/papitsho/airfoil/internal/ingest"
	"github.com/papitsho/airfoil/internal/llm"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/publish"
	"github.com/papitsho/airfoil/internal/score"
	"github.com/papitsho/airfoil/internal/store"
	"github.com/papitsho/airfoil/internal/summarize"
	"github.com/papitsho/airfoil/internal/write"
)

// Pipeline runs stages against one configuration.
type Pipeline struct {
	cfg *config.Config
	log *slog.Logger
}

// New returns a Pipeline bound to cfg.
func New(cfg *config.Config, log *slog.Logger) *Pipeline {
	return &Pipeline{cfg: cfg, log: log}
}

// Config exposes the configuration the stages run against.
func (p *Pipeline) Config() *config.Config { return p.cfg }

// Options carry the flags a stage honours.
type Options struct {
	// Since overrides the window a stage considers. Zero means the configured
	// clustering window.
	Since time.Duration
	// Dry performs the work but writes nothing.
	Dry bool
	// Debug turns on a stage's diagnostic output.
	Debug bool
}

// window resolves the effective time window for a stage.
func (p *Pipeline) window(opts Options) time.Duration {
	if opts.Since > 0 {
		return opts.Since
	}
	return time.Duration(p.cfg.Scoring.Clustering.WindowHours) * time.Hour
}

// ClustersPath is the intermediate handoff from Cluster to Rank. It is
// regenerable and gitignored.
func (p *Pipeline) ClustersPath() string {
	return filepath.Join(p.cfg.DataDir, "clusters.json")
}

// --- ingest -----------------------------------------------------------------

// IngestResult summarizes one ingest pass.
type IngestResult struct {
	NewItems     int
	Written      int
	Days         int
	SourcesOK    int
	SourcesFail  int
	SourceDetail []ingest.SourceResult
	Duration     time.Duration
}

// Ingest fetches every enabled source and files new items under data/items/.
func (p *Pipeline) Ingest(ctx context.Context, opts Options) (IngestResult, error) {
	cfg := p.cfg
	started := time.Now()

	state, err := store.LoadState(cfg.StatePath())
	if err != nil {
		return IngestResult{}, fmt.Errorf("ingest: %w", err)
	}

	// Several feeds serve their entire history — the OpenAI feed alone returns
	// over a thousand entries. Anything older than the clustering window can
	// never join a new cluster, so there is no reason to embed it. Pass a
	// Since to backfill deliberately.
	result, err := ingest.New(cfg, p.log).Run(ctx, state, p.window(opts))
	if err != nil {
		return IngestResult{}, err
	}

	out := IngestResult{
		NewItems:     len(result.Items),
		SourcesOK:    len(result.Sources) - result.Failed,
		SourcesFail:  result.Failed,
		SourceDetail: result.Sources,
		Duration:     time.Since(started),
	}

	p.log.Info("ingest complete",
		"new_items", out.NewItems,
		"sources_ok", out.SourcesOK,
		"sources_failed", out.SourcesFail,
		"seconds", int(out.Duration.Seconds()))

	if opts.Dry {
		p.log.Info("dry run: nothing written")
		return out, nil
	}

	now := time.Now().UTC()
	if len(result.Items) == 0 {
		// Still worth recording that the run happened.
		state.LastRun = &now
		return out, store.SaveState(cfg.StatePath(), state)
	}

	// Items are filed under the day they were published, not the day they were
	// fetched, so a late-arriving item lands in the right bucket.
	byDay := map[string][]model.Item{}
	for _, item := range result.Items {
		day := item.PublishedAt.Format(time.DateOnly)
		byDay[day] = append(byDay[day], item)
	}

	for day, batch := range byDay {
		parsed, err := time.Parse(time.DateOnly, day)
		if err != nil {
			return out, fmt.Errorf("ingest: %w", err)
		}
		added, err := store.AppendItems(cfg.ItemsDir(), parsed, batch)
		if err != nil {
			return out, fmt.Errorf("ingest: %w", err)
		}
		out.Written += added
	}
	out.Days = len(byDay)

	for _, item := range result.Items {
		state.MarkSeen(item.ID, now)
	}
	state.LastRun = &now

	if err := store.SaveState(cfg.StatePath(), state); err != nil {
		return out, fmt.Errorf("ingest: %w", err)
	}

	p.log.Info("wrote items", "added", out.Written, "days", out.Days)
	return out, nil
}

// --- cluster ----------------------------------------------------------------

// ClusterResult summarizes one clustering pass.
type ClusterResult struct {
	Items       int
	Clusters    []model.Cluster
	MultiSource int
	Largest     int
	Borderline  []cluster.Pair
	CacheHits   int
	CacheMisses int
	Dimensions  int
	Provider    string
	Duration    time.Duration
}

// Cluster embeds recent items and groups them into stories.
func (p *Pipeline) Cluster(ctx context.Context, opts Options) (ClusterResult, error) {
	cfg := p.cfg
	sc := cfg.Scoring
	started := time.Now()

	window := p.window(opts)

	// Load a day either side of the window: a story breaking at 23:50 must
	// still group with its follow-up coverage at 00:10.
	days := int(window.Hours()/24) + 2
	items, err := store.LoadItemsSince(cfg.ItemsDir(), time.Now().UTC(), days)
	if err != nil {
		return ClusterResult{}, fmt.Errorf("cluster: %w", err)
	}
	if len(items) == 0 {
		return ClusterResult{}, fmt.Errorf("cluster: no items in %s — run ingest first", cfg.ItemsDir())
	}

	provider, err := embed.New(cfg)
	if err != nil {
		return ClusterResult{}, err
	}
	cache := embed.NewCached(provider, cfg.EmbedCachePath(), sc.Retention.EmbeddingCacheDays)

	texts := make([]string, len(items))
	for i, item := range items {
		texts[i] = embed.Text(item.Title, item.Excerpt)
	}

	p.log.Info("embedding", "provider", provider.Name(), "items", len(items))

	vectors, err := cache.Embed(ctx, texts)
	if err != nil {
		return ClusterResult{}, fmt.Errorf("cluster: %w", err)
	}
	if err := cache.Save(); err != nil {
		// A cache that will not save costs speed, not correctness.
		p.log.Warn("could not save embedding cache", "error", err)
	}

	p.log.Info("embedded",
		"cached", cache.Hits, "fetched", cache.Misses, "dims", provider.Dimensions())

	debugRange := [2]float64{}
	if opts.Debug {
		debugRange = sc.Clustering.DebugRange
	}

	grouped := cluster.Cluster(items, vectors, cluster.Options{
		Threshold: sc.Clustering.SimilarityThreshold,
		Window:    window,
		Now:       time.Now().UTC(),
	}, debugRange)

	out := ClusterResult{
		Items:       len(items),
		Clusters:    grouped.Clusters,
		Borderline:  grouped.Borderline,
		CacheHits:   cache.Hits,
		CacheMisses: cache.Misses,
		Dimensions:  provider.Dimensions(),
		Provider:    provider.Name(),
		Duration:    time.Since(started),
	}
	for _, c := range grouped.Clusters {
		if len(c.Items) > 1 {
			out.MultiSource++
		}
		out.Largest = max(out.Largest, len(c.Items))
	}

	if !opts.Dry {
		if err := store.WriteJSON(p.ClustersPath(), grouped.Clusters); err != nil {
			return out, fmt.Errorf("cluster: %w", err)
		}
	}

	p.log.Info("clustered",
		"items", out.Items, "clusters", len(out.Clusters),
		"multi_source", out.MultiSource, "largest", out.Largest,
		"threshold", sc.Clustering.SimilarityThreshold)

	return out, nil
}

// LoadClusters reads the intermediate produced by Cluster.
func (p *Pipeline) LoadClusters() ([]model.Cluster, error) {
	return store.ReadJSONOr(p.ClustersPath(), []model.Cluster(nil))
}

// --- rank -------------------------------------------------------------------

// RankedPath is the intermediate handoff from Rank to Summarize. It carries the
// full score breakdown, which index.json deliberately does not.
func (p *Pipeline) RankedPath() string {
	return filepath.Join(p.cfg.DataDir, "ranked.json")
}

// RankResult summarizes one scoring pass.
type RankResult struct {
	Results  []score.Result
	Major    int
	Notable  int
	Minor    int
	Builder  int
	Budgeted int // how many would be summarized under the LLM call cap
	Duration time.Duration
}

// Rank scores the clusters produced by Cluster and writes data/ranked.json.
//
// It does not write index.json. That file is served to the live site, and its
// entries carry summaries that do not exist until the summarize stage has run —
// publishing half-built entries would degrade a working site mid-pipeline.
func (p *Pipeline) Rank(_ context.Context, opts Options) (RankResult, error) {
	started := time.Now()

	clusters, err := p.LoadClusters()
	if err != nil {
		return RankResult{}, fmt.Errorf("rank: %w", err)
	}
	if len(clusters) == 0 {
		return RankResult{}, fmt.Errorf("rank: no clusters in %s — run cluster first", p.ClustersPath())
	}

	sc := p.cfg.Scoring
	results := score.Rank(clusters, sc, p.cfg.Keywords, time.Now().UTC())

	out := RankResult{Results: results, Duration: time.Since(started)}
	for _, r := range results {
		switch r.Tier {
		case model.TierMajor:
			out.Major++
		case model.TierNotable:
			out.Notable++
		default:
			out.Minor++
		}
		if r.BuilderRelevant {
			out.Builder++
		}
	}

	// Minor stories are index entries only — they never cost an LLM call.
	out.Budgeted = min(out.Major+out.Notable, sc.Caps.LLMCallsPerRun)

	if !opts.Dry {
		if err := store.WriteJSON(p.RankedPath(), results); err != nil {
			return out, fmt.Errorf("rank: %w", err)
		}
	}

	p.log.Info("ranked",
		"clusters", len(results),
		"major", out.Major, "notable", out.Notable, "minor", out.Minor,
		"builder_relevant", out.Builder,
		"llm_budget", out.Budgeted)

	return out, nil
}

// LoadRanked reads the intermediate produced by Rank.
func (p *Pipeline) LoadRanked() ([]score.Result, error) {
	return store.ReadJSONOr(p.RankedPath(), []score.Result(nil))
}

// --- write ------------------------------------------------------------------

// WriteResult summarizes one summarize-and-write pass.
type WriteResult struct {
	Stats      write.Stats
	Summarized int
	Skipped    int
	IndexOnly  int
	Providers  []string
	Duration   time.Duration
}

// Write summarizes the top-ranked clusters and publishes all three outputs.
func (p *Pipeline) Write(ctx context.Context, opts Options) (WriteResult, error) {
	started := time.Now()

	ranked, err := p.LoadRanked()
	if err != nil {
		return WriteResult{}, fmt.Errorf("write: %w", err)
	}
	if len(ranked) == 0 {
		return WriteResult{}, fmt.Errorf("write: no rankings in %s — run rank first", p.RankedPath())
	}

	clusters, err := p.LoadClusters()
	if err != nil {
		return WriteResult{}, fmt.Errorf("write: %w", err)
	}
	byID := make(map[string]model.Cluster, len(clusters))
	for _, c := range clusters {
		byID[c.ID] = c
	}

	chain := llm.New(p.cfg, p.log)
	names := make([]string, 0, len(chain.Providers()))
	for _, prov := range chain.Providers() {
		names = append(names, prov.Name())
	}
	if len(names) == 0 {
		return WriteResult{}, fmt.Errorf("write: no LLM provider configured")
	}
	p.log.Info("llm chain", "providers", names)

	results := summarize.New(chain, p.cfg, p.log).Run(ctx, ranked, byID)

	out := WriteResult{Providers: names}
	for _, r := range results {
		switch {
		case r.Summarized:
			out.Summarized++
		case r.Err != nil:
			out.Skipped++
		default:
			out.IndexOnly++
		}
	}

	stats, err := write.New(p.cfg, p.log).Run(results, time.Now().UTC(), opts.Dry)
	if err != nil {
		return out, err
	}
	out.Stats = stats
	out.Duration = time.Since(started)

	return out, nil
}

// --- publish ----------------------------------------------------------------

// Publish validates the generated output and commits it.
func (p *Pipeline) Publish(ctx context.Context, opts publish.Options, repoRoot string) (publish.Result, error) {
	return publish.New(p.cfg, p.log, repoRoot).Run(ctx, opts, time.Now().UTC())
}
