package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/score"
	"github.com/papitsho/airfoil/internal/store"
)

// dashboard is the at-a-glance view: what is configured, what is on disk, and
// whether the agent can actually run right now.
type dashboard struct {
	stats  dataStats
	loaded bool
}

type dataStats struct {
	Items       int
	ItemDays    int
	Clusters    int
	MultiSource int
	Ranked      int
	Major       int
	Notable     int
	Stories     int
	Markdown    int
	DigestFiles int
	SeenURLs    int
	LastRun     *time.Time
	Err         error
}

type statsMsg dataStats

func newDashboard() *dashboard { return &dashboard{} }

func (d *dashboard) Init() tea.Cmd { return nil }

func (d *dashboard) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case statsMsg:
		d.stats, d.loaded = dataStats(msg), true
		return nil, false

	case tea.KeyMsg:
		if msg.String() == "R" {
			d.loaded = false
			return loadStats(m.cfg), true
		}
	}

	// Load lazily on the first render rather than blocking startup.
	if !d.loaded {
		d.loaded = true
		return loadStats(m.cfg), false
	}
	return nil, false
}

// loadStats reads what is on disk. It runs as a command so a large items file
// never stalls the interface.
func loadStats(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		var out dataStats

		state, err := store.LoadState(cfg.StatePath())
		if err != nil {
			out.Err = err
			return statsMsg(out)
		}
		out.SeenURLs = len(state.SeenURLs)
		out.LastRun = state.LastRun

		items, err := store.LoadItemsSince(cfg.ItemsDir(), time.Now().UTC(), 30)
		if err == nil {
			out.Items = len(items)
			days := map[string]bool{}
			for _, it := range items {
				days[it.PublishedAt.Format(time.DateOnly)] = true
			}
			out.ItemDays = len(days)
		}

		clusters, err := store.ReadJSONOr(cfg.DataDir+"/clusters.json", []model.Cluster(nil))
		if err == nil {
			out.Clusters = len(clusters)
			for _, c := range clusters {
				if len(c.Items) > 1 {
					out.MultiSource++
				}
			}
		}

		ranked, err := store.ReadJSONOr(cfg.DataDir+"/ranked.json", []score.Result(nil))
		if err == nil {
			out.Ranked = len(ranked)
			for _, r := range ranked {
				switch r.Tier {
				case model.TierMajor:
					out.Major++
				case model.TierNotable:
					out.Notable++
				}
			}
		}

		stories, err := store.ReadJSONOr(cfg.StoriesPath(), []model.Story(nil))
		if err == nil {
			out.Stories = len(stories)
		}

		// The markdown collection is the product; count it directly rather
		// than trusting stories.json to match.
		if files, err := filepath.Glob(filepath.Join(cfg.StoriesDir, "*.md")); err == nil {
			out.Markdown = len(files)
		}
		if files, err := filepath.Glob(filepath.Join(cfg.DigestDir(), "*")); err == nil {
			out.DigestFiles = len(files)
		}

		return statsMsg(out)
	}
}

func (d *dashboard) Footer() string {
	return styleKey.Render("R") + styleFooter.Render(" refresh")
}

// bannerMinHeight is the body height below which the wordmark gives way to the
// numbers. Five rows of block face plus its margins are a poor trade on a short
// terminal.
const bannerMinHeight = 22

func (d *dashboard) View(m *Model, width, height int) string {
	cfg, s := m.cfg, d.stats

	// Two columns of panels, a single blank column between them.
	colWidth := max((width-1)/2, 24)
	inner := max(colWidth-4, 12)

	left := lipgloss.JoinVertical(lipgloss.Left,
		panel("pipeline", d.pipelineBody(inner), colWidth),
		panel("providers", providersBody(cfg, inner), colWidth),
	)
	right := lipgloss.JoinVertical(lipgloss.Left,
		panel("configuration", configBody(cfg, inner), colWidth),
		panel("sources by tier", tierBody(cfg, inner), colWidth),
	)

	var b strings.Builder

	// The mark leads the first screen when the terminal has room for it; the
	// numbers win the space otherwise.
	if banner := logoBanner(width); banner != "" && height >= bannerMinHeight {
		b.WriteString(banner + "\n")
	}
	b.WriteString(center(readiness(cfg, s, width), width) + "\n\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right))

	if s.Err != nil {
		b.WriteString("\n" + styleError.Render(truncate(s.Err.Error(), width)))
	}
	return b.String()
}

// readiness answers the only question the dashboard exists to answer: can the
// agent run right now, and how stale is what is on disk.
func readiness(cfg *config.Config, s dataStats, width int) string {
	sources := len(cfg.EnabledSources())

	ready := 0
	for _, p := range providerStatuses(cfg) {
		if p.ready {
			ready++
		}
	}

	mark, label := styleOK.Render("●"), styleOK.Render("ready")
	switch {
	case ready == 0 || sources == 0:
		mark, label = styleError.Render("●"), styleError.Render("not runnable")
	case ready < len(providerStatuses(cfg)):
		mark, label = styleWarn.Render("●"), styleWarn.Render("degraded — one provider")
	}

	last := "never run"
	if s.LastRun != nil {
		last = "last run " + relativeTime(*s.LastRun)
	}

	head := mark + " " + label
	detail := fmt.Sprintf("  ·  %d sources  ·  %d stories  ·  %s", sources, s.Stories, last)
	return head + styleDim.Render(truncate(detail, max(width-lipgloss.Width(head), 0)))
}

// pipelineBody draws the funnel: how many items survived each narrowing, with a
// bar in the same solid block the wordmark is cut from.
func (d *dashboard) pipelineBody(inner int) string {
	s := d.stats
	if !d.loaded {
		return styleDim.Render("reading data/…")
	}

	rows := []struct {
		label string
		n     int
		note  string
	}{
		{"items", s.Items, fmt.Sprintf("%d days", s.ItemDays)},
		{"clusters", s.Clusters, fmt.Sprintf("%d multi", s.MultiSource)},
		{"ranked", s.Ranked, fmt.Sprintf("%d maj·%d not", s.Major, s.Notable)},
		{"stories", s.Stories, fmt.Sprintf("%d md", s.Markdown)},
	}

	peak := 1
	for _, r := range rows {
		peak = max(peak, r.n)
	}

	// label + count + bar + note, with the bar taking what the fixed fields
	// leave and never dropping below a stub.
	const labelW, countW, noteW = 9, 6, 14
	barW := max(inner-labelW-countW-noteW-3, 4)

	var b strings.Builder
	for _, r := range rows {
		b.WriteString(styleLabel.Render(pad(r.label, labelW)) +
			styleValue.Render(padLeft(fmt.Sprint(r.n), countW)) + " " +
			bar(r.n, peak, barW) + " " +
			styleDim.Render(truncate(r.note, noteW)) + "\n")
	}
	b.WriteString(stat("digests", fmt.Sprint(s.DigestFiles), inner))
	b.WriteString(stat("seen urls", fmt.Sprint(s.SeenURLs), inner))
	return strings.TrimRight(b.String(), "\n")
}

func configBody(cfg *config.Config, inner int) string {
	var b strings.Builder
	b.WriteString(stat("sources", fmt.Sprintf("%d of %d enabled",
		len(cfg.EnabledSources()), len(cfg.Sources)), inner))
	b.WriteString(stat("embedder", cfg.Embed.Provider+" · "+cfg.Embed.Model(), inner))
	b.WriteString(stat("threshold", fmt.Sprintf("%.2f over %dh",
		cfg.Scoring.Clustering.SimilarityThreshold, cfg.Scoring.Clustering.WindowHours), inner))
	b.WriteString(stat("llm budget", fmt.Sprintf("%d calls per run",
		cfg.Scoring.Caps.LLMCallsPerRun), inner))
	b.WriteString(stat("config dir", cfg.ConfigDir, inner))
	b.WriteString(stat("data dir", cfg.DataDir, inner))
	return strings.TrimRight(b.String(), "\n")
}

func providersBody(cfg *config.Config, inner int) string {
	var b strings.Builder
	for _, p := range providerStatuses(cfg) {
		mark := styleDim.Render(pad("missing", 8))
		if p.ready {
			mark = styleOK.Render(pad("ready", 8))
		}
		b.WriteString(styleLabel.Render(pad(p.name, 12)) + mark +
			styleDim.Render(truncate(p.detail, max(inner-20, 6))) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// tierBody groups the enabled sources the way scoring weights them, so a tier
// that has quietly emptied out is visible at a glance.
func tierBody(cfg *config.Config, inner int) string {
	byTier := map[int][]string{}
	for _, src := range cfg.EnabledSources() {
		byTier[src.Tier] = append(byTier[src.Tier], src.ID)
	}
	peak := 1
	for _, ids := range byTier {
		peak = max(peak, len(ids))
	}

	var b strings.Builder
	for tier := 1; tier <= 5; tier++ {
		ids := byTier[tier]
		if len(ids) == 0 {
			continue
		}
		const labelW, barW = 7, 8
		b.WriteString(tierStyle(tier).Render(pad(fmt.Sprintf("tier %d", tier), labelW)) +
			tierStyle(tier).Render(pad(repeat("█", scale(len(ids), peak, barW)), barW)) + " " +
			styleDim.Render(truncate(strings.Join(ids, ", "), max(inner-labelW-barW-1, 6))) + "\n")
	}
	if b.Len() == 0 {
		return styleWarn.Render("no sources enabled")
	}
	return strings.TrimRight(b.String(), "\n")
}

type providerStatus struct {
	name   string
	ready  bool
	detail string
}

// providerStatuses reports which providers are configured. This checks that a
// key is present, not that it works — `doctor` makes the live calls.
func providerStatuses(cfg *config.Config) []providerStatus {
	return []providerStatus{
		{"nvidia_nim", cfg.LLM.NvidiaNIM.Configured(), cfg.LLM.NvidiaNIM.Model},
		{"openrouter", cfg.LLM.OpenRouter.Configured(), cfg.LLM.OpenRouter.Model},
	}
}

// stat renders one "label  value" row, giving the label at most half the room.
func stat(label, value string, inner int) string {
	labelW := min(18, max(inner/2, 6))
	return styleLabel.Render(pad(label, labelW)) +
		styleValue.Render(truncate(value, max(inner-labelW, 4))) + "\n"
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
