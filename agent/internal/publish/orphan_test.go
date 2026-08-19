package publish

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/papitsho/airfoil/internal/model"
)

func writeMD(t *testing.T, dir, slug string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, slug+".md"), []byte("---\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateStoryFilesAcceptsMatchingPages(t *testing.T) {
	dir := t.TempDir()
	writeMD(t, dir, "2026-08-17-a")
	writeMD(t, dir, "2026-08-17-b")

	stories := []model.Story{{Slug: "2026-08-17-a"}, {Slug: "2026-08-17-b"}}

	if got := ValidateStoryFiles(stories, dir); len(got) != 0 {
		t.Errorf("problems = %v, want none", got)
	}
}

func TestValidateStoryFilesFlagsOrphan(t *testing.T) {
	// The exact failure this exists for: stories.json was rebuilt, the title
	// changed, a new slug was written, and the old page stayed behind.
	dir := t.TempDir()
	writeMD(t, dir, "2026-08-17-amazon-destroys-rare-books-to-train-ai-language-models")
	writeMD(t, dir, "2026-08-17-amazon-destroys-rare-books-to-train-its-ai-models")

	stories := []model.Story{{Slug: "2026-08-17-amazon-destroys-rare-books-to-train-its-ai-models"}}

	got := ValidateStoryFiles(stories, dir)
	if len(got) != 1 {
		t.Fatalf("problems = %v, want exactly the stale page", got)
	}
	if got[0].Slug != "2026-08-17-amazon-destroys-rare-books-to-train-ai-language-models" {
		t.Errorf("flagged %q, want the stale slug", got[0].Slug)
	}
}

func TestValidateStoryFilesCountsIndexOnlyStories(t *testing.T) {
	// A story without prose has no markdown, but it still claims its slug —
	// a page bearing that slug is legitimate, not an orphan.
	dir := t.TempDir()
	writeMD(t, dir, "2026-08-17-quiet")

	stories := []model.Story{{Slug: "2026-08-17-quiet", Summary: ""}}

	if got := ValidateStoryFiles(stories, dir); len(got) != 0 {
		t.Errorf("problems = %v, want none", got)
	}
}

func TestValidateStoryFilesIgnoresNonMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "drafts"), 0o755); err != nil {
		t.Fatal(err)
	}

	if got := ValidateStoryFiles(nil, dir); len(got) != 0 {
		t.Errorf("problems = %v, want none — only .md files are pages", got)
	}
}

func TestValidateStoryFilesMissingDirIsNotAFailure(t *testing.T) {
	// A first run has written nothing yet.
	if got := ValidateStoryFiles(nil, filepath.Join(t.TempDir(), "nope")); len(got) != 0 {
		t.Errorf("problems = %v, want none", got)
	}
}
