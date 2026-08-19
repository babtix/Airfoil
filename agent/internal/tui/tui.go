// Package tui is the interactive terminal interface: run the pipeline, edit
// every configuration file, check provider health, and browse what the agent
// produced — without leaving the terminal.
package tui

import (
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/pipeline"
)

// page is one screen. Each owns its own state and key handling; the root model
// routes to whichever is active.
type page interface {
	// Init returns any command the page needs on first display.
	Init() tea.Cmd
	// Update handles a message. Returning true for handled stops the root from
	// also acting on the key, which is how editors capture typing.
	Update(msg tea.Msg, m *Model) (tea.Cmd, bool)
	// View renders the page body at the given size.
	View(m *Model, width, height int) string
	// Footer returns the key hints for this page.
	Footer() string
}

type viewID int

const (
	viewDashboard viewID = iota
	viewRun
	viewSources
	viewScoring
	viewKeywords
	viewDoctor
	viewBrowse
)

var tabs = []struct {
	id    viewID
	label string
}{
	{viewDashboard, "1 Dashboard"},
	{viewRun, "2 Run"},
	{viewSources, "3 Sources"},
	{viewScoring, "4 Scoring"},
	{viewKeywords, "5 Keywords"},
	{viewDoctor, "6 Doctor"},
	{viewBrowse, "7 Browse"},
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg  *config.Config
	pipe *pipeline.Pipeline
	log  *slog.Logger
	sink *LogSink

	view          viewID
	width, height int
	showHelp      bool
	quitting      bool

	// status is a transient line under the tabs: the result of the last action.
	status    string
	statusErr bool

	pages map[viewID]page
}

// New builds the root model.
func New(cfg *config.Config, log *slog.Logger, sink *LogSink) *Model {
	m := &Model{
		cfg:    cfg,
		pipe:   pipeline.New(cfg, log),
		log:    log,
		sink:   sink,
		width:  100,
		height: 30,
	}

	m.pages = map[viewID]page{
		viewDashboard: newDashboard(),
		viewRun:       newRunPage(),
		viewSources:   newSourcesPage(cfg),
		viewScoring:   newScoringPage(cfg),
		viewKeywords:  newKeywordsPage(cfg),
		viewDoctor:    newDoctorPage(),
		viewBrowse:    newBrowsePage(),
	}
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.pages[m.view].Init(), m.sink.Wait())
}

// setStatus shows a transient result line.
func (m *Model) setStatus(format string, args ...any) {
	m.status, m.statusErr = fmt.Sprintf(format, args...), false
}

func (m *Model) setError(err error) {
	m.status, m.statusErr = err.Error(), true
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case logLineMsg:
		// Keep pulling log lines for as long as the program runs.
		return m, m.sink.Wait()

	case tea.KeyMsg:
		// The active page sees every key first, so a text field can swallow
		// digits and letters that would otherwise switch tabs.
		cmd, handled := m.pages[m.view].Update(msg, m)
		if handled {
			return m, cmd
		}
		if next, cmd2 := m.handleKey(msg); next != nil {
			return next, tea.Batch(cmd, cmd2)
		}
		return m, cmd
	}

	cmd, _ := m.pages[m.view].Update(msg, m)
	return m, cmd
}

// handleKey processes global keys the active page did not claim.
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "q":
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case "?":
		m.showHelp = !m.showHelp
		return m, nil

	case "esc":
		if m.showHelp {
			m.showHelp = false
		}
		return m, nil

	case "tab":
		return m, m.switchTo(viewID((int(m.view) + 1) % len(tabs)))

	case "shift+tab":
		return m, m.switchTo(viewID((int(m.view) - 1 + len(tabs)) % len(tabs)))

	case "1", "2", "3", "4", "5", "6", "7":
		idx := int(msg.String()[0] - '1')
		if idx < len(tabs) {
			return m, m.switchTo(tabs[idx].id)
		}
	}
	return m, nil
}

func (m *Model) switchTo(v viewID) tea.Cmd {
	if m.view == v {
		return nil
	}
	m.view = v
	m.status = ""
	return m.pages[v].Init()
}

func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	if m.showHelp {
		return lipgloss.NewStyle().
			MaxWidth(max(m.width, 1)).
			MaxHeight(max(m.height, 1)).
			Render(m.helpView())
	}

	var b strings.Builder

	b.WriteString(m.headerView())
	b.WriteString("\n")

	// The body gets whatever the chrome does not use.
	bodyHeight := m.height - m.chromeHeight()
	b.WriteString(m.pages[m.view].View(m, m.width, max(bodyHeight, 1)))

	b.WriteString("\n")
	b.WriteString(m.footerView())

	// One guard on the whole frame: no page can overflow the terminal, however
	// it renders. MaxWidth is ANSI-aware, so styling survives the cut — which
	// hand-rolled rune truncation would not.
	return lipgloss.NewStyle().
		MaxWidth(max(m.width, 1)).
		MaxHeight(max(m.height, 1)).
		Render(b.String())
}

// chromeHeight is the number of lines the header, status, and footer occupy.
func (m *Model) chromeHeight() int {
	n := 4 // title, tabs, rule, footer
	if m.status != "" {
		n++
	}
	return n
}

func (m *Model) headerView() string {
	title := logoWordmark()
	sub := styleDim.Render("  ai intelligence pipeline")

	var tabline strings.Builder
	for _, t := range tabs {
		if t.id == m.view {
			tabline.WriteString(styleTabOn.Render(t.label))
		} else {
			tabline.WriteString(styleTab.Render(t.label))
		}
	}

	out := title + sub + "\n" + tabline.String() + "\n" + rule(m.width)
	if m.status != "" {
		style := styleOK
		if m.statusErr {
			style = styleError
		}
		out += "\n" + style.Render(truncate(m.status, m.width))
	}
	return out
}

func (m *Model) footerView() string {
	hints := m.pages[m.view].Footer()
	global := styleFooter.Render("  ·  ") +
		styleKey.Render("tab") + styleFooter.Render(" switch  ") +
		styleKey.Render("?") + styleFooter.Render(" help  ") +
		styleKey.Render("q") + styleFooter.Render(" quit")

	// hints and global carry ANSI styling. Rune-truncating them would cut an
	// escape sequence in half; the frame-level MaxWidth in View clamps safely.
	return rule(m.width) + "\n" + hints + global
}

func (m *Model) helpView() string {
	sections := []struct {
		title string
		keys  [][2]string
	}{
		{"Global", [][2]string{
			{"1 – 7", "jump to a tab"},
			{"tab / shift+tab", "cycle tabs"},
			{"?", "toggle this help"},
			{"q / ctrl+c", "quit"},
		}},
		{"Run", [][2]string{
			{"↑ ↓", "choose a stage"},
			{"enter", "start the selected stage"},
			{"d", "toggle dry run — write nothing"},
			{"p", "arm push, so publish reaches the remote"},
			{"y / n", "answer the push confirmation"},
			{"ctrl+x", "cancel a running stage"},
			{"c", "clear the log"},
		}},
		{"Sources", [][2]string{
			{"↑ ↓", "move"},
			{"space", "enable or disable"},
			{"enter", "edit the selected source"},
			{"n", "new source"},
			{"D", "delete the selected source"},
			{"s", "save to config/sources.json"},
			{"r", "reload from disk, discarding edits"},
		}},
		{"Scoring and Keywords", [][2]string{
			{"↑ ↓", "move between fields"},
			{"enter", "edit the selected field"},
			{"← →", "nudge a number down or up"},
			{"n", "add an entry (Keywords)"},
			{"D", "delete an entry (Keywords)"},
			{"s", "save"},
			{"r", "reload from disk"},
		}},
		{"Doctor", [][2]string{
			{"enter", "run the checks"},
			{"↑ ↓", "scroll results"},
		}},
		{"Browse", [][2]string{
			{"← →", "items, clusters, ranked, stories"},
			{"↑ ↓", "move"},
			{"enter", "open the detail pane"},
			{"R", "reload from disk"},
		}},
		{"Stages", [][2]string{
			{"Ingest", "sources into data/items/"},
			{"Cluster", "embed and group into stories"},
			{"Rank", "score and set the LLM budget"},
			{"Write", "summarize, then emit .md, index, stories"},
			{"Digest", "newsletter, X thread, LinkedIn draft"},
			{"Publish", "validate, prune, commit, optionally push"},
			{"Run all", "ingest through write, in order"},
		}},
	}

	var b strings.Builder
	b.WriteString(logoWordmark() + styleTitle.Render(" — keys"))
	b.WriteString("\n\n")

	for _, s := range sections {
		b.WriteString(styleHeader.Render(s.title))
		b.WriteString("\n")
		for _, k := range s.keys {
			b.WriteString("  " + styleKey.Render(pad(k[0], 18)) + styleLabel.Render(k[1]) + "\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(styleDim.Render("press ? or esc to return"))
	return b.String()
}

// panel renders a titled box whose total width, borders included, is width.
// lipgloss counts padding inside Width but draws the border outside it, so the
// text a caller may fit is width-4.
func panel(title string, body string, width int) string {
	content := styleHeader.Render(title) + "\n" + body
	return stylePanel.Width(max(width-2, 10)).Render(content)
}

// columns lays out two blocks side by side.
func columns(left, right string, leftWidth, gap int) string {
	l := lipgloss.NewStyle().Width(leftWidth).Render(left)
	return lipgloss.JoinHorizontal(lipgloss.Top, l, repeat(" ", gap), right)
}
