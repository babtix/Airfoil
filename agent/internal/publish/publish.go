package publish

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/store"
)

// Publisher validates and commits a run's output.
type Publisher struct {
	cfg  *config.Config
	log  *slog.Logger
	repo string // repository root; git commands run here
}

func New(cfg *config.Config, log *slog.Logger, repoRoot string) *Publisher {
	return &Publisher{cfg: cfg, log: log, repo: repoRoot}
}

// Options control how far publish goes.
type Options struct {
	// Dry validates and reports without staging, committing, or pushing.
	Dry bool
	// Push sends the commit to the remote. It is off by default: publishing
	// to a remote is not reversible from here, so it must be asked for.
	Push bool
	// SkipPrune keeps retention data intact, for debugging.
	SkipPrune bool
}

// Result reports what publish did.
type Result struct {
	Stories        int
	Problems       []Problem
	DroppedStories int
	DroppedURLs    int
	Committed      bool
	Pushed         bool
	Commit         string
	Message        string
}

// ErrValidation means the output failed its checks and nothing was committed.
var ErrValidation = errors.New("publish: validation failed")

// Run validates, prunes, commits, and optionally pushes.
//
// R9: validation happens first and aborts before anything is staged. A broken
// run leaves the working tree untouched rather than deploying a broken site.
func (p *Publisher) Run(ctx context.Context, opts Options, now time.Time) (Result, error) {
	var res Result

	stories, err := store.ReadJSONOr(p.storiesPath(), []model.Story(nil))
	if err != nil {
		return res, fmt.Errorf("publish: %w", err)
	}
	idx, err := store.ReadJSONOr(p.indexPath(), model.Index{})
	if err != nil {
		return res, fmt.Errorf("publish: %w", err)
	}
	res.Stories = len(stories)

	if len(stories) == 0 {
		return res, fmt.Errorf("publish: no stories in %s — run write first", p.storiesPath())
	}

	res.Problems = Validate(stories, idx, p.cfg.Scoring)
	// A page nothing links to is as broken as a malformed one, and only a
	// disk check can see it.
	res.Problems = append(res.Problems, ValidateStoryFiles(stories, p.cfg.StoriesDir)...)
	if len(res.Problems) > 0 {
		return res, fmt.Errorf("%w: %d problems", ErrValidation, len(res.Problems))
	}

	if !opts.SkipPrune {
		state, err := store.LoadState(p.cfg.StatePath())
		if err != nil {
			return res, fmt.Errorf("publish: %w", err)
		}

		kept, droppedStories, droppedURLs := Prune(stories, state, p.cfg.Scoring, now)
		res.DroppedStories, res.DroppedURLs = droppedStories, droppedURLs

		if !opts.Dry && (droppedStories > 0 || droppedURLs > 0) {
			if err := store.WriteJSON(p.storiesPath(), kept); err != nil {
				return res, fmt.Errorf("publish: %w", err)
			}
			if err := store.SaveState(p.cfg.StatePath(), state); err != nil {
				return res, fmt.Errorf("publish: %w", err)
			}
			stories = kept
		}
	}

	res.Message = fmt.Sprintf("content: %s — %d stories",
		now.UTC().Format(time.DateOnly), len(stories))

	if opts.Dry {
		p.log.Info("dry run: validated, nothing committed",
			"stories", len(stories), "would_commit", res.Message)
		return res, nil
	}

	changed, err := p.stage(ctx)
	if err != nil {
		return res, err
	}
	if !changed {
		p.log.Info("nothing to commit — output is unchanged since the last publish")
		return res, nil
	}

	commit, err := p.commit(ctx, res.Message)
	if err != nil {
		return res, err
	}
	res.Committed = true
	res.Commit = commit
	p.log.Info("committed", "sha", commit, "message", res.Message)

	if opts.Push {
		if err := p.git(ctx, "push"); err != nil {
			return res, fmt.Errorf("publish: push: %w", err)
		}
		res.Pushed = true
		p.log.Info("pushed")
	}

	return res, nil
}

// stage adds the generated paths and reports whether anything actually differs.
func (p *Publisher) stage(ctx context.Context) (bool, error) {
	paths := []string{p.cfg.DataDir, p.cfg.StoriesDir}
	for _, path := range paths {
		if err := p.git(ctx, "add", path); err != nil {
			return false, fmt.Errorf("publish: git add %s: %w", path, err)
		}
	}

	// --cached compares the index against HEAD: exit 1 means there is
	// something staged to commit.
	err := p.git(ctx, "diff", "--cached", "--quiet")
	if err == nil {
		return false, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return true, nil
	}
	return false, fmt.Errorf("publish: git diff: %w", err)
}

func (p *Publisher) commit(ctx context.Context, message string) (string, error) {
	if err := p.git(ctx, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("publish: git commit: %w", err)
	}
	out, err := p.gitOutput(ctx, "rev-parse", "--short", "HEAD")
	if err != nil {
		return "", nil // the commit succeeded; the sha is only for reporting
	}
	return strings.TrimSpace(out), nil
}

func (p *Publisher) git(ctx context.Context, args ...string) error {
	_, err := p.gitOutput(ctx, args...)
	return err
}

func (p *Publisher) gitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = p.repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (p *Publisher) storiesPath() string { return filepath.Join(p.cfg.DataDir, "stories.json") }
func (p *Publisher) indexPath() string   { return filepath.Join(p.cfg.DataDir, "index.json") }
