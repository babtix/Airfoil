package tui

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/papitsho/airfoil/internal/config"
)

// scoringPage edits config/scoring.json — every knob the ranking depends on.
//
// The arrow keys nudge a value by its natural step, which is how RUNBOOK says
// to tune: move the threshold in 0.02 steps and look at the result.
type scoringPage struct {
	scoring config.Scoring
	rows    []scoringRow
	cursor  int
	dirty   bool
	input   textinput.Model
	active  bool
}

// scoringRow is either a section heading or an editable numeric field.
type scoringRow struct {
	section string // non-empty makes this a heading

	label string
	help  string
	get   func(*config.Scoring) string
	set   func(*config.Scoring, string) error
	step  func(*config.Scoring, int)
}

func (r scoringRow) isHeading() bool { return r.section != "" }

func newScoringPage(cfg *config.Config) *scoringPage {
	in := textinput.New()
	in.Prompt = "› "
	in.CharLimit = 40

	p := &scoringPage{scoring: cfg.Scoring, input: in}
	p.rows = scoringRows()
	p.cursor = p.nextEditable(0, +1)
	return p
}

func (p *scoringPage) Init() tea.Cmd { return nil }

func (p *scoringPage) Footer() string {
	if p.active {
		return styleKey.Render("enter") + styleFooter.Render(" accept  ") +
			styleKey.Render("esc") + styleFooter.Render(" cancel")
	}
	dirty := ""
	if p.dirty {
		dirty = styleWarn.Render("  unsaved")
	}
	return styleKey.Render("↑↓") + styleFooter.Render(" field  ") +
		styleKey.Render("←→") + styleFooter.Render(" nudge  ") +
		styleKey.Render("enter") + styleFooter.Render(" type  ") +
		styleKey.Render("s") + styleFooter.Render(" save  ") +
		styleKey.Render("r") + styleFooter.Render(" reload") + dirty
}

// nextEditable finds the next row that is not a heading, staying put when
// there is none. Returning `from` would hand back an out-of-range index at
// either end of the list.
func (p *scoringPage) nextEditable(from, dir int) int {
	for i := from; i >= 0 && i < len(p.rows); i += dir {
		if !p.rows[i].isHeading() {
			return i
		}
	}
	return p.cursor
}

func (p *scoringPage) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}

	if p.active {
		switch key.String() {
		case "enter":
			if err := p.rows[p.cursor].set(&p.scoring, p.input.Value()); err != nil {
				m.setError(err)
				return nil, true
			}
			p.active = false
			p.dirty = true
			m.setStatus("")
			return nil, true

		case "esc":
			p.active = false
			return nil, true
		}

		var cmd tea.Cmd
		p.input, cmd = p.input.Update(key)
		return cmd, true
	}

	switch key.String() {
	case "up", "k":
		if next := p.nextEditable(p.cursor-1, -1); next != p.cursor {
			p.cursor = next
		}
		return nil, true

	case "down", "j":
		if next := p.nextEditable(p.cursor+1, +1); next != p.cursor {
			p.cursor = next
		}
		return nil, true

	case "left":
		p.rows[p.cursor].step(&p.scoring, -1)
		p.dirty = true
		return nil, true

	case "right":
		p.rows[p.cursor].step(&p.scoring, +1)
		p.dirty = true
		return nil, true

	case "enter":
		p.input.SetValue(p.rows[p.cursor].get(&p.scoring))
		p.input.CursorEnd()
		p.input.Focus()
		p.active = true
		return nil, true

	case "s":
		if err := m.cfg.SaveScoring(p.scoring); err != nil {
			m.setError(err)
			return nil, true
		}
		p.dirty = false
		m.setStatus("saved %s", m.cfg.ScoringPath())
		return nil, true

	case "r":
		if err := m.cfg.Reload(); err != nil {
			m.setError(err)
			return nil, true
		}
		p.scoring = m.cfg.Scoring
		p.dirty = false
		m.setStatus("reloaded from disk")
		return nil, true
	}
	return nil, false
}

func (p *scoringPage) View(m *Model, width, height int) string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("scoring"))
	b.WriteString(styleDim.Render("   " + m.cfg.ScoringPath()))
	b.WriteString("\n")

	// Two columns: the list scrolls, the help text for the selected row sits
	// underneath so tuning always shows what the knob does.
	visible := max(height-4, 5)
	start := 0
	if p.cursor >= visible {
		start = p.cursor - visible + 1
	}
	end := min(start+visible, len(p.rows))

	for i := start; i < end; i++ {
		row := p.rows[i]
		if row.isHeading() {
			b.WriteString("\n" + styleHeader.Render("  "+row.section) + "\n")
			continue
		}

		label := styleLabel.Render(pad(row.label, 30))
		value := row.get(&p.scoring)

		switch {
		case i == p.cursor && p.active:
			b.WriteString(styleCursor.Render("▸ ") + label + p.input.View() + "\n")
		case i == p.cursor:
			b.WriteString(styleCursor.Render("▸ ") + label +
				styleSelected.Render(" "+pad(value, 10)+" ") +
				styleDim.Render("  "+row.help) + "\n")
		default:
			b.WriteString("  " + label + styleValue.Render(pad(value, 12)) + "\n")
		}
	}
	return b.String()
}

// floatRow builds an editable float field.
func floatRow(label, help string, ref func(*config.Scoring) *float64, step float64, lo, hi float64) scoringRow {
	return scoringRow{
		label: label,
		help:  help,
		get:   func(s *config.Scoring) string { return trimFloat(*ref(s)) },
		set: func(s *config.Scoring, v string) error {
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return fmt.Errorf("%q is not a number", v)
			}
			if f < lo || f > hi {
				return fmt.Errorf("%s must be between %s and %s", label, trimFloat(lo), trimFloat(hi))
			}
			*ref(s) = f
			return nil
		},
		step: func(s *config.Scoring, d int) {
			p := ref(s)
			next := *p + float64(d)*step
			*p = clampFloat(round(next, 4), lo, hi)
		},
	}
}

// intRow builds an editable integer field.
func intRow(label, help string, ref func(*config.Scoring) *int, step, lo, hi int) scoringRow {
	return scoringRow{
		label: label,
		help:  help,
		get:   func(s *config.Scoring) string { return strconv.Itoa(*ref(s)) },
		set: func(s *config.Scoring, v string) error {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil {
				return fmt.Errorf("%q is not a whole number", v)
			}
			if n < lo || n > hi {
				return fmt.Errorf("%s must be between %d and %d", label, lo, hi)
			}
			*ref(s) = n
			return nil
		},
		step: func(s *config.Scoring, d int) {
			p := ref(s)
			*p = clamp(*p+d*step, lo, hi)
		},
	}
}

// tierWeightRow edits one entry of the tier_weights map.
func tierWeightRow(tier int) scoringRow {
	key := strconv.Itoa(tier)
	label := fmt.Sprintf("tier %d weight", tier)

	return scoringRow{
		label: label,
		help:  tierHelp(tier),
		get: func(s *config.Scoring) string {
			return trimFloat(s.TierWeights[key])
		},
		set: func(s *config.Scoring, v string) error {
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return fmt.Errorf("%q is not a number", v)
			}
			if f < 0 || f > 1 {
				return fmt.Errorf("tier weight must be between 0 and 1")
			}
			if s.TierWeights == nil {
				s.TierWeights = map[string]float64{}
			}
			s.TierWeights[key] = f
			return nil
		},
		step: func(s *config.Scoring, d int) {
			if s.TierWeights == nil {
				s.TierWeights = map[string]float64{}
			}
			s.TierWeights[key] = clampFloat(round(s.TierWeights[key]+float64(d)*0.05, 4), 0, 1)
		},
	}
}

func tierHelp(tier int) string {
	switch tier {
	case 1:
		return "official lab blogs"
	case 2:
		return "research — arXiv, HF papers"
	case 3:
		return "dev tools and repos"
	case 4:
		return "press"
	default:
		return "community — HN, Reddit"
	}
}

func scoringRows() []scoringRow {
	return []scoringRow{
		{section: "clustering"},
		floatRow("similarity threshold",
			"raise to split merged stories, lower to group more",
			func(s *config.Scoring) *float64 { return &s.Clustering.SimilarityThreshold },
			0.02, 0.01, 1),
		intRow("window hours",
			"how far apart two items can be and still group",
			func(s *config.Scoring) *int { return &s.Clustering.WindowHours },
			6, 1, 720),
		floatRow("debug range low", "lower bound of the pairs --debug prints",
			func(s *config.Scoring) *float64 { return &s.Clustering.DebugRange[0] }, 0.01, 0, 1),
		floatRow("debug range high", "upper bound of the pairs --debug prints",
			func(s *config.Scoring) *float64 { return &s.Clustering.DebugRange[1] }, 0.01, 0, 1),

		{section: "weights"},
		floatRow("sources", "reward for distinct outlets covering one event",
			func(s *config.Scoring) *float64 { return &s.Weights.Sources }, 1, 0, 100),
		floatRow("tier", "raise when press outranks lab blogs",
			func(s *config.Scoring) *float64 { return &s.Weights.Tier }, 1, 0, 100),
		floatRow("hn", "weight on Hacker News points",
			func(s *config.Scoring) *float64 { return &s.Weights.HN }, 1, 0, 100),
		floatRow("reddit", "weight on Reddit score",
			func(s *config.Scoring) *float64 { return &s.Weights.Reddit }, 1, 0, 100),
		floatRow("builder", "reward per builder signal matched",
			func(s *config.Scoring) *float64 { return &s.Weights.Builder }, 1, 0, 100),
		floatRow("hype", "penalty per hype phrase matched",
			func(s *config.Scoring) *float64 { return &s.Weights.Hype }, 1, 0, 100),

		{section: "source tiers"},
		tierWeightRow(1), tierWeightRow(2), tierWeightRow(3), tierWeightRow(4), tierWeightRow(5),

		{section: "recency"},
		floatRow("half life hours", "shorten to age stories out faster",
			func(s *config.Scoring) *float64 { return &s.Recency.HalfLifeHours }, 6, 1, 720),

		{section: "caps"},
		intRow("builder signal max", "ceiling on counted builder signals",
			func(s *config.Scoring) *int { return &s.Caps.BuilderSignalMax }, 1, 1, 50),
		intRow("hype signal max", "ceiling on counted hype signals",
			func(s *config.Scoring) *int { return &s.Caps.HypeSignalMax }, 1, 1, 50),
		intRow("llm calls per run", "summarization budget per run",
			func(s *config.Scoring) *int { return &s.Caps.LLMCallsPerRun }, 5, 1, 500),

		{section: "story tiers"},
		intRow("major min score", "at or above this a story is major",
			func(s *config.Scoring) *int { return &s.Tiers.MajorMinScore }, 5, 1, 100),
		intRow("notable min score", "below this a story gets no LLM call",
			func(s *config.Scoring) *int { return &s.Tiers.NotableMinScore }, 5, 0, 99),
		intRow("builder relevant min", "builder signals needed for the SHIP badge",
			func(s *config.Scoring) *int { return &s.BuilderRelevantMinSignals }, 1, 1, 20),

		{section: "limits — R1 and R2"},
		intRow("excerpt max chars", "hard cap on stored source text (R1)",
			func(s *config.Scoring) *int { return &s.Limits.ExcerptMaxChars }, 50, 1, 5000),
		intRow("summary max words", "reject summaries longer than this (R1)",
			func(s *config.Scoring) *int { return &s.Limits.SummaryMaxWords }, 10, 1, 1000),
		intRow("takeaways max", "how many bullets a story carries",
			func(s *config.Scoring) *int { return &s.Limits.TakeawaysMax }, 1, 1, 10),
		intRow("verbatim overlap max", "longest run of copied words allowed (R1)",
			func(s *config.Scoring) *int { return &s.Limits.VerbatimOverlapMaxWords }, 1, 1, 100),
		intRow("quotes max", "quoted spans allowed per summary (R2)",
			func(s *config.Scoring) *int { return &s.Limits.QuotesMax }, 1, 0, 10),
		intRow("quote max words", "longest quoted span allowed (R2)",
			func(s *config.Scoring) *int { return &s.Limits.QuoteMaxWords }, 1, 1, 100),

		{section: "retention"},
		intRow("items days", "how long raw items stay on disk",
			func(s *config.Scoring) *int { return &s.Retention.ItemsDays }, 5, 1, 365),
		intRow("seen urls days", "how long deduplication remembers a url",
			func(s *config.Scoring) *int { return &s.Retention.SeenURLsDays }, 5, 1, 365),
		intRow("embedding cache days", "how long a vector stays cached",
			func(s *config.Scoring) *int { return &s.Retention.EmbeddingCacheDays }, 1, 1, 90),
	}
}

func clampFloat(v, lo, hi float64) float64 { return min(max(v, lo), hi) }

// round to n decimal places, so repeated nudges do not accumulate float noise —
// twenty presses of ← must land on exactly 0.62, not 0.6199999999.
func round(v float64, places int) float64 {
	pow := math.Pow(10, float64(places))
	return math.Round(v*pow) / pow
}

// trimFloat prints a float without trailing zeros.
func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
