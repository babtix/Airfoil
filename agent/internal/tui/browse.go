package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/store"
)

// browsePage inspects what the agent actually produced — the items it ingested
// and the clusters it formed. This is where a bad threshold becomes visible.
type browsePage struct {
	mode     int // 0 = items, 1 = clusters
	items    []model.Item
	clusters []model.Cluster
	cursor   int
	detail   bool
	loaded   bool
	err      error
}

type browseDataMsg struct {
	items    []model.Item
	clusters []model.Cluster
	err      error
}

func newBrowsePage() *browsePage { return &browsePage{} }

func (p *browsePage) Init() tea.Cmd { return nil }

func (p *browsePage) Footer() string {
	if p.detail {
		return styleKey.Render("esc") + styleFooter.Render(" back")
	}
	return styleKey.Render("←→") + styleFooter.Render(" items / clusters  ") +
		styleKey.Render("↑↓") + styleFooter.Render(" move  ") +
		styleKey.Render("enter") + styleFooter.Render(" detail  ") +
		styleKey.Render("R") + styleFooter.Render(" reload")
}

func (p *browsePage) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case browseDataMsg:
		p.items, p.clusters, p.err = msg.items, msg.clusters, msg.err
		p.loaded = true
		return nil, false

	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			p.mode, p.cursor, p.detail = 0, 0, false
			return nil, true

		case "right", "l":
			p.mode, p.cursor, p.detail = 1, 0, false
			return nil, true

		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
			}
			return nil, true

		case "down", "j":
			if p.cursor < p.count()-1 {
				p.cursor++
			}
			return nil, true

		case "enter":
			if p.count() > 0 {
				p.detail = true
			}
			return nil, true

		case "esc":
			if p.detail {
				p.detail = false
				return nil, true
			}
			return nil, false

		case "R":
			p.loaded = false
			return loadBrowseData(m.cfg), true
		}
	}

	if !p.loaded {
		p.loaded = true
		return loadBrowseData(m.cfg), false
	}
	return nil, false
}

func (p *browsePage) count() int {
	if p.mode == 0 {
		return len(p.items)
	}
	return len(p.clusters)
}

func loadBrowseData(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		var out browseDataMsg

		items, err := store.LoadItemsSince(cfg.ItemsDir(), time.Now().UTC(), 7)
		if err != nil {
			out.err = err
		}
		out.items = items

		clusters, err := store.ReadJSONOr(cfg.DataDir+"/clusters.json", []model.Cluster(nil))
		if err == nil {
			out.clusters = clusters
		}
		return out
	}
}

func (p *browsePage) View(m *Model, width, height int) string {
	var b strings.Builder

	modeLabel := func(i int, label string, n int) string {
		text := fmt.Sprintf(" %s (%d) ", label, n)
		if p.mode == i {
			return styleSelected.Render(text)
		}
		return styleDim.Render(text)
	}
	b.WriteString(modeLabel(0, "items", len(p.items)) + "  " +
		modeLabel(1, "clusters", len(p.clusters)))
	b.WriteString("\n\n")

	if p.err != nil {
		b.WriteString(styleError.Render("  " + p.err.Error()))
		return b.String()
	}
	if p.count() == 0 {
		b.WriteString(styleDim.Render("  nothing here yet — run ingest, then cluster"))
		return b.String()
	}

	if p.detail {
		b.WriteString(p.detailView(width))
		return b.String()
	}

	visible := max(height-3, 4)
	start := 0
	if p.cursor >= visible {
		start = p.cursor - visible + 1
	}
	end := min(start+visible, p.count())

	for i := start; i < end; i++ {
		row := p.rowFor(i, width)
		if i == p.cursor {
			b.WriteString(styleCursor.Render("▸ ") + styleSelected.Render(row) + "\n")
		} else {
			b.WriteString("  " + row + "\n")
		}
	}
	return b.String()
}

func (p *browsePage) rowFor(i, width int) string {
	if p.mode == 0 {
		it := p.items[i]
		return pad(truncate(it.SourceID, 17), 18) +
			pad(fmt.Sprintf("t%d", it.SourceTier), 4) +
			pad(relativeTime(it.PublishedAt), 10) +
			truncate(it.Title, max(width-36, 20))
	}

	c := p.clusters[i]
	lead := "(empty)"
	if len(c.Items) > 0 {
		lead = c.Items[0].Title
	}
	return pad(fmt.Sprintf("%d items", len(c.Items)), 10) +
		truncate(lead, max(width-16, 20))
}

func (p *browsePage) detailView(width int) string {
	var b strings.Builder
	w := max(width-4, 30)

	if p.mode == 0 {
		it := p.items[p.cursor]
		b.WriteString(styleHeader.Render(truncate(it.Title, w)) + "\n\n")
		b.WriteString(statLine("source", fmt.Sprintf("%s (tier %d)", it.SourceName, it.SourceTier)))
		b.WriteString(statLine("published", it.PublishedAt.Format(time.RFC3339)))
		b.WriteString(statLine("id", it.ID))
		b.WriteString(statLine("url", truncate(it.URL, w-20)))
		if it.RepoURL != "" {
			b.WriteString(statLine("repo", truncate(it.RepoURL, w-20)))
		}
		if it.PaperURL != "" {
			b.WriteString(statLine("paper", truncate(it.PaperURL, w-20)))
		}
		if m := it.Metrics; m != (model.Metrics{}) {
			b.WriteString(statLine("metrics", fmt.Sprintf(
				"hn %d/%d · reddit %d · hf %d · stars %d",
				m.HNPoints, m.HNComments, m.RedditScore, m.HFUpvotes, m.GitHubStars)))
		}

		b.WriteString("\n" + styleLabel.Render("excerpt") +
			styleDim.Render(fmt.Sprintf("  (%d chars)", len([]rune(it.Excerpt)))) + "\n")
		b.WriteString(wrap(it.Excerpt, w))
		return b.String()
	}

	c := p.clusters[p.cursor]
	b.WriteString(styleHeader.Render(fmt.Sprintf("cluster of %d", len(c.Items))) + "\n")
	b.WriteString(styleDim.Render("  "+c.ID) + "\n\n")

	for _, it := range c.Items {
		b.WriteString(tierStyle(it.SourceTier).Render(pad(fmt.Sprintf("t%d", it.SourceTier), 4)) +
			styleLabel.Render(pad(truncate(it.SourceID, 19), 20)) +
			styleValue.Render(truncate(it.Title, max(w-26, 20))) + "\n")
		b.WriteString(styleDim.Render("      "+truncate(it.URL, max(w-8, 20))) + "\n")
	}
	return b.String()
}

// wrap breaks text into lines of at most width, on word boundaries.
func wrap(s string, width int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return styleDim.Render("  (empty)")
	}

	var b strings.Builder
	line := "  "
	for _, w := range words {
		if len([]rune(line))+len([]rune(w))+1 > width {
			b.WriteString(styleValue.Render(line) + "\n")
			line = "  "
		}
		line += w + " "
	}
	b.WriteString(styleValue.Render(strings.TrimRight(line, " ")))
	return b.String()
}
