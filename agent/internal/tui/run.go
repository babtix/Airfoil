package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/papitsho/airfoil/internal/pipeline"
	"github.com/papitsho/airfoil/internal/publish"
)

// stageOpts carries what a stage needs beyond the pipeline defaults.
type stageOpts struct {
	pipeline.Options
	// Push sends the publish commit to the remote. Off unless explicitly
	// turned on and confirmed.
	Push bool
}

// stage is one runnable pipeline step.
type stage struct {
	key   string
	name  string
	about string
	// needsLLM marks stages that cannot do anything without a provider, so the
	// interface can say so before spending a run on it.
	needsLLM bool
	// confirms marks stages that reach outside this machine.
	confirms bool
	run      func(ctx context.Context, p *pipeline.Pipeline, opts stageOpts) (string, error)
}

// stages are listed in pipeline order, with the whole-pipeline run last.
var stages = []stage{
	{
		key:   "ingest",
		name:  "Ingest",
		about: "Fetch every enabled source into data/items/",
		run: func(ctx context.Context, p *pipeline.Pipeline, o stageOpts) (string, error) {
			r, err := p.Ingest(ctx, o.Options)
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
		run: func(ctx context.Context, p *pipeline.Pipeline, o stageOpts) (string, error) {
			r, err := p.Cluster(ctx, o.Options)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d items into %d clusters, %d multi-source, largest %d",
				r.Items, len(r.Clusters), r.MultiSource, r.Largest), nil
		},
	},
	{
		key:   "rank",
		name:  "Rank",
		about: "Score every cluster and pick the LLM budget",
		run: func(ctx context.Context, p *pipeline.Pipeline, o stageOpts) (string, error) {
			r, err := p.Rank(ctx, o.Options)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d major, %d notable, %d minor · %d builder-relevant · %d budgeted",
				r.Major, r.Notable, r.Minor, r.Builder, r.Budgeted), nil
		},
	},
	{
		key:      "write",
		name:     "Write",
		about:    "Summarize the top clusters and emit markdown",
		needsLLM: true,
		run: func(ctx context.Context, p *pipeline.Pipeline, o stageOpts) (string, error) {
			r, err := p.Write(ctx, o.Options)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%d summarized, %d skipped, %d created, %d updated",
				r.Summarized, r.Skipped, r.Stats.Created, r.Stats.Updated), nil
		},
	},
	{
		key:      "digest",
		name:     "Digest",
		about:    "Build the newsletter, X thread, and LinkedIn draft",
		needsLLM: true,
		run: func(ctx context.Context, p *pipeline.Pipeline, o stageOpts) (string, error) {
			r, err := p.Digest(ctx, time.Now().UTC(), o.Dry)
			if err != nil {
				return "", err
			}
			summary := fmt.Sprintf("%d stories, %d files (view in 9 Digest)", len(r.Stories), len(r.Files))
			if len(r.Failures) > 0 {
				summary += fmt.Sprintf(", %d parts not generated", len(r.Failures))
			}
			return summary, nil
		},
	},
	{
		key:      "publish",
		name:     "Publish",
		about:    "Validate, prune, and commit — push only when asked",
		confirms: true,
		run: func(ctx context.Context, p *pipeline.Pipeline, o stageOpts) (string, error) {
			root, err := p.RepoRoot()
			if err != nil {
				return "", err
			}
			r, err := p.Publish(ctx, publish.Options{Dry: o.Dry, Push: o.Push}, root)
			if err != nil {
				// The validation problems are the useful part of this failure.
				if len(r.Problems) > 0 {
					return "", fmt.Errorf("%w — first problem: %v", err, r.Problems[0])
				}
				return "", err
			}
			switch {
			case r.Pushed && r.Committed:
				return fmt.Sprintf("%d stories committed and pushed — %s", r.Stories, r.Commit), nil
			case r.Pushed:
				return fmt.Sprintf("%d stories up to date — pushed to remote", r.Stories), nil
			case r.Committed:
				return fmt.Sprintf("%d stories committed — %s", r.Stories, r.Commit), nil
			default:
				return fmt.Sprintf("%d stories validated, nothing committed", r.Stories), nil
			}
		},
	},
	{
		key:      "run",
		name:     "Run all",
		about:    "Ingest, cluster, rank, then write — in order",
		needsLLM: true,
		confirms: true,
		run: func(ctx context.Context, p *pipeline.Pipeline, o stageOpts) (string, error) {
			ing, err := p.Ingest(ctx, o.Options)
			if err != nil {
				return "", err
			}
			// A dry run wrote no items, so clustering here would group whatever
			// the previous run left behind and report it as this pass's work.
			if o.Dry {
				return fmt.Sprintf("%d new items — dry run stops before cluster", ing.NewItems), nil
			}

			cl, err := p.Cluster(ctx, o.Options)
			if err != nil {
				return "", err
			}
			rk, err := p.Rank(ctx, o.Options)
			if err != nil {
				return "", err
			}
			wr, err := p.Write(ctx, o.Options)
			if err != nil {
				return "", err
			}
			baseSummary := fmt.Sprintf("%d items into %d clusters, %d major and %d notable, %d summarized, %d created",
				cl.Items, len(cl.Clusters), rk.Major, rk.Notable, wr.Summarized, wr.Stats.Created)
			if o.Push {
				root, err := p.RepoRoot()
				if err != nil {
					return "", err
				}
				pub, err := p.Publish(ctx, publish.Options{Push: true}, root)
				if err != nil {
					if len(pub.Problems) > 0 {
						return "", fmt.Errorf("%w — first problem: %v", err, pub.Problems[0])
					}
					return "", err
				}
				return baseSummary + fmt.Sprintf(" — published and pushed %s", pub.Commit), nil
			}
			return baseSummary, nil
		},
	},
}

var loopIntervals = []time.Duration{
	24 * time.Hour,
	12 * time.Hour,
	6 * time.Hour,
	1 * time.Hour,
	30 * time.Minute,
}

// runPage drives the pipeline and shows its log as it happens.
type runPage struct {
	cursor   int
	dry      bool
	push     bool
	loop     bool
	interval time.Duration
	nextRun  time.Time
	running  bool
	current  string
	started  time.Time
	cancel   context.CancelFunc
	elapsed  time.Duration

	// confirming holds the stage awaiting a y/n answer, for anything that
	// reaches outside this machine.
	confirming *stage

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

func newRunPage() *runPage {
	return &runPage{
		interval: 24 * time.Hour,
	}
}

func (r *runPage) Init() tea.Cmd {
	if r.loop || r.running {
		return tickEvery()
	}
	return nil
}

func (r *runPage) Footer() string {
	if r.confirming != nil {
		return styleWarn.Render("push to the remote? ") +
			styleKey.Render("y") + styleFooter.Render(" confirm  ") +
			styleKey.Render("n") + styleFooter.Render(" cancel")
	}
	if r.running {
		return styleKey.Render("ctrl+x") + styleFooter.Render(" cancel  ") +
			styleKey.Render("↑↓") + styleFooter.Render(" scroll log")
	}
	return styleKey.Render("↑↓") + styleFooter.Render(" stage  ") +
		styleKey.Render("enter") + styleFooter.Render(" run  ") +
		styleKey.Render("d") + styleFooter.Render(" dry  ") +
		styleKey.Render("p") + styleFooter.Render(" push  ") +
		styleKey.Render("l") + styleFooter.Render(fmt.Sprintf(" loop (%s)  ", formatInterval(r.interval))) +
		styleKey.Render("L") + styleFooter.Render(" interval  ") +
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
		if r.loop {
			r.nextRun = time.Now().Add(r.interval)
			m.log.Info("scheduled next loop run", "interval", r.interval, "at", r.nextRun.Format(time.TimeOnly))
		}
		// Refresh the counts now that what is on disk has changed.
		return loadStats(m.cfg), false

	case tickMsg:
		if r.running {
			r.elapsed = time.Since(r.started)
		}
		if r.loop && !r.running && !r.nextRun.IsZero() && time.Now().After(r.nextRun) {
			runAllStage := stages[len(stages)-1]
			for _, s := range stages {
				if s.key == "run" {
					runAllStage = s
					break
				}
			}
			m.setStatus("starting scheduled loop run (%s interval)…", formatInterval(r.interval))
			r.nextRun = time.Now().Add(r.interval)
			return r.launch(m, runAllStage), false
		}
		if r.running || r.loop {
			return tickEvery(), false
		}
		return nil, false

	case tea.KeyMsg:
		return r.handleKey(msg, m)
	}
	return nil, false
}

func (r *runPage) handleKey(msg tea.KeyMsg, m *Model) (tea.Cmd, bool) {
	// A pending confirmation swallows everything until it is answered.
	if r.confirming != nil {
		if msg.String() == "y" {
			s := *r.confirming
			r.confirming = nil
			return r.launch(m, s), true
		}
		r.confirming = nil
		m.setStatus("cancelled — nothing was pushed")
		return nil, true
	}

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
			m.setStatus("dry run %s", onOff(r.dry))
		}
		return nil, true

	case "p":
		if !r.running {
			r.push = !r.push
			m.setStatus("push on publish %s", onOff(r.push))
		}
		return nil, true

	case "l":
		r.loop = !r.loop
		if r.loop {
			if !r.running {
				runAllStage := stages[len(stages)-1]
				for _, s := range stages {
					if s.key == "run" {
						runAllStage = s
						break
					}
				}
				r.nextRun = time.Now().Add(r.interval)
				m.setStatus("loop mode ON — running every %s (next in %s)",
					formatInterval(r.interval), formatCountdown(time.Until(r.nextRun)))
				return r.launch(m, runAllStage), true
			}
			r.nextRun = time.Now().Add(r.interval)
			m.setStatus("loop mode ON — scheduled next run in %s", formatInterval(r.interval))
			return tickEvery(), true
		}
		r.nextRun = time.Time{}
		m.setStatus("loop mode OFF")
		return nil, true

	case "L", "i":
		curIdx := 0
		for idx, d := range loopIntervals {
			if r.interval == d {
				curIdx = idx
				break
			}
		}
		r.interval = loopIntervals[(curIdx+1)%len(loopIntervals)]
		if r.loop {
			r.nextRun = time.Now().Add(r.interval)
			m.setStatus("loop interval set to %s (next run in %s)",
				formatInterval(r.interval), formatCountdown(time.Until(r.nextRun)))
		} else {
			m.setStatus("loop interval set to %s", formatInterval(r.interval))
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
		s := stages[r.cursor]

		if s.needsLLM && !m.pipe.HasLLM() {
			m.setError(fmt.Errorf(
				"%s needs an LLM provider — set GEMINI_API_KEY, NVIDIA_NIM_API_KEY, or OPENROUTER_API_KEY",
				s.name))
			return nil, true
		}
		// Pushing is not reversible from here, so it is always asked for.
		if s.confirms && r.push && !r.dry {
			r.confirming = &s
			return nil, true
		}
		return r.launch(m, s), true
	}
	return nil, false
}

// launch starts a stage in the background.
func (r *runPage) launch(m *Model, s stage) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	r.running = true
	r.cancel = cancel
	r.current = s.name
	r.started = time.Now()
	r.elapsed = 0
	r.scroll = 0

	opts := stageOpts{
		Options: pipeline.Options{Dry: r.dry, Debug: true},
		Push:    r.push,
	}

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

func onOff(v bool) string {
	if v {
		return "on"
	}
	return "off"
}

func formatInterval(d time.Duration) string {
	if d >= 24*time.Hour && d%(24*time.Hour) == 0 {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	if d >= time.Hour && d%time.Hour == 0 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	if d >= time.Minute && d%time.Minute == 0 {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return d.String()
}

func formatCountdown(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	d = d.Round(time.Second)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%02dh %02dm %02ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%02dm %02ds", m, s)
	}
	return fmt.Sprintf("%02ds", s)
}

func (r *runPage) View(m *Model, width, height int) string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("stages"))
	if r.dry {
		b.WriteString(styleWarn.Render("   dry — nothing will be written"))
	}
	if r.push {
		b.WriteString(styleError.Render("   push armed"))
	}
	if r.loop {
		countdown := "now"
		if remain := time.Until(r.nextRun); remain > 0 {
			countdown = formatCountdown(remain)
		}
		b.WriteString(styleOK.Render(fmt.Sprintf("   ● loop ON (%s) · next in %s", formatInterval(r.interval), countdown)))
	} else {
		b.WriteString(styleDim.Render(fmt.Sprintf("   ○ loop off (%s)", formatInterval(r.interval))))
	}
	b.WriteString("\n")

	hasLLM := m.pipe.HasLLM()

	for i, s := range stages {
		cursor := "  "
		label := styleValue.Render(pad(s.name, 10))
		if i == r.cursor {
			cursor = styleCursor.Render("▸ ")
			label = styleSelected.Render(pad(s.name, 10))
		}

		note := ""
		switch {
		case r.running && r.current == s.name:
			note = styleWarn.Render(fmt.Sprintf("  running %s", r.elapsed.Round(100*time.Millisecond)))
		case s.needsLLM && !hasLLM:
			note = styleDim.Render("  needs an LLM key")
		case s.confirms && r.push:
			note = styleError.Render("  will push")
		}

		b.WriteString(cursor + label + styleDim.Render(pad(s.about, 48)) + note + "\n")
	}

	if r.confirming != nil {
		b.WriteString("\n" + styleError.Render(
			"  push sends this commit to the remote and cannot be undone from here.") + "\n")
		b.WriteString(styleWarn.Render("  press y to confirm, any other key to cancel") + "\n")
	}

	b.WriteString("\n" + styleHeader.Render("log") + "\n")

	lines := m.sink.Lines()
	logHeight := max(height-len(stages)-6, 3)

	if len(lines) == 0 {
		b.WriteString(styleDim.Render("  no output yet — press enter to run the selected stage"))
		return b.String()
	}

	// Pin to the bottom unless the reader has scrolled up.
	end := max(len(lines)-r.scroll, 1)
	start := max(end-logHeight, 0)

	for _, line := range lines[start:end] {
		b.WriteString("  " + line + "\n")
	}
	if r.scroll > 0 {
		b.WriteString(styleDim.Render(fmt.Sprintf("  … %d newer lines below", r.scroll)))
	}
	return b.String()
}
