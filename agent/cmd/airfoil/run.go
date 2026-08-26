package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/pipeline"
	"github.com/papitsho/airfoil/internal/publish"
)

func newRunCmd(a *app) *cobra.Command {
	var (
		dry      bool
		push     bool
		loop     bool
		interval string
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the whole pipeline: ingest, cluster, rank, write",
		Long: "Run chains every stage in order.\n\n" +
			"--dry does everything except write files, call the LLM chain, or\n" +
			"commit. Publishing is opt-in: without --push nothing leaves the\n" +
			"machine, and the commit step only runs when --push is given.\n\n" +
			"--loop runs repeatedly on a recurring interval (default 24h).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dur := 24 * time.Hour
			if interval != "" {
				parsed, err := time.ParseDuration(interval)
				if err != nil {
					return fmt.Errorf("--interval %q: invalid duration: %w", interval, err)
				}
				if parsed <= 0 {
					return fmt.Errorf("--interval %q: must be positive", interval)
				}
				dur = parsed
			}
			return runAll(cmd.Context(), a, cmd.OutOrStdout(), dry, push, loop, dur)
		},
	}
	cmd.Flags().BoolVar(&dry, "dry", false, "run every stage without writing, summarizing, or committing")
	cmd.Flags().BoolVar(&push, "push", false, "commit and push the result when the run succeeds")
	cmd.Flags().BoolVar(&loop, "loop", false, "run repeatedly in a continuous loop")
	cmd.Flags().StringVar(&interval, "interval", "24h", "loop interval duration, e.g. 24h, 12h, 6h, 1h")

	return cmd
}

func runAll(ctx context.Context, a *app, out io.Writer, dry, push, loop bool, interval time.Duration) error {
	for {
		err := runSinglePass(ctx, a, out, dry, push)
		if err != nil {
			if !loop {
				return err
			}
			fmt.Fprintf(out, "\nrun error: %v (will retry in %s)\n", err, interval)
		}
		if !loop {
			return nil
		}
		fmt.Fprintf(out, "\n[loop active] sleeping for %s until next run (press Ctrl+C to stop)...\n", interval)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
			fmt.Fprintln(out, "\n[loop trigger] starting scheduled pipeline run...")
		}
	}
}

func runSinglePass(ctx context.Context, a *app, out io.Writer, dry, push bool) error {
	p := a.pipeline()
	opts := pipeline.Options{Since: a.since, Dry: dry}

	ing, err := p.Ingest(ctx, opts)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	fmt.Fprintf(out, "ingest   %d new items from %d/%d sources\n",
		ing.NewItems, ing.SourcesOK, ing.SourcesOK+ing.SourcesFail)

	// A dry run stops here: clustering needs items on disk, and ingest wrote
	// none. Continuing would cluster whatever the previous run left behind
	// and report it as though this run produced it.
	if dry {
		fmt.Fprintln(out, "\ndry run: stopping before cluster — nothing was written to disk")
		return nil
	}

	cl, err := p.Cluster(ctx, opts)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	fmt.Fprintf(out, "cluster  %d items into %d clusters (%d multi-item)\n",
		cl.Items, len(cl.Clusters), cl.MultiSource)

	rk, err := p.Rank(ctx, opts)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	fmt.Fprintf(out, "rank     %d major, %d notable, %d minor\n",
		rk.Major, rk.Notable, rk.Minor)

	wr, err := p.Write(ctx, opts)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	fmt.Fprintf(out, "write    %d summarized, %d created, %d updated\n",
		wr.Summarized, wr.Stats.Created, wr.Stats.Updated)

	if !push {
		fmt.Fprintln(out, "\nnot published. Run `airfoil publish` to validate and commit.")
		return nil
	}

	root, err := repoRoot(a)
	if err != nil {
		return err
	}
	pub, err := p.Publish(ctx, publish.Options{Push: true}, root)
	if err != nil {
		for _, problem := range pub.Problems {
			fmt.Fprintf(out, "  %s\n", problem)
		}
		return fmt.Errorf("run: %w", err)
	}
	fmt.Fprintf(out, "publish  %s\n", pub.Message)

	return nil
}
