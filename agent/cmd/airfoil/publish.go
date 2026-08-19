package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/papitsho/airfoil/internal/publish"
)

func newPublishCmd(a *app) *cobra.Command {
	var opts publish.Options

	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Validate the generated output and commit it",
		Long: "Publish validates every story, prunes past the retention window,\n" +
			"then stages and commits data/ and the story markdown.\n\n" +
			"Validation runs first and aborts before anything is staged, so a\n" +
			"broken run cannot reach the site. Pushing is opt-in via --push.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPublish(cmd.Context(), a, cmd.OutOrStdout(), opts)
		},
	}
	cmd.Flags().BoolVar(&opts.Dry, "dry", false, "validate and report, but do not commit")
	cmd.Flags().BoolVar(&opts.Push, "push", false, "push the commit to the remote")
	cmd.Flags().BoolVar(&opts.SkipPrune, "skip-prune", false, "keep data past the retention window")

	return cmd
}

func runPublish(ctx context.Context, a *app, out io.Writer, opts publish.Options) error {
	root, err := repoRoot(a)
	if err != nil {
		return err
	}

	res, err := a.pipeline().Publish(ctx, opts, root)

	// Validation problems are the useful output, so print them before
	// returning the error.
	if len(res.Problems) > 0 {
		fmt.Fprintf(out, "\n%d validation problems — nothing was committed:\n", len(res.Problems))
		for _, p := range res.Problems {
			fmt.Fprintf(out, "  %s\n", p)
		}
	}
	if err != nil {
		if errors.Is(err, publish.ErrValidation) {
			return errors.New("publish aborted: fix the problems above and re-run")
		}
		return err
	}

	fmt.Fprintf(out, "\n%d stories validated\n", res.Stories)
	if res.DroppedStories > 0 || res.DroppedURLs > 0 {
		fmt.Fprintf(out, "pruned %d stories and %d seen URLs past retention\n",
			res.DroppedStories, res.DroppedURLs)
	}
	switch {
	case opts.Dry:
		fmt.Fprintf(out, "dry run — would commit %q\n", res.Message)
	case res.Committed:
		fmt.Fprintf(out, "committed %s — %s\n", res.Commit, res.Message)
		if !res.Pushed {
			fmt.Fprintln(out, "not pushed. Re-run with --push, or push manually.")
		}
	default:
		fmt.Fprintln(out, "nothing to commit — output is unchanged")
	}
	return nil
}

// repoRoot finds the directory git commands should run in. The data and story
// directories are both inside the repo, so their common ancestor is a safe
// starting point regardless of where the binary was invoked from.
func repoRoot(a *app) (string, error) {
	start, err := filepath.Abs(a.cfg.DataDir)
	if err != nil {
		return "", fmt.Errorf("publish: %w", err)
	}
	for dir := start; ; {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info != nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("publish: no git repository found above %s", start)
		}
		dir = parent
	}
}
