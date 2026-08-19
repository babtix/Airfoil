package main

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/pipeline"
)

func newWriteCmd(a *app) *cobra.Command {
	var dry bool

	cmd := &cobra.Command{
		Use:   "write",
		Short: "Summarize top-ranked clusters and publish stories",
		Long: "Write sends the highest-scoring clusters through the LLM chain,\n" +
			"validates every summary against the R1/R2 rules in Go, and writes\n" +
			"the markdown files, data/index.json and data/stories.json.\n\n" +
			"Existing stories are never rewritten — a story that gains coverage\n" +
			"keeps its title, summary and URL, and only its score and sources move.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWrite(cmd.Context(), a, cmd.OutOrStdout(), dry)
		},
	}
	cmd.Flags().BoolVar(&dry, "dry", false, "summarize and report, but write no files")

	return cmd
}

func runWrite(ctx context.Context, a *app, out io.Writer, dry bool) error {
	result, err := a.pipeline().Write(ctx, pipeline.Options{Since: a.since, Dry: dry})
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\n%d summarized · %d index-only · %d skipped\n",
		result.Summarized, result.IndexOnly, result.Skipped)
	fmt.Fprintf(out, "%d created · %d updated · %d unchanged · %d indexed\n",
		result.Stats.Created, result.Stats.Updated,
		result.Stats.Unchanged, result.Stats.Indexed)

	if n := len(result.Stats.Files); n > 0 {
		fmt.Fprintf(out, "%d markdown files under %s\n", n, a.cfg.StoriesDir)
	}
	return nil
}
