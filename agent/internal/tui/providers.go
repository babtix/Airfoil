package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/papitsho/airfoil/internal/config"
)

// providersPage edits .env credentials and provider model IDs directly from the terminal.
type providersPage struct {
	values map[string]string
	rows   []providerRow
	cursor int
	dirty  bool
	input  textinput.Model
	active bool
}

type providerRow struct {
	section string // non-empty makes this a section heading

	key   string
	label string
	help  string
	mask  bool
	cycle func(string, int) string // optional for cyclic options
}

func (r providerRow) isHeading() bool { return r.section != "" }

func newProvidersPage(cfg *config.Config) *providersPage {
	in := textinput.New()
	in.Prompt = "› "
	in.CharLimit = 200

	p := &providersPage{
		values: cfg.AsEnvMap(),
		input:  in,
	}
	p.rows = providerRows()
	p.cursor = p.nextEditable(0, +1)
	return p
}

func providerRows() []providerRow {
	logLevels := []string{"debug", "info", "warn", "error"}
	cycleLogLevel := func(cur string, dir int) string {
		idx := 1 // default info
		for i, lvl := range logLevels {
			if strings.EqualFold(lvl, cur) {
				idx = i
				break
			}
		}
		next := (idx + dir + len(logLevels)) % len(logLevels)
		return logLevels[next]
	}

	return []providerRow{
		{section: "OpenRouter (Primary LLM)"},
		{
			key:   "OPENROUTER_API_KEY",
			label: "OpenRouter key",
			help:  "OpenRouter API key (sk-or-...)",
			mask:  true,
		},
		{
			key:   "OPENROUTER_MODEL",
			label: "primary model",
			help:  "Primary LLM for synthesis (e.g. meta-llama/llama-3.3-70b-instruct:free)",
		},

		{section: "NVIDIA NIM (Fallback LLM & Embedder)"},
		{
			key:   "NVIDIA_NIM_API_KEY",
			label: "NIM API key",
			help:  "NVIDIA NIM API key (starts with nvapi-)",
			mask:  true,
		},
		{
			key:   "NVIDIA_NIM_MODEL",
			label: "fallback model",
			help:  "Fallback LLM for synthesis (e.g. meta/llama-3.3-70b-instruct)",
		},
		{
			key:   "AIRFOIL_EMBEDDER",
			label: "embedder provider",
			help:  "Vector embedding engine (nvidia_nim)",
		},
		{
			key:   "NVIDIA_NIM_EMBED_MODEL",
			label: "embedder model",
			help:  "Vector embedding model (e.g. nvidia/nv-embedqa-e5-v5)",
		},
		{
			key:   "NVIDIA_NIM_BASE_URL",
			label: "NIM base url",
			help:  "NVIDIA NIM API base URL",
		},

		{section: "Ingestion Credentials & Rate Limits"},
		{
			key:   "GITHUB_TOKEN",
			label: "github token",
			help:  "Optional PAT raising rate limit 60 → 5000 req/hr",
			mask:  true,
		},
		{
			key:   "REDDIT_USER_AGENT",
			label: "reddit user-agent",
			help:  "Descriptive User-Agent header to avoid 429 rate limits",
		},

		{section: "Deployment & Runtime"},
		{
			key:   "AIRFOIL_LOG_LEVEL",
			label: "log level",
			help:  "debug, info, warn, error",
			cycle: cycleLogLevel,
		},
		{
			key:   "SITE_URL",
			label: "site url",
			help:  "Public site base URL for digests and RSS",
		},
	}
}

func (p *providersPage) Init() tea.Cmd { return nil }

func (p *providersPage) Footer() string {
	if p.active {
		return styleKey.Render("enter") + styleFooter.Render(" accept  ") +
			styleKey.Render("esc") + styleFooter.Render(" cancel")
	}
	dirty := ""
	if p.dirty {
		dirty = styleWarn.Render("  unsaved")
	}
	return styleKey.Render("↑↓") + styleFooter.Render(" field  ") +
		styleKey.Render("enter") + styleFooter.Render(" edit  ") +
		styleKey.Render("←→") + styleFooter.Render(" cycle  ") +
		styleKey.Render("s") + styleFooter.Render(" save .env  ") +
		styleKey.Render("r") + styleFooter.Render(" reload") + dirty
}

func (p *providersPage) nextEditable(from, dir int) int {
	for i := from; i >= 0 && i < len(p.rows); i += dir {
		if !p.rows[i].isHeading() {
			return i
		}
	}
	return p.cursor
}

func (p *providersPage) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}

	if p.active {
		switch key.String() {
		case "enter":
			row := p.rows[p.cursor]
			p.values[row.key] = strings.TrimSpace(p.input.Value())
			p.active = false
			p.dirty = true
			m.setStatus("")
			return nil, true

		case "esc":
			p.active = false
			m.setStatus("")
			return nil, true

		default:
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			return cmd, true
		}
	}

	switch key.String() {
	case "up", "k":
		p.cursor = p.nextEditable(p.cursor-1, -1)
		return nil, true

	case "down", "j":
		p.cursor = p.nextEditable(p.cursor+1, +1)
		return nil, true

	case "enter":
		row := p.rows[p.cursor]
		if row.isHeading() {
			return nil, true
		}
		p.input.SetValue(p.values[row.key])
		p.input.CursorEnd()
		p.input.Focus()
		p.active = true
		return textinput.Blink, true

	case "left", "h":
		row := p.rows[p.cursor]
		if row.cycle != nil {
			p.values[row.key] = row.cycle(p.values[row.key], -1)
			p.dirty = true
			return nil, true
		}

	case "right", "l":
		row := p.rows[p.cursor]
		if row.cycle != nil {
			p.values[row.key] = row.cycle(p.values[row.key], +1)
			p.dirty = true
			return nil, true
		}

	case "s":
		envPath := filepath.Join(m.cfg.ConfigDir, "..", ".env")
		if err := config.SaveDotEnv(envPath, p.values); err != nil {
			// Fallback to local .env
			if err2 := config.SaveDotEnv(".env", p.values); err2 != nil {
				m.setError(fmt.Errorf("save .env: %w", err))
				return nil, true
			}
		}
		m.cfg.SyncEnv(p.values)
		p.dirty = false
		m.setStatus("saved provider configuration to .env")
		return nil, true

	case "r":
		p.values = m.cfg.AsEnvMap()
		p.dirty = false
		m.setStatus("reloaded configuration from memory")
		return nil, true
	}

	return nil, false
}

func (p *providersPage) View(m *Model, width, height int) string {
	var b strings.Builder

	for i, row := range p.rows {
		if row.isHeading() {
			b.WriteString("\n" + styleHeader.Render("  "+row.section) + "\n")
			continue
		}

		label := styleLabel.Render(pad(row.label, 24))
		val := p.values[row.key]
		display := formatDisplayValue(val, row.mask)

		switch {
		case i == p.cursor && p.active:
			b.WriteString(styleCursor.Render("▸ ") + label + p.input.View() + "\n")
		case i == p.cursor:
			b.WriteString(styleCursor.Render("▸ ") + label +
				styleSelected.Render(" "+pad(display, 28)+" ") +
				styleDim.Render("  "+row.help) + "\n")
		default:
			b.WriteString("  " + label + styleValue.Render(pad(display, 30)) + "\n")
		}
	}

	return b.String()
}

// formatDisplayValue masks secrets for shoulder-surfing safety.
func formatDisplayValue(val string, mask bool) string {
	if val == "" {
		return styleDim.Render("(not set)")
	}
	if !mask {
		return val
	}
	if len(val) <= 8 {
		return "••••••••"
	}
	// Show prefix (e.g. nvapi- or sk-) + masked body + last 4 chars
	prefix := ""
	if strings.HasPrefix(val, "nvapi-") {
		prefix = "nvapi-"
	} else if strings.HasPrefix(val, "sk-") {
		prefix = "sk-"
	} else if len(val) > 10 {
		prefix = val[:4]
	}
	suffix := val[len(val)-4:]
	return prefix + "••••" + suffix
}
