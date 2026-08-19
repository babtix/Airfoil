package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/score"
	"github.com/papitsho/airfoil/internal/store"
)

// browse modes, in pipeline order: what came in, how it grouped, how it scored,
// and what was published.
const (
	modeItems = iota
	modeClusters
	modeRanked
	modeStories
	modeCount
)

var modeLabels = [modeCount]string{"items", "clusters", "ranked", "stories"}

// browsePage inspects what the agent actually produced at every stage. This is
// where a bad threshold or a bad weight becomes visible.
type browsePage struct {
	mode     int
	items    []model.Item
	clusters []model.Cluster
	ranked   []score.Result
	stories  []model.Story
	cursor   int
	detail   bool
	loaded   bool
	err      error
}

type browseDataMsg struct {
	items    []model.Item
	clusters []model.Cluster
	ranked   []score.Result
	stories  []model.Story
	err      error
}

func newBrowsePage() *browsePage { return &browsePage{} }

func (p *browsePage) Init() tea.Cmd { return nil }

func (p *browsePage) Footer() string {
	if p.detail {
		return styleKey.Render("esc") + styleFooter.Render(" back")
	}
	return styleKey.Render("←→") + styleFooter.Render(" stage  ") +
		styleKey.Render("↑↓") + styleFooter.Render(" move  ") +
		styleKey.Render("enter") + styleFooter.Render(" detail  ") +
		styleKey.Render("R") + styleFooter.Render(" reload")
}

func (p *browsePage) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case browseDataMsg:
		p.items, p.clusters = msg.items, msg.clusters
		p.ranked, p.stories = msg.ranked, msg.stories
		p.err, p.loaded = msg.err, true
		return nil, false

	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			p.mode = (p.mode - 1 + modeCount) % modeCount
			p.cursor, p.detail = 0, false
			return nil, true

		case "right", "l":
			p.mode = (p.mode + 1) % modeCount
			p.cursor, p.detail = 0, false
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
	switch p.mode {
	case modeItems:
		return len(p.items)
	case modeClusters:
		return len(p.clusters)
	case modeRanked:
		return len(p.ranked)
	default:
		return len(p.stories)
	}
}

func loadBrowseData(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		var out browseDataMsg

		items, err := store.LoadItemsSince(cfg.ItemsDir(), time.Now().UTC(), 7)
		if err != nil {
			out.err = err
		}
		out.items = items

		// Each intermediate is optional: the stage that writes it may not have
		// run yet, which is not an error worth showing.
		if c, err := store.ReadJSONOr(cfg.DataDir+"/clusters.json", []model.Cluster(nil)); err == nil {
			out.clusters = c
		}
		if r, err := store.ReadJSONOr(cfg.DataDir+"/ranked.json", []score.Result(nil)); err == nil {
			out.ranked = r
		}
		if s, err := store.ReadJSONOr(cfg.StoriesPath(), []model.Story(nil)); err == nil {
			out.stories = s
		}
		return out
	}
}

func (p *browsePage) View(m *Model, width, height int) string {
	var b strings.Builder

	counts := [modeCount]int{len(p.items), len(p.clusters), len(p.ranked), len(p.stories)}
	for i, label := range modeLabels {
		text := fmt.Sprintf(" %s (%d) ", label, counts[i])
		if p.mode == i {
			b.WriteString(styleSelected.Render(text))
		} else {
			b.WriteString(styleDim.Render(text))
		}
		b.WriteString(" ")
	}
	b.WriteString("\n\n")

	if p.err != nil {
		b.WriteString(styleError.Render("  " + p.err.Error()))
		return b.String()
	}
	if p.count() == 0 {
		b.WriteString(styleDim.Render("  nothing here yet — " + emptyHint(p.mode)))
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

func emptyHint(mode int) string {
	switch mode {
	case modeItems:
		return "run ingest"
	case modeClusters:
		return "run cluster"
	case modeRanked:
		return "run rank"
	default:
		return "run write"
	}
}

func (p *browsePage) rowFor(i, width int) string {
	switch p.mode {
	case modeItems:
		it := p.items[i]
		return pad(truncate(it.SourceID, 17), 18) +
			pad(fmt.Sprintf("t%d", it.SourceTier), 4) +
			pad(relativeTime(it.PublishedAt), 10) +
			truncate(it.Title, max(width-36, 20))

	case modeClusters:
		c := p.clusters[i]
		lead := "(empty)"
		if len(c.Items) > 0 {
			lead = c.Items[0].Title
		}
		return pad(fmt.Sprintf("%d items", len(c.Items)), 10) +
			truncate(lead, max(width-16, 20))

	case modeRanked:
		r := p.ranked[i]
		badge := "   "
		if r.BuilderRelevant {
			badge = "SHIP"
		}
		return pad(fmt.Sprintf("%3d", r.Score), 5) +
			tierStyle(r.BestTier).Render(pad(r.Tier, 9)) +
			pad(fmt.Sprintf("%dsrc", r.DistinctSources), 7) +
			pad(badge, 6) +
			truncate(r.Title, max(width-30, 20))

	default:
		s := p.stories[i]
		badge := "   "
		if s.BuilderRelevant {
			badge = "SHIP"
		}
		return pad(fmt.Sprintf("%3d", s.Score), 5) +
			pad(s.Tier, 9) +
			pad(fmt.Sprintf("%dsrc", len(s.Sources)), 7) +
			pad(badge, 6) +
			truncate(s.Title, max(width-30, 20))
	}
}

func (p *browsePage) detailView(width int) string {
	w := max(width-4, 30)

	switch p.mode {
	case modeItems:
		return p.itemDetail(p.items[p.cursor], w)
	case modeClusters:
		return p.clusterDetail(p.clusters[p.cursor], w)
	case modeRanked:
		return p.rankedDetail(p.ranked[p.cursor], w)
	default:
		return p.storyDetail(p.stories[p.cursor], w)
	}
}

func (p *browsePage) itemDetail(it model.Item, w int) string {
	var b strings.Builder

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

func (p *browsePage) clusterDetail(c model.Cluster, w int) string {
	var b strings.Builder

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

// rankedDetail shows the score breakdown, which is what makes a weight change
// explicable rather than a guess.
func (p *browsePage) rankedDetail(r score.Result, w int) string {
	var b strings.Builder

	b.WriteString(styleHeader.Render(truncate(r.Title, w)) + "\n\n")
	b.WriteString(statLine("score", fmt.Sprintf("%d  (%s)", r.Score, r.Tier)))
	b.WriteString(statLine("raw", fmt.Sprintf("%.2f before decay", r.Raw)))
	b.WriteString(statLine("decay", fmt.Sprintf("%.3f over %.1f hours", r.Decay, r.AgeHours)))
	b.WriteString(statLine("sources", fmt.Sprintf("%d distinct, best tier %d (weight %.2f)",
		r.DistinctSources, r.BestTier, r.MaxTierWeight)))

	if r.HNPoints > 0 || r.RedditScore > 0 {
		b.WriteString(statLine("community", fmt.Sprintf("hn %d · reddit %d", r.HNPoints, r.RedditScore)))
	}
	b.WriteString(statLine("builder", fmt.Sprintf("%v", r.BuilderRelevant)))
	if len(r.Tags) > 0 {
		b.WriteString(statLine("tags", strings.Join(r.Tags, ", ")))
	}

	if len(r.BuilderSignals) > 0 {
		b.WriteString("\n" + styleLabel.Render("builder signals") + "\n")
		b.WriteString(wrap(strings.Join(r.BuilderSignals, ", "), w))
		b.WriteString("\n")
	}
	if len(r.HypeSignals) > 0 {
		b.WriteString("\n" + styleError.Render("hype signals") + "\n")
		b.WriteString(wrap(strings.Join(r.HypeSignals, ", "), w))
	}
	return b.String()
}

// storyDetail shows the published record, including every outbound link (R3).
func (p *browsePage) storyDetail(s model.Story, w int) string {
	var b strings.Builder

	b.WriteString(styleHeader.Render(truncate(s.Title, w)) + "\n\n")
	b.WriteString(statLine("score", fmt.Sprintf("%d  (%s)", s.Score, s.Tier)))
	b.WriteString(statLine("slug", truncate(s.Slug, w-20)))
	b.WriteString(statLine("date", s.Date.Format(time.RFC3339)))
	b.WriteString(statLine("cluster size", fmt.Sprint(s.ClusterSize)))
	if len(s.Tags) > 0 {
		b.WriteString(statLine("tags", strings.Join(s.Tags, ", ")))
	}

	b.WriteString("\n" + styleLabel.Render("summary") +
		styleDim.Render(fmt.Sprintf("  (%d words)", len(strings.Fields(s.Summary)))) + "\n")
	b.WriteString(wrap(s.Summary, w) + "\n")

	if len(s.Takeaways) > 0 {
		b.WriteString("\n" + styleLabel.Render("takeaways") + "\n")
		for _, t := range s.Takeaways {
			b.WriteString(styleValue.Render("  · "+truncate(t, w-4)) + "\n")
		}
	}

	b.WriteString("\n" + styleLabel.Render(fmt.Sprintf("sources (%d)", len(s.Sources))) + "\n")
	for _, src := range s.Sources {
		b.WriteString(tierStyle(src.Tier).Render(pad(fmt.Sprintf("  t%d", src.Tier), 6)) +
			styleValue.Render(pad(truncate(src.Name, 22), 24)) +
			styleDim.Render(truncate(src.URL, max(w-32, 16))) + "\n")
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
