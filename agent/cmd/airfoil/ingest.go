package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/pipeline"
)

func newIngestCmd(a *app) *cobra.Command {
	var dry bool

	cmd := &cobra.Command{
		Use:   "ingest",
		Short: "Fetch every enabled source into data/items/",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runIngest(cmd.Context(), a, dry)
		},
	}
	cmd.Flags().BoolVar(&dry, "dry", false, "fetch and report, but write nothing")

	return cmd
}

func runIngest(ctx context.Context, a *app, dry bool) error {
	_, err := a.pipeline().Ingest(ctx, pipeline.Options{
		Since: a.since,
		Dry:   dry,
	})
	return err
}
