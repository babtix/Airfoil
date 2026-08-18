package write

import (
	"strings"
	"testing"
	"time"

	"github.com/papitsho/airfoil/internal/model"
)

func TestMarkdownFrontmatter(t *testing.T) {
	s := model.Story{
		ID:              "a3f9c21b8e04d7f2",
		Slug:            "2026-08-17-claude-opus-5",
		Title:           "Anthropic ships Claude Opus 5",
		Summary:         "An original summary.",
		Score:           87,
		Tier:            model.TierMajor,
		Tags:            []string{"models", "agents"},
		BuilderRelevant: true,
		Date:            time.Date(2026, 8, 17, 9, 12, 0, 0, time.UTC),
		ClusterSize:     14,
		Sources: []model.StorySource{
			{Name: "Anthropic Blog", URL: "https://anthropic.test/post", Tier: 1, Type: "lab"},
			{Name: "Hacker News", URL: "https://hn.test/item", Tier: 5, Type: "community",
				Metrics: &model.SourceMetrics{Points: 1240, Comments: 380}},
		},
		Takeaways: []string{"Context jumps to 500K", "Pricing unchanged"},
	}

	md := Markdown(s)

	if !strings.HasPrefix(md, "---\n") {
		t.Fatal("does not open with a frontmatter fence")
	}
	if strings.Count(md, "---\n") < 2 {
		t.Fatal("frontmatter is not closed")
	}

	for _, want := range []string{
		`id: "a3f9c21b8e04d7f2"`,
		`title: "Anthropic ships Claude Opus 5"`,
		"score: 87",
		`tier: "major"`,
		`tags: ["models", "agents"]`,
		"builder_relevant: true",
		"date: 2026-08-17T09:12:00Z",
		"cluster_size: 14",
		`- name: "Anthropic Blog"`,
		`    tier: 1`,
		`    type: "lab"`,
		"metrics: { points: 1240, comments: 380 }",
		`  - "Context jumps to 500K"`,
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q\n---\n%s", want, md)
		}
	}
}

func TestMarkdownQuotesDangerousScalars(t *testing.T) {
	// Unquoted, each of these would parse as something other than a string.
	tests := []struct {
		name  string
		title string
	}{
		{"yes parses as bool", "Yes"},
		{"no parses as bool", "No"},
		{"leading dash parses as list", "- not a list item"},
		{"colon splits the key", "GPT-5: what changed"},
		{"leading hash is a comment", "#1 model"},
		{"numeric parses as int", "2026"},
		{"embedded quote", `He said "hello"`},
		{"backslash", `C:\path\to`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			md := Markdown(model.Story{Title: tt.title, Summary: "s", Tier: model.TierMinor})

			line := findLine(md, "title: ")
			if line == "" {
				t.Fatalf("no title line in\n%s", md)
			}
			value := strings.TrimPrefix(line, "title: ")
			if !strings.HasPrefix(value, `"`) || !strings.HasSuffix(value, `"`) {
				t.Errorf("title not quoted: %s", line)
			}
		})
	}
}

func TestMarkdownFlattensNewlines(t *testing.T) {
	// A newline inside a double-quoted YAML scalar would break the document.
	md := Markdown(model.Story{
		Title:   "A title",
		Summary: "First line.\nSecond line.\r\nThird.",
		Tier:    model.TierMinor,
	})

	line := findLine(md, "summary: ")
	if line == "" {
		t.Fatalf("no summary line in\n%s", md)
	}
	if strings.Count(line, `"`) != 2 {
		t.Errorf("summary scalar is not a single quoted span: %s", line)
	}
	if !strings.Contains(line, "First line. Second line.") {
		t.Errorf("newlines were not flattened: %s", line)
	}
}

func TestMarkdownOmitsEmptyMetricsAndTakeaways(t *testing.T) {
	md := Markdown(model.Story{
		Title: "T", Summary: "S", Tier: model.TierMinor,
		Sources: []model.StorySource{{Name: "Blog", URL: "https://a.test", Tier: 1, Type: "lab"}},
	})

	if strings.Contains(md, "metrics:") {
		t.Errorf("emitted an empty metrics block\n%s", md)
	}
	if strings.Contains(md, "takeaways:") {
		t.Errorf("emitted an empty takeaways block\n%s", md)
	}
}

func TestMarkdownBody(t *testing.T) {
	md := Markdown(model.Story{
		Title: "T", Summary: "S", Tier: model.TierMinor,
		Body: []string{"First paragraph.", "Second paragraph."},
	})

	if !strings.Contains(md, "First paragraph.\n\nSecond paragraph.") {
		t.Errorf("body paragraphs not joined\n%s", md)
	}
}

func findLine(s, prefix string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	return ""
}
