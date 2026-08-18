package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/papitsho/airfoil/internal/pipeline"
)

// stage is one runnable pipeline step.
type stage struct {
	key   string
	name  string
	about string
	run   func(ctx context.Context, p *pipeline.Pipeline, opts pipeline.Options) (string, error)
}

// stages are listed in pipeline order.
var stages = []stage{
	{
		key:   "ingest",
		name:  "Ingest",
		about: "Fetch every enabled source into data/items/",
		run: func(ctx context.Context, p *pipeline.Pipeline, opts pipeline.Options) (string, error) {
			r, err := p.Ingest(ctx, opts)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d new items, %d written, %d of %d sources ok",
				r.NewItems, r.Written, r.SourcesOK, r.SourcesOK+r.SourcesFail), nil
		},
	},
	{
		key:   "cluster",
		name:  "Cluster",
		about: "Embed recent items and group them into stories",
		run: func(ctx context.Context, p *pipeline.Pipeline, opts pipeline.Options) (string, error) {
			r, err := p.Cluster(ctx, opts)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d items into %d clusters, %d multi-source, largest %d",
				r.Items, len(r.Clusters), r.MultiSource, r.Largest), nil
		},
	},
}

// runPage drives the pipeline and shows its log as it happens.
type runPage struct {
	cursor  int
	dry     bool
	running bool
	current string
	started time.Time
	cancel  context.CancelFunc
	elapsed time.Duration

	// scroll is how many lines from the bottom the log is pinned; 0 follows.
	scroll int
}

type stageDoneMsg struct {
	stage   string
	summary string
	err     error
	elapsed time.Duration
}

type tickMsg time.Time

func newRunPage() *runPage { return &runPage{} }

func (r *runPage) Init() tea.Cmd { return nil }

func (r *runPage) Footer() string {
	if r.running {
		return styleKey.Render("ctrl+x") + styleFooter.Render(" cancel  ") +
			styleKey.Render("↑↓") + styleFooter.Render(" scroll log")
	}
	return styleKey.Render("↑↓") + styleFooter.Render(" stage  ") +
		styleKey.Render("enter") + styleFooter.Render(" run  ") +
		styleKey.Render("d") + styleFooter.Render(" dry  ") +
		styleKey.Render("c") + styleFooter.Render(" clear log")
}

func (r *runPage) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case stageDoneMsg:
		r.running = false
		r.cancel = nil
		r.elapsed = msg.elapsed

		if msg.err != nil {
			m.setError(fmt.Errorf("%s: %w", msg.stage, msg.err))
			m.log.Error("stage failed", "stage", msg.stage, "error", msg.err)
		} else {
			m.setStatus("%s complete in %s — %s", msg.stage,
				msg.elapsed.Round(time.Millisecond), msg.summary)
		}
		// Refresh the dashboard numbers now that the data on disk has changed.
		return loadStats(m.cfg), false

	case tickMsg:
		if r.running {
			r.elapsed = time.Since(r.started)
			return tickEvery(), false
		}
		return nil, false

	case tea.KeyMsg:
		return r.handleKey(msg, m)
	}
	return nil, false
}

func (r *runPage) handleKey(msg tea.KeyMsg, m *Model) (tea.Cmd, bool) {
	switch msg.String() {
	case "up", "k":
		if r.running {
			r.scroll++
			return nil, true
		}
		if r.cursor > 0 {
			r.cursor--
		}
		return nil, true

	case "down", "j":
		if r.running {
			r.scroll = max(r.scroll-1, 0)
			return nil, true
		}
		if r.cursor < len(stages)-1 {
			r.cursor++
		}
		return nil, true

	case "d":
		if !r.running {
			r.dry = !r.dry
			m.setStatus("dry run %s", map[bool]string{true: "on", false: "off"}[r.dry])
		}
		return nil, true

	case "c":
		m.sink.Clear()
		r.scroll = 0
		return nil, true

	case "ctrl+x":
		if r.running && r.cancel != nil {
			r.cancel()
			m.setStatus("cancelling %s…", r.current)
		}
		return nil, true

	case "enter":
		if r.running {
			return nil, true
		}
		return r.start(m), true
	}
	return nil, false
}

// start launches the selected stage in the background.
func (r *runPage) start(m *Model) tea.Cmd {
	s := stages[r.cursor]

	ctx, cancel := context.WithCancel(context.Background())
	r.running = true
	r.cancel = cancel
	r.current = s.name
	r.started = time.Now()
	r.elapsed = 0
	r.scroll = 0

	opts := pipeline.Options{Dry: r.dry, Debug: true}
	if r.dry {
		m.log.Info("starting stage", "stage", s.key, "mode", "dry")
	} else {
		m.log.Info("starting stage", "stage", s.key)
	}

	work := func() tea.Msg {
		defer cancel()
		started := time.Now()
		summary, err := s.run(ctx, m.pipe, opts)
		return stageDoneMsg{
			stage:   s.name,
			summary: summary,
			err:     err,
			elapsed: time.Since(started),
		}
	}
	return tea.Batch(work, tickEvery())
}

// tickEvery drives the elapsed-time counter while a stage runs.
func tickEvery() tea.Cmd {
	return tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (r *runPage) View(m *Model, width, height int) string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("stages"))
	if r.dry {
		b.WriteString(styleWarn.Render("   dry run — nothing will be written"))
	}
	b.WriteString("\n")

	for i, s := range stages {
		cursor := "  "
		label := styleValue.Render(pad(s.name, 10))
		if i == r.cursor {
			cursor = styleCursor.Render("▸ ")
			label = styleSelected.Render(pad(s.name, 10))
		}

		state := ""
		if r.running && r.current == s.name {
			state = styleWarn.Render(fmt.Sprintf("  running %s", r.elapsed.Round(100*time.Millisecond)))
		}
		b.WriteString(cursor + label + styleDim.Render(pad(s.about, 52)) + state + "\n")
	}

	b.WriteString("\n" + styleHeader.Render("log") + "\n")

	lines := m.sink.Lines()
	logHeight := max(height-len(stages)-5, 3)

	if len(lines) == 0 {
		b.WriteString(styleDim.Render("  no output yet — press enter to run the selected stage"))
		return b.String()
	}

	// Pin to the bottom unless the reader has scrolled up.
	end := max(len(lines)-r.scroll, 1)
	start := max(end-logHeight, 0)

	for _, line := range lines[start:end] {
		b.WriteString("  " + truncate(line, max(width-2, 10)) + "\n")
	}
	if r.scroll > 0 {
		b.WriteString(styleDim.Render(fmt.Sprintf("  … %d newer lines below", r.scroll)))
	}
	return b.String()
}
