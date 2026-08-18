package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/digest"
	"github.com/papitsho/airfoil/internal/llm"
)

func newDigestCmd(a *app) *cobra.Command {
	var (
		dry string
		day string
	)

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Generate the daily newsletter, X thread, and LinkedIn draft",
		Long: "Digest writes the day's outputs to data/digest/ for review.\n\n" +
			"Nothing is sent anywhere. The LinkedIn draft deliberately leaves a\n" +
			"[YOUR TAKE] placeholder — the commentary stays human.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDigest(cmd.Context(), a, cmd.OutOrStdout(), day, dry == "true")
		},
	}
	cmd.Flags().StringVar(&day, "day", "", "day to summarize, YYYY-MM-DD (default today)")
	cmd.Flags().StringVar(&dry, "dry", "false", "generate and report, but write no files")
	cmd.Flag("dry").NoOptDefVal = "true"

	return cmd
}

func runDigest(ctx context.Context, a *app, out io.Writer, day string, dry bool) error {
	when := time.Now().UTC()
	if day != "" {
		parsed, err := time.Parse(time.DateOnly, day)
		if err != nil {
			return fmt.Errorf("--day %q: want YYYY-MM-DD: %w", day, err)
		}
		when = parsed
	}

	chain := llm.New(a.cfg, a.log)
	if len(chain.Providers()) == 0 {
		return fmt.Errorf("digest: no LLM provider configured")
	}

	res, err := digest.New(chain, a.cfg, a.log).Run(ctx, when, dry)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "\n%s — %d stories\n", res.Date.Format(time.DateOnly), len(res.Stories))
	if res.Intro.Theme != "" {
		fmt.Fprintf(out, "theme: %s\n", res.Intro.Theme)
	}
	if res.Intro.Intro != "" {
		fmt.Fprintf(out, "\n%s\n", res.Intro.Intro)
	}
	if n := len(res.Thread.Posts); n > 0 {
		fmt.Fprintf(out, "\nX thread: %d posts\n", n)
	}

	for _, f := range res.Failures {
		fmt.Fprintf(out, "\nnot generated — %s\n", f)
	}
	for _, f := range res.Files {
		fmt.Fprintf(out, "wrote %s\n", f)
	}
	if dry {
		fmt.Fprintln(out, "\ndry run: nothing written")
	}
	return nil
}
