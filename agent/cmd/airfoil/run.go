package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/pipeline"
	"github.com/papitsho/airfoil/internal/publish"
)

func newRunCmd(a *app) *cobra.Command {
	var (
		dry  bool
		push bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the whole pipeline: ingest, cluster, rank, write",
		Long: "Run chains every stage in order.\n\n" +
			"--dry does everything except write files, call the LLM chain, or\n" +
			"commit. Publishing is opt-in: without --push nothing leaves the\n" +
			"machine, and the commit step only runs when --push is given.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAll(cmd.Context(), a, cmd.OutOrStdout(), dry, push)
		},
	}
	cmd.Flags().BoolVar(&dry, "dry", false, "run every stage without writing, summarizing, or committing")
	cmd.Flags().BoolVar(&push, "push", false, "commit and push the result when the run succeeds")

	return cmd
}

func runAll(ctx context.Context, a *app, out io.Writer, dry, push bool) error {
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
