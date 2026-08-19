package write

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/store"
)

// writeMarkdown renders one file per summarized story.
//
// Stories without a summary are index entries only (BUILD_SPEC §5.5) and get
// no file: a markdown page with an empty body is worse than no page.
func (w *Writer) writeMarkdown(stories []model.Story) ([]string, error) {
	dir := w.cfg.StoriesDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("write: stories dir: %w", err)
	}

	var files []string
	for _, s := range stories {
		if strings.TrimSpace(s.Summary) == "" {
			continue
		}
		path := filepath.Join(dir, s.Slug+".md")
		if err := store.WriteFile(path, []byte(Markdown(s))); err != nil {
			return files, fmt.Errorf("write %s: %w", s.Slug, err)
		}
		files = append(files, path)
	}
	return files, nil
}

// Markdown renders a story as frontmatter plus body, in the shape defined by
// BUILD_SPEC §4.
//
// The YAML is emitted by hand rather than through a library because the schema
// is fixed and small, and adding a YAML dependency for six fields would breach
// the AGENTS.md dependency allowlist.
func Markdown(s model.Story) string {
	var b strings.Builder

	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", yamlString(s.ID))
	fmt.Fprintf(&b, "title: %s\n", yamlString(s.Title))
	fmt.Fprintf(&b, "summary: %s\n", yamlString(s.Summary))
	fmt.Fprintf(&b, "score: %d\n", s.Score)
	fmt.Fprintf(&b, "tier: %s\n", yamlString(s.Tier))
	fmt.Fprintf(&b, "tags: [%s]\n", yamlList(s.Tags))
	fmt.Fprintf(&b, "builder_relevant: %t\n", s.BuilderRelevant)
	fmt.Fprintf(&b, "date: %s\n", s.Date.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "cluster_size: %d\n", s.ClusterSize)

	b.WriteString("sources:\n")
	for _, src := range s.Sources {
		fmt.Fprintf(&b, "  - name: %s\n", yamlString(src.Name))
		fmt.Fprintf(&b, "    url: %s\n", yamlString(src.URL))
		fmt.Fprintf(&b, "    tier: %d\n", src.Tier)
		fmt.Fprintf(&b, "    type: %s\n", yamlString(src.Type))
		if m := src.Metrics; m != nil {
			if inline := yamlMetrics(*m); inline != "" {
				fmt.Fprintf(&b, "    metrics: { %s }\n", inline)
			}
		}
	}

	if len(s.Takeaways) > 0 {
		b.WriteString("takeaways:\n")
		for _, t := range s.Takeaways {
			fmt.Fprintf(&b, "  - %s\n", yamlString(t))
		}
	}

	b.WriteString("---\n")

	if len(s.Body) > 0 {
		b.WriteString("\n")
		b.WriteString(strings.Join(s.Body, "\n\n"))
		b.WriteString("\n")
	}

	return b.String()
}

// yamlString always quotes and escapes. Quoting unconditionally avoids the
// long tail of YAML scalar surprises — a title of "No" parsing as false, one
// starting with "-" parsing as a list, or a colon splitting the key.
func yamlString(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", " ",
		"\r", " ",
		"\t", " ",
	)
	return `"` + r.Replace(s) + `"`
}

func yamlList(items []string) string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		out = append(out, yamlString(s))
	}
	return strings.Join(out, ", ")
}

func yamlMetrics(m model.SourceMetrics) string {
	var parts []string
	add := func(k string, v int) {
		if v > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", k, v))
		}
	}
	add("points", m.Points)
	add("comments", m.Comments)
	add("score", m.Score)
	add("upvotes", m.Upvotes)
	add("stars", m.Stars)
	return strings.Join(parts, ", ")
}
