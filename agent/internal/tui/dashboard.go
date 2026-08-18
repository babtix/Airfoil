package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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

func (d *dashboard) View(m *Model, width, height int) string {
	cfg := m.cfg
	half := max(width/2-2, 30)

	var left strings.Builder
	left.WriteString(styleHeader.Render("pipeline") + "\n")
	s := d.stats
	left.WriteString(statLine("items on disk", fmt.Sprintf("%d across %d days", s.Items, s.ItemDays)))
	left.WriteString(statLine("clusters", fmt.Sprintf("%d (%d multi-source)", s.Clusters, s.MultiSource)))
	left.WriteString(statLine("ranked", fmt.Sprintf("%d — %d major, %d notable", s.Ranked, s.Major, s.Notable)))
	left.WriteString(statLine("stories", fmt.Sprintf("%d (%d markdown files)", s.Stories, s.Markdown)))
	left.WriteString(statLine("digest files", fmt.Sprint(s.DigestFiles)))
	left.WriteString(statLine("seen urls", fmt.Sprint(s.SeenURLs)))

	lastRun := styleDim.Render("never")
	if s.LastRun != nil {
		lastRun = styleValue.Render(relativeTime(*s.LastRun))
	}
	left.WriteString(styleLabel.Render(pad("last run", 18)) + lastRun + "\n")

	if s.Err != nil {
		left.WriteString("\n" + styleError.Render(truncate(s.Err.Error(), half)))
	}

	var right strings.Builder
	right.WriteString(styleHeader.Render("configuration") + "\n")

	enabled := cfg.EnabledSources()
	right.WriteString(statLine("sources", fmt.Sprintf("%d of %d enabled", len(enabled), len(cfg.Sources))))
	right.WriteString(statLine("embedder", cfg.Embed.Provider+" · "+cfg.Embed.Model()))
	right.WriteString(statLine("threshold", fmt.Sprintf("%.2f over %dh",
		cfg.Scoring.Clustering.SimilarityThreshold, cfg.Scoring.Clustering.WindowHours)))
	right.WriteString(statLine("llm budget", fmt.Sprintf("%d calls per run", cfg.Scoring.Caps.LLMCallsPerRun)))
	right.WriteString(statLine("config dir", cfg.ConfigDir))
	right.WriteString(statLine("data dir", cfg.DataDir))

	body := columns(left.String(), right.String(), half, 2)

	// Provider readiness: the thing most likely to stop a run.
	var providers strings.Builder
	providers.WriteString("\n" + styleHeader.Render("providers") + "\n")
	for _, p := range providerStatuses(cfg) {
		mark := styleDim.Render("missing")
		if p.ready {
			mark = styleOK.Render("ready")
		}
		providers.WriteString("  " + styleLabel.Render(pad(p.name, 14)) +
			pad(mark, 18) + styleDim.Render(p.detail) + "\n")
	}

	var sources strings.Builder
	sources.WriteString("\n" + styleHeader.Render("sources by tier") + "\n")
	byTier := map[int][]string{}
	for _, src := range enabled {
		byTier[src.Tier] = append(byTier[src.Tier], src.ID)
	}
	for tier := 1; tier <= 5; tier++ {
		ids := byTier[tier]
		if len(ids) == 0 {
			continue
		}
		sources.WriteString("  " + tierStyle(tier).Render(fmt.Sprintf("tier %d", tier)) +
			styleDim.Render(fmt.Sprintf(" (%d)  ", len(ids))) +
			styleLabel.Render(truncate(strings.Join(ids, ", "), max(width-24, 20))) + "\n")
	}

	return body + providers.String() + sources.String()
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
		{"gemini", cfg.LLM.Gemini.Configured(), cfg.LLM.Gemini.Model},
		{"nvidia_nim", cfg.LLM.NvidiaNIM.Configured(), cfg.LLM.NvidiaNIM.Model},
		{"openrouter", cfg.LLM.OpenRouter.Configured(), cfg.LLM.OpenRouter.Model},
	}
}

func statLine(label, value string) string {
	return styleLabel.Render(pad(label, 18)) + styleValue.Render(value) + "\n"
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
