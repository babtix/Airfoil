package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/papitsho/airfoil/internal/digest"
	"github.com/papitsho/airfoil/internal/llm"
)

// Digest generates the newsletter, X thread, and LinkedIn draft for a day.
//
// Nothing is sent anywhere: the files land in data/digest/ for review. The
// LinkedIn draft keeps its [YOUR TAKE] placeholder by design.
func (p *Pipeline) Digest(ctx context.Context, when time.Time, dry bool) (digest.Result, error) {
	chain := llm.New(p.cfg, p.log)
	if len(chain.Providers()) == 0 {
		return digest.Result{}, fmt.Errorf("digest: no LLM provider configured")
	}
	return digest.New(chain, p.cfg, p.log).Run(ctx, when, dry)
}

// RepoRoot walks up from the data directory to the enclosing git repository.
//
// Publishing commits into that repository, so locating it explicitly beats
// assuming the process happens to be running from the right directory.
func (p *Pipeline) RepoRoot() (string, error) {
	start, err := filepath.Abs(p.cfg.DataDir)
	if err != nil {
		return "", fmt.Errorf("pipeline: %w", err)
	}
	for dir := start; ; {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info != nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("pipeline: no git repository found above %s", start)
		}
		dir = parent
	}
}

// HasLLM reports whether any provider is configured, so callers can explain a
// missing key before spending a stage on it.
func (p *Pipeline) HasLLM() bool {
	return len(llm.New(p.cfg, p.log).Providers()) > 0
}
