package main

import (
	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/tui"
)

func newTUICmd(a *app, verbose *bool) *cobra.Command {
	return &cobra.Command{
		Use:     "tui",
		Aliases: []string{"ui"},
		Short:   "Interactive interface: run stages, edit config, check health, browse output",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return tui.Run(a.cfg, *verbose)
		},
	}
}
