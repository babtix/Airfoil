package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/pipeline"
	"github.com/papitsho/airfoil/internal/score"
)

func newRankCmd(a *app) *cobra.Command {
	var (
		explain bool
		top     int
		dry     bool
	)

	cmd := &cobra.Command{
		Use:   "rank",
		Short: "Score clusters and write data/ranked.json",
		Long: "Rank scores every cluster produced by `airfoil cluster` and orders\n" +
			"them best first. Read the top 20 before trusting it — the weights in\n" +
			"config/scoring.json are meant to be tuned against real output.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRank(cmd.Context(), a, cmd.OutOrStdout(), rankOpts{explain, top, dry})
		},
	}
	cmd.Flags().BoolVar(&explain, "explain", false, "show the score breakdown for each story")
	cmd.Flags().IntVar(&top, "top", 20, "how many stories to print")
	cmd.Flags().BoolVar(&dry, "dry", false, "score and print, but write nothing")

	return cmd
}

type rankOpts struct {
	explain bool
	top     int
	dry     bool
}

func runRank(ctx context.Context, a *app, out io.Writer, opts rankOpts) error {
	result, err := a.pipeline().Rank(ctx, pipeline.Options{Since: a.since, Dry: opts.dry})
	if err != nil {
		return err
	}

	printRanking(out, result, opts)
	return nil
}

func printRanking(out io.Writer, result pipeline.RankResult, opts rankOpts) {
	n := min(opts.top, len(result.Results))
	if n <= 0 {
		return
	}

	fmt.Fprintf(out, "\n=== top %d of %d ===\n", n, len(result.Results))
	for i, r := range result.Results[:n] {
		fmt.Fprintf(out, "\n%2d. %3d  %-8s %s\n", i+1, r.Score, r.Tier, truncate(r.Title, 76))
		fmt.Fprintf(out, "        %d source(s) · tier %d · %s",
			r.DistinctSources, r.BestTier, age(r.AgeHours))
		if r.BuilderRelevant {
			fmt.Fprint(out, " · builder")
		}
		if len(r.Tags) > 0 {
			fmt.Fprintf(out, " · %s", strings.Join(r.Tags, " "))
		}
		fmt.Fprintln(out)

		if opts.explain {
			printBreakdown(out, r)
		}
	}

	fmt.Fprintf(out, "\n%d major · %d notable · %d minor · %d builder-relevant\n",
		result.Major, result.Notable, result.Minor, result.Builder)
	fmt.Fprintf(out, "%d would be summarized under the LLM budget\n", result.Budgeted)
}

// printBreakdown shows where a score came from, so a wrong ranking can be
// traced to a weight rather than guessed at.
func printBreakdown(out io.Writer, r score.Result) {
	fmt.Fprintf(out, "        raw %.2f × decay %.3f = %d\n", r.Raw, r.Decay, r.Score)
	if r.HNPoints > 0 || r.RedditScore > 0 {
		fmt.Fprintf(out, "        hn %d · reddit %d\n", r.HNPoints, r.RedditScore)
	}
	if len(r.BuilderSignals) > 0 {
		fmt.Fprintf(out, "        builder: %s\n", strings.Join(r.BuilderSignals, ", "))
	}
	if len(r.HypeSignals) > 0 {
		fmt.Fprintf(out, "        hype:    %s\n", strings.Join(r.HypeSignals, ", "))
	}
}

func age(hours float64) string {
	switch {
	case hours < 1:
		return "just now"
	case hours < 24:
		return fmt.Sprintf("%.0fh old", hours)
	default:
		return fmt.Sprintf("%.1fd old", hours/24)
	}
}
