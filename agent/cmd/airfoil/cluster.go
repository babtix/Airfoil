package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/pipeline"
)

func newClusterCmd(a *app) *cobra.Command {
	var debug bool

	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Embed recent items and group them into stories",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCluster(cmd.Context(), a, cmd.OutOrStdout(), debug)
		},
	}
	cmd.Flags().BoolVar(&debug, "debug", false,
		"print similarity pairs inside the tuning range from scoring.json")

	return cmd
}

func runCluster(ctx context.Context, a *app, out io.Writer, debug bool) error {
	result, err := a.pipeline().Cluster(ctx, pipeline.Options{
		Since: a.since,
		Debug: debug,
	})
	if err != nil {
		return err
	}
	if debug {
		printClusterDebug(out, result, a.cfg.Scoring.Clustering.DebugRange)
	}
	return nil
}

// printClusterDebug shows the grouped clusters and the borderline pairs, which
// is what the threshold should be tuned against — evidence, not vibes.
func printClusterDebug(out io.Writer, result pipeline.ClusterResult, r [2]float64) {
	fmt.Fprintf(out, "\n=== multi-item clusters ===\n")
	for _, c := range result.Clusters {
		if len(c.Items) < 2 {
			continue
		}
		fmt.Fprintf(out, "\n[%d items] %s\n", len(c.Items), c.Items[0].Title)
		for _, item := range c.Items {
			fmt.Fprintf(out, "    t%d  %-22s  %s\n", item.SourceTier, item.SourceID, truncate(item.Title, 72))
		}
	}

	fmt.Fprintf(out, "\n=== borderline pairs (%.2f–%.2f) ===\n", r[0], r[1])
	if len(result.Borderline) == 0 {
		fmt.Fprintln(out, "none — no pair landed in the tuning range")
		return
	}
	for _, p := range result.Borderline {
		fmt.Fprintf(out, "\n%.4f\n  A: %s\n  B: %s\n", p.Similarity,
			truncate(p.A.Title, 88), truncate(p.B.Title, 88))
	}
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
