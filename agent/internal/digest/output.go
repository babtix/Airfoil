package digest

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/papitsho/airfoil/internal/store"
)

// write renders every output for the day into data/digest/.
//
// Nothing is sent. These files exist to be reviewed and posted by a person.
func (g *Generator) write(day time.Time, res Result) ([]string, error) {
	dir := g.Dir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("digest: %w", err)
	}

	stamp := day.UTC().Format(time.DateOnly)
	var files []string

	outputs := []struct {
		name string
		data []byte
	}{
		{stamp + "-newsletter.html", []byte(g.newsletterHTML(res))},
		{stamp + "-newsletter.txt", []byte(g.newsletterText(res))},
		{stamp + "-linkedin.md", []byte(g.linkedInMarkdown(res))},
	}

	for _, o := range outputs {
		path := filepath.Join(dir, o.name)
		if err := store.WriteFile(path, o.data); err != nil {
			return files, fmt.Errorf("digest: %w", err)
		}
		files = append(files, path)
	}

	// The thread stays JSON: it is a payload for a posting tool, not prose.
	threadPath := filepath.Join(dir, stamp+"-x.json")
	if err := store.WriteJSON(threadPath, res.Thread); err != nil {
		return files, fmt.Errorf("digest: %w", err)
	}
	files = append(files, threadPath)

	return files, nil
}

func (g *Generator) newsletterText(res Result) string {
	var b strings.Builder

	fmt.Fprintf(&b, "AIRFOIL — %s\n", res.Date.UTC().Format("Monday, 2 January 2006"))
	if res.Intro.Theme != "" {
		fmt.Fprintf(&b, "%s\n", res.Intro.Theme)
	}
	b.WriteString(strings.Repeat("=", 60) + "\n\n")

	if res.Intro.Intro != "" {
		b.WriteString(res.Intro.Intro + "\n\n")
	}

	for i, s := range res.Stories {
		fmt.Fprintf(&b, "%d. [%d] %s\n\n", i+1, s.Score, s.Title)
		fmt.Fprintf(&b, "%s\n\n", s.Summary)
		for _, t := range s.Takeaways {
			fmt.Fprintf(&b, "   - %s\n", t)
		}
		if len(s.Takeaways) > 0 {
			b.WriteString("\n")
		}
		for _, src := range s.Sources {
			fmt.Fprintf(&b, "   %s: %s\n", src.Name, src.URL)
		}
		b.WriteString("\n" + strings.Repeat("-", 60) + "\n\n")
	}

	if g.cfg.SiteURL != "" {
		fmt.Fprintf(&b, "Read more: %s\n", g.cfg.SiteURL)
	}
	return b.String()
}

func (g *Generator) newsletterHTML(res Result) string {
	var b strings.Builder

	b.WriteString(`<!doctype html>` + "\n")
	b.WriteString(`<html lang="en"><head><meta charset="utf-8">` + "\n")
	fmt.Fprintf(&b, "<title>Airfoil — %s</title>\n", esc(res.Date.UTC().Format(time.DateOnly)))
	// Inline styles only: email clients strip <style> blocks and never load
	// external CSS.
	b.WriteString(`</head><body style="font-family:-apple-system,Segoe UI,sans-serif;` +
		`max-width:640px;margin:0 auto;padding:24px;color:#111;line-height:1.5">` + "\n")

	fmt.Fprintf(&b, `<h1 style="font-size:20px;margin:0 0 4px">Airfoil</h1>`+"\n")
	fmt.Fprintf(&b, `<p style="color:#666;margin:0 0 24px;font-size:14px">%s</p>`+"\n",
		esc(res.Date.UTC().Format("Monday, 2 January 2006")))

	if res.Intro.Intro != "" {
		fmt.Fprintf(&b, `<p style="font-size:16px;margin:0 0 32px">%s</p>`+"\n",
			esc(res.Intro.Intro))
	}

	for i, s := range res.Stories {
		b.WriteString(`<div style="margin:0 0 32px;padding:0 0 24px;border-bottom:1px solid #eee">` + "\n")
		fmt.Fprintf(&b,
			`<h2 style="font-size:17px;margin:0 0 8px">%d. %s `+
				`<span style="color:#999;font-weight:normal;font-size:14px">%d</span></h2>`+"\n",
			i+1, esc(s.Title), s.Score)
		fmt.Fprintf(&b, `<p style="margin:0 0 12px">%s</p>`+"\n", esc(s.Summary))

		if len(s.Takeaways) > 0 {
			b.WriteString(`<ul style="margin:0 0 12px;padding-left:20px;color:#333">` + "\n")
			for _, t := range s.Takeaways {
				fmt.Fprintf(&b, `<li>%s</li>`+"\n", esc(t))
			}
			b.WriteString("</ul>\n")
		}

		b.WriteString(`<p style="margin:0;font-size:13px;color:#666">` + "\n")
		for j, src := range s.Sources {
			if j > 0 {
				b.WriteString(" · ")
			}
			fmt.Fprintf(&b, `<a href="%s" style="color:#0066cc">%s</a>`,
				esc(src.URL), esc(src.Name))
		}
		b.WriteString("\n</p>\n</div>\n")
	}

	if g.cfg.SiteURL != "" {
		fmt.Fprintf(&b, `<p style="font-size:14px"><a href="%s">Read more on Airfoil</a></p>`+"\n",
			esc(g.cfg.SiteURL))
	}
	b.WriteString("</body></html>\n")

	return b.String()
}

func (g *Generator) linkedInMarkdown(res Result) string {
	var b strings.Builder

	b.WriteString("# LinkedIn draft — " + res.Date.UTC().Format(time.DateOnly) + "\n\n")
	b.WriteString("> Draft only. Edit the [YOUR TAKE] section and post manually.\n\n")
	b.WriteString("---\n\n")
	b.WriteString(res.LinkedIn.Body + "\n\n")

	if len(res.LinkedIn.Hashtags) > 0 {
		for i, tag := range res.LinkedIn.Hashtags {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString("#" + strings.TrimPrefix(tag, "#"))
		}
		b.WriteString("\n\n")
	}
	if res.LinkedIn.StoryURL != "" {
		b.WriteString(res.LinkedIn.StoryURL + "\n")
	}
	return b.String()
}

// esc escapes text for HTML. Story text comes from an LLM over third-party
// feeds, so it is never trusted into markup unescaped.
func esc(s string) string { return html.EscapeString(s) }
