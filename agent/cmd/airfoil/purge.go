package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/purge"
	"github.com/papitsho/airfoil/internal/store"
	"github.com/papitsho/airfoil/internal/write"
)

func newPurgeCmd(a *app) *cobra.Command {
	var (
		beforeStr string
		tags      []string
		sources   []string
		tier      string
		maxScore  int
		slugFile  string
		query     string
		dry       bool
		force     bool
	)

	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Bulk delete stories by age, tag, source, tier, or slug manifest",
		Long: "Purge removes matching stories from data/stories.json and data/index.json,\n" +
			"and deletes their corresponding markdown files under site/src/content/stories/.\n\n" +
			"Examples:\n" +
			"  airfoil purge --before 90d --dry\n" +
			"  airfoil purge --before 3m\n" +
			"  airfoil purge --tag crypto --tier minor\n" +
			"  airfoil purge --slug-file purge-manifest.json",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPurge(a, cmd.OutOrStdout(), purgeCmdOptions{
				beforeStr: beforeStr,
				tags:      tags,
				sources:   sources,
				tier:      tier,
				maxScore:  maxScore,
				slugFile:  slugFile,
				query:     query,
				dry:       dry,
				force:     force,
			})
		},
	}

	f := cmd.Flags()
	f.StringVar(&beforeStr, "before", "", "delete stories older than duration or date (e.g. 90d, 3m, 1y, 2026-01-01)")
	f.StringSliceVar(&tags, "tag", nil, "delete stories matching any of these tags (repeatable or comma-separated)")
	f.StringSliceVar(&sources, "source", nil, "delete stories from any of these sources")
	f.StringVar(&tier, "tier", "", "delete stories of this tier (minor, notable, major)")
	f.IntVar(&maxScore, "max-score", -1, "delete stories with score <= this value")
	f.StringVar(&slugFile, "slug-file", "", "path to JSON manifest file containing list of slugs to delete")
	f.StringVar(&query, "query", "", "delete stories containing query text in title, summary or tags")
	f.BoolVar(&dry, "dry", false, "preview matching stories without modifying files")
	f.BoolVar(&force, "force", false, "allow deleting all stories without safety prompt")

	return cmd
}

type purgeCmdOptions struct {
	beforeStr string
	tags      []string
	sources   []string
	tier      string
	maxScore  int
	slugFile  string
	query     string
	dry       bool
	force     bool
}

func runPurge(a *app, out io.Writer, opts purgeCmdOptions) error {
	storiesPath := a.cfg.StoriesPath()
	stories, err := store.ReadJSONOr(storiesPath, []model.Story(nil))
	if err != nil {
		return fmt.Errorf("read stories: %w", err)
	}
	if len(stories) == 0 {
		fmt.Fprintln(out, "No stories found in data/stories.json.")
		return nil
	}

	var slugs []string
	if opts.slugFile != "" {
		slugs, err = loadSlugManifest(opts.slugFile)
		if err != nil {
			return fmt.Errorf("read slug manifest %q: %w", opts.slugFile, err)
		}
	}

	var beforeCutoff time.Time
	if opts.beforeStr != "" {
		beforeCutoff, err = purge.ParseBefore(opts.beforeStr, time.Now())
		if err != nil {
			return err
		}
	}

	var maxScorePtr *int
	if opts.maxScore >= 0 {
		ms := opts.maxScore
		maxScorePtr = &ms
	}

	filterOpts := purge.Options{
		Before:   beforeCutoff,
		Tags:     opts.tags,
		Sources:  opts.sources,
		Tier:     opts.tier,
		MaxScore: maxScorePtr,
		Slugs:    slugs,
		Query:    opts.query,
	}

	if filterOpts.IsEmpty() {
		return fmt.Errorf("no filter criteria specified. Provide --before, --tag, --source, --tier, --max-score, --query, or --slug-file")
	}

	result := purge.Filter(stories, filterOpts)

	if len(result.Removed) == 0 {
		fmt.Fprintln(out, "No stories matched the purge criteria.")
		return nil
	}

	if len(result.Kept) == 0 && !opts.force {
		return fmt.Errorf("purge would remove all %d stories in the database. Use --force if you really want to delete everything", len(stories))
	}

	fmt.Fprintf(out, "Matched %d stories for deletion (%d to keep):\n", len(result.Removed), len(result.Kept))
	for _, s := range result.Removed {
		fmt.Fprintf(out, "  - [%s] (Score: %d, Tier: %s) %s (%s)\n",
			s.Date.Format("2006-01-02"), s.Score, s.Tier, s.Title, s.Slug)
	}

	if opts.dry {
		fmt.Fprintf(out, "\n[DRY RUN] Would purge %d stories. No files modified.\n", len(result.Removed))
		return nil
	}

	// Write updated stories.json
	if err := store.WriteJSON(a.cfg.StoriesPath(), result.Kept); err != nil {
		return fmt.Errorf("write stories: %w", err)
	}

	// Write updated index.json
	idx := write.BuildIndex(result.Kept, time.Now())
	if err := store.WriteJSON(a.cfg.IndexPath(), idx); err != nil {
		return fmt.Errorf("write index: %w", err)
	}

	// Remove corresponding markdown files
	removedMDCount := 0
	for _, s := range result.Removed {
		mdPath := filepath.Join(a.cfg.StoriesDir, s.Slug+".md")
		if store.Exists(mdPath) {
			if err := os.Remove(mdPath); err != nil {
				a.log.Warn("failed to remove markdown file", "path", mdPath, "err", err)
			} else {
				removedMDCount++
			}
		}
	}

	fmt.Fprintf(out, "\n✓ Successfully purged %d stories (%d remaining in index, %d markdown files deleted).\n",
		len(result.Removed), len(result.Kept), removedMDCount)

	return nil
}

func loadSlugManifest(path string) ([]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Try []string format first: ["slug1", "slug2"]
	var list []string
	if err := json.Unmarshal(b, &list); err == nil {
		return list, nil
	}

	// Try object format: {"slugs": ["slug1", "slug2"]}
	var obj struct {
		Slugs []string `json:"slugs"`
	}
	if err := json.Unmarshal(b, &obj); err == nil && len(obj.Slugs) > 0 {
		return obj.Slugs, nil
	}

	return nil, fmt.Errorf("invalid manifest JSON; expected array of strings or object with 'slugs' array")
}
