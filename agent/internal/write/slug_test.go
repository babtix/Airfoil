package write

import (
	"strings"
	"testing"
	"time"
)

func TestSlug(t *testing.T) {
	date := time.Date(2026, 8, 17, 9, 12, 0, 0, time.UTC)

	tests := []struct {
		name  string
		title string
		want  string
	}{
		{"simple", "Claude Opus 5 ships", "2026-08-17-claude-opus-5-ships"},
		{"punctuation dropped", "OpenAI's new API: it's here!", "2026-08-17-openai-s-new-api-it-s-here"},
		{"collapses separators", "A  --  B", "2026-08-17-a-b"},
		{"trims edges", "  --Hello--  ", "2026-08-17-hello"},
		{"digits kept", "GPT 5 vs Llama 4", "2026-08-17-gpt-5-vs-llama-4"},
		{"non-ascii dropped", "Café über AI", "2026-08-17-caf-ber-ai"},
		{"empty title still yields a name", "!!!", "2026-08-17-story"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Slug(date, tt.title); got != tt.want {
				t.Errorf("Slug(%q) = %q, want %q", tt.title, got, tt.want)
			}
		})
	}
}

func TestSlugTruncatesOnWordBoundary(t *testing.T) {
	date := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	long := "Anthropic ships Claude Opus 5 with a five hundred thousand token " +
		"context window and much better tool calling for agents"

	got := Slug(date, long)
	stem := strings.TrimPrefix(got, "2026-08-17-")

	if len(stem) > slugMaxChars {
		t.Errorf("stem is %d chars, want <= %d: %q", len(stem), slugMaxChars, stem)
	}
	if strings.HasSuffix(stem, "-") {
		t.Errorf("slug ends with a separator: %q", stem)
	}
	// Truncating mid-word would leave a fragment; the last segment should be
	// a whole word from the title.
	last := stem[strings.LastIndexByte(stem, '-')+1:]
	if !strings.Contains(strings.ToLower(long), last) {
		t.Errorf("last segment %q is not a whole word from the title", last)
	}
}

func TestSlugIsStableForSameInput(t *testing.T) {
	date := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	first := Slug(date, "A story about models")
	for i := 0; i < 5; i++ {
		if got := Slug(date, "A story about models"); got != first {
			t.Fatalf("slug changed between calls: %q then %q", first, got)
		}
	}
}

func TestDedupeSlug(t *testing.T) {
	taken := map[string]bool{
		"2026-08-17-story":   true,
		"2026-08-17-story-2": true,
	}
	in := func(s string) bool { return taken[s] }

	if got := DedupeSlug("2026-08-17-fresh", in); got != "2026-08-17-fresh" {
		t.Errorf("unused slug = %q, want it unchanged", got)
	}
	if got := DedupeSlug("2026-08-17-story", in); got != "2026-08-17-story-3" {
		t.Errorf("collision = %q, want 2026-08-17-story-3", got)
	}
}
