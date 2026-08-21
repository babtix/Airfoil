package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/purge"
	"github.com/papitsho/airfoil/internal/store"
	"github.com/papitsho/airfoil/internal/write"

	"github.com/papitsho/airfoil/internal/config"
)

// purge filter mode (which field has focus)
type purgeField int

const (
	pfAge purgeField = iota
	pfTier
	pfScore
	pfQuery
	pfList
	pfFieldCount
)

// purgePage lets the operator filter stories by age/tier/score/query and
// execute a bulk delete, previewing matches before committing.
type purgePage struct {
	cfg *config.Config

	// filter state
	ageDays   int    // 0 = no cutoff, otherwise "older than N days"
	tierFilter string // "" = all, "minor", "notable", "major"
	maxScore   int    // 0 = disabled, otherwise score ≤ maxScore
	query      string // substring match on title/summary/tags
	queryEdit  bool   // true while typing in the query field

	// loaded data
	stories []model.Story
	loaded  bool
	err     error

	// filter result cache
	matching []model.Story
	dirty    bool // true = re-filter needed

	// selection (map slug → selected)
	selected map[string]bool

	// list cursor + scroll
	focus  purgeField
	cursor int
	scroll int

	// confirmation state: 0=idle 1=armed 2=executing 3=done
	confirmState int
	lastMsg      string
	lastErr      bool

	// toggle display
	showSelected bool
}

// Allowed age presets in days (0 = all time)
var agePresets = []int{0, 7, 14, 30, 60, 90, 180, 365}

func newPurgeTUIPage(cfg *config.Config) *purgePage {
	return &purgePage{
		cfg:     cfg,
		ageDays: 90,
		selected: make(map[string]bool),
		focus:   pfAge,
		dirty:   true,
	}
}

// ── Messages ────────────────────────────────────────────────────────────────

type purgeLoadMsg struct {
	stories []model.Story
	err     error
}

type purgeDoneMsg struct {
	deleted int
	mdFiles int
	err     error
}

// ── page interface ───────────────────────────────────────────────────────────

func (p *purgePage) Init() tea.Cmd {
	if p.cfg != nil && !p.loaded {
		return loadPurgeStories(p.cfg)
	}
	return nil
}

func (p *purgePage) Footer() string {
	if p.queryEdit {
		return styleKey.Render("type") + styleFooter.Render(" to filter  ") +
			styleKey.Render("enter/esc") + styleFooter.Render(" done  ") +
			styleKey.Render("backspace") + styleFooter.Render(" delete")
	}
	switch p.focus {
	case pfList:
		return styleKey.Render("↑↓") + styleFooter.Render(" move  ") +
			styleKey.Render("space") + styleFooter.Render(" toggle  ") +
			styleKey.Render("a") + styleFooter.Render(" all  ") +
			styleKey.Render("n") + styleFooter.Render(" none  ") +
			styleKey.Render("esc") + styleFooter.Render(" filters  ") +
			styleKey.Render("enter") + styleFooter.Render(" execute")
	default:
		return styleKey.Render("←→") + styleFooter.Render(" adjust  ") +
			styleKey.Render("[]") + styleFooter.Render(" field  ") +
			styleKey.Render("↓/enter") + styleFooter.Render(" list  ") +
			styleKey.Render("r") + styleFooter.Render(" reload")
	}
}

func (p *purgePage) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	switch msg := msg.(type) {

	case purgeLoadMsg:
		p.stories, p.err, p.loaded = msg.stories, msg.err, true
		p.dirty = true
		p.refilter()
		p.initSelected()
		return nil, false

	case purgeDoneMsg:
		p.confirmState = 3
		if msg.err != nil {
			p.lastMsg = msg.err.Error()
			p.lastErr = true
		} else {
			p.lastMsg = fmt.Sprintf("✓ Purged %d stories · %d .md files deleted", msg.deleted, msg.mdFiles)
			p.lastErr = false
		}
	case tea.MouseMsg:
		return p.handleMouse(msg, m)

	case tea.KeyMsg:
		return p.handleKey(msg, m)
	}
	return nil, false
}

func (p *purgePage) handleMouse(msg tea.MouseMsg, m *Model) (tea.Cmd, bool) {
	// Mouse wheel scrolling
	if msg.Button == tea.MouseButtonWheelUp || msg.Type == tea.MouseWheelUp {
		list := p.visibleList()
		if len(list) > 0 {
			if p.cursor > 0 {
				p.cursor--
			}
			if p.cursor < p.scroll {
				p.scroll = p.cursor
			}
			return nil, true
		}
	}
	if msg.Button == tea.MouseButtonWheelDown || msg.Type == tea.MouseWheelDown {
		list := p.visibleList()
		if len(list) > 0 {
			if p.cursor < len(list)-1 {
				p.cursor++
			}
			listHeight := max(m.height-10, 6)
			if p.cursor >= p.scroll+listHeight-2 {
				p.scroll++
			}
			return nil, true
		}
	}

	// Left click
	if (msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress) || msg.Type == tea.MouseLeft {
		// Confirmation popup mode click
		if p.confirmState == 1 {
			if msg.Y >= 6 && msg.Y <= 9 {
				p.confirmState = 2
				return p.execPurge(m), true
			}
			if msg.Y >= 10 {
				p.confirmState = 0
				return nil, true
			}
			return nil, true
		}

		headerHeight := 4
		if m.status != "" {
			headerHeight = 5
		}
		pageLineY := msg.Y - headerHeight

		// Top filter boxes (pageLineY 0 to 2)
		if pageLineY >= 0 && pageLineY <= 2 {
			fieldWidth := max((m.width-4)/4, 14)
			box0End := fieldWidth + 2
			box1Start := fieldWidth + 4
			box1End := box1Start + fieldWidth + 2
			box2Start := (fieldWidth + 4) * 2
			box2End := box2Start + fieldWidth + 2
			box3Start := (fieldWidth + 4) * 3

			if msg.X < box0End {
				p.focus = pfAge
				p.stepAge(+1)
				p.dirty = true
				p.refilter()
				p.initSelected()
				return nil, true
			} else if msg.X >= box1Start && msg.X < box1End {
				p.focus = pfTier
				p.stepTier(+1)
				p.dirty = true
				p.refilter()
				p.initSelected()
				return nil, true
			} else if msg.X >= box2Start && msg.X < box2End {
				p.focus = pfScore
				if p.maxScore == 0 {
					p.maxScore = 20
				} else {
					p.maxScore = (p.maxScore + 10) % 100
				}
				p.dirty = true
				p.refilter()
				p.initSelected()
				return nil, true
			} else if msg.X >= box3Start {
				p.focus = pfQuery
				p.queryEdit = true
				return nil, true
			}
		}

		// Stats bar (pageLineY == 4)
		if pageLineY == 4 {
			if msg.X >= m.width-16 {
				// Clicked [EXECUTE]
				count := p.selectedCount()
				if count > 0 {
					p.confirmState = 1
				}
				return nil, true
			}
		}

		// Story list rows
		listRowsStartY := 7
		if p.lastMsg != "" {
			listRowsStartY += 2
		}

		if pageLineY >= listRowsStartY {
			rowOffset := pageLineY - listRowsStartY
			list := p.visibleList()
			targetIndex := p.scroll + rowOffset
			if targetIndex >= 0 && targetIndex < len(list) {
				p.focus = pfList
				p.cursor = targetIndex
				slug := list[targetIndex].Slug
				p.selected[slug] = !p.selected[slug]
				return nil, true
			}
		}
	}

	return nil, false
}

func (p *purgePage) handleKey(msg tea.KeyMsg, m *Model) (tea.Cmd, bool) {
	// Query edit mode: capture all typing
	if p.queryEdit {
		switch msg.String() {
		case "enter", "esc":
			p.queryEdit = false
			p.dirty = true
			p.refilter()
			p.initSelected()
			return nil, true
		case "backspace", "ctrl+h":
			if len(p.query) > 0 {
				p.query = p.query[:len(p.query)-1]
				p.dirty = true
			}
			return nil, true
		default:
			// Accept printable characters
			if len(msg.Runes) > 0 {
				p.query += string(msg.Runes)
				p.dirty = true
			}
			return nil, true
		}
	}

	// Confirmation prompt in-flight
	if p.confirmState == 1 {
		switch msg.String() {
		case "y", "Y", "enter":
			p.confirmState = 2
			return p.execPurge(m), true
		case "n", "N", "esc", "q":
			p.confirmState = 0
			return nil, true
		}
		return nil, true
	}

	switch msg.String() {
	case "r", "R":
		p.loaded = false
		p.confirmState = 0
		p.lastMsg = ""
		return loadPurgeStories(p.cfg), true

	case "[":
		// Previous filter field
		if p.focus == pfList {
			p.focus = pfQuery
		} else if p.focus > 0 {
			p.focus--
		} else {
			p.focus = pfQuery
		}
		return nil, true

	case "]":
		// Next filter field
		if p.focus < pfQuery {
			p.focus++
		} else {
			p.focus = pfAge
		}
		return nil, true
	}

	// Field-specific keys
	switch p.focus {
	case pfAge:
		switch msg.String() {
		case "left", "h":
			p.stepAge(-1)
			p.dirty = true
			p.refilter()
			p.initSelected()
			return nil, true
		case "right", "l":
			p.stepAge(+1)
			p.dirty = true
			p.refilter()
			p.initSelected()
			return nil, true
		case "down", "j", "enter":
			p.focus = pfList
			p.cursor = 0
			return nil, true
		}

	case pfTier:
		switch msg.String() {
		case "left", "h":
			p.stepTier(-1)
			p.dirty = true
			p.refilter()
			p.initSelected()
			return nil, true
		case "right", "l":
			p.stepTier(+1)
			p.dirty = true
			p.refilter()
			p.initSelected()
			return nil, true
		case "down", "j", "enter":
			p.focus = pfList
			p.cursor = 0
			return nil, true
		}

	case pfScore:
		switch msg.String() {
		case "left", "h":
			if p.maxScore > 0 {
				p.maxScore = max(0, p.maxScore-5)
			}
			p.dirty = true
			p.refilter()
			p.initSelected()
			return nil, true
		case "right", "l":
			p.maxScore = min(p.maxScore+5, 100)
			p.dirty = true
			p.refilter()
			p.initSelected()
			return nil, true
		case "enter":
			if p.maxScore == 0 {
				p.maxScore = 20
			} else {
				p.maxScore = 0
			}
			p.dirty = true
			p.refilter()
			p.initSelected()
			return nil, true
		case "down", "j":
			p.focus = pfList
			p.cursor = 0
			return nil, true
		}

	case pfQuery:
		switch msg.String() {
		case "enter":
			p.queryEdit = true
			return nil, true
		case "backspace":
			if len(p.query) > 0 {
				p.query = ""
				p.dirty = true
				p.refilter()
				p.initSelected()
			}
			return nil, true
		case "down", "j":
			p.focus = pfList
			p.cursor = 0
			return nil, true
		}

	case pfList:
		return p.handleListKey(msg, m)
	}

	return nil, false
}

func (p *purgePage) stepAge(delta int) {
	idx := 0
	for i, v := range agePresets {
		if v == p.ageDays {
			idx = i
			break
		}
	}
	idx = max(0, min(idx+delta, len(agePresets)-1))
	p.ageDays = agePresets[idx]
}

func (p *purgePage) stepTier(delta int) {
	tiers := []string{"", "minor", "notable", "major"}
	idx := 0
	for i, t := range tiers {
		if t == p.tierFilter {
			idx = i
			break
		}
	}
	idx = (idx + delta + len(tiers)) % len(tiers)
	p.tierFilter = tiers[idx]
}

func (p *purgePage) handleListKey(msg tea.KeyMsg, m *Model) (tea.Cmd, bool) {
	list := p.visibleList()
	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		} else {
			p.focus = pfAge
		}
		return nil, true
	case "down", "j":
		if p.cursor < len(list)-1 {
			p.cursor++
		}
		return nil, true
	case "space":
		if p.cursor < len(list) {
			slug := list[p.cursor].Slug
			p.selected[slug] = !p.selected[slug]
		}
		return nil, true
	case "a":
		// Select all matching
		for _, s := range p.matching {
			p.selected[s.Slug] = true
		}
		return nil, true
	case "n":
		// Deselect all
		p.selected = make(map[string]bool)
		return nil, true
	case "i":
		// Invert selection
		for _, s := range p.matching {
			p.selected[s.Slug] = !p.selected[s.Slug]
		}
		return nil, true
	case "s":
		p.showSelected = !p.showSelected
		return nil, true
	case "enter":
		// Arm confirmation
		count := p.selectedCount()
		if count == 0 {
			m.setError(fmt.Errorf("no stories selected"))
			return nil, true
		}
		p.confirmState = 1
		return nil, true
	case "esc":
		p.focus = pfAge
		return nil, true
	}
	return nil, false
}

// ── Business logic ───────────────────────────────────────────────────────────

func (p *purgePage) refilter() {
	if !p.dirty {
		return
	}
	opts := p.buildOpts()
	result := purge.Filter(p.stories, opts)
	p.matching = result.Removed
	p.dirty = false
}

func (p *purgePage) initSelected() {
	// On filter change, set all newly matching to selected=true by default
	// but preserve any explicit deselection that was already in the map
	for _, s := range p.matching {
		if _, exists := p.selected[s.Slug]; !exists {
			p.selected[s.Slug] = true
		}
	}
}

func (p *purgePage) buildOpts() purge.Options {
	var before time.Time
	if p.ageDays > 0 {
		before = time.Now().AddDate(0, 0, -p.ageDays)
	}
	var maxPtr *int
	if p.maxScore > 0 {
		v := p.maxScore
		maxPtr = &v
	}
	return purge.Options{
		Before:   before,
		Tier:     p.tierFilter,
		MaxScore: maxPtr,
		Query:    p.query,
	}
}

func (p *purgePage) selectedCount() int {
	n := 0
	for _, s := range p.matching {
		if p.selected[s.Slug] {
			n++
		}
	}
	return n
}

func (p *purgePage) visibleList() []model.Story {
	if p.showSelected {
		var out []model.Story
		for _, s := range p.matching {
			if p.selected[s.Slug] {
				out = append(out, s)
			}
		}
		return out
	}
	return p.matching
}

func (p *purgePage) execPurge(m *Model) tea.Cmd {
	// Collect slugs to remove
	slugs := make([]string, 0, p.selectedCount())
	for _, s := range p.matching {
		if p.selected[s.Slug] {
			slugs = append(slugs, s.Slug)
		}
	}
	cfg := p.cfg

	return func() tea.Msg {
		// Load fresh copy right before writing
		stories, err := store.ReadJSONOr(cfg.StoriesPath(), []model.Story(nil))
		if err != nil {
			return purgeDoneMsg{err: fmt.Errorf("read stories: %w", err)}
		}

		slugSet := make(map[string]bool, len(slugs))
		for _, sl := range slugs {
			slugSet[sl] = true
		}

		filterOpts := purge.Options{Slugs: slugs}
		result := purge.Filter(stories, filterOpts)

		if err := store.WriteJSON(cfg.StoriesPath(), result.Kept); err != nil {
			return purgeDoneMsg{err: fmt.Errorf("write stories: %w", err)}
		}

		idx := write.BuildIndex(result.Kept, time.Now())
		if err := store.WriteJSON(cfg.IndexPath(), idx); err != nil {
			return purgeDoneMsg{err: fmt.Errorf("write index: %w", err)}
		}

		mdCount := 0
		for slug := range slugSet {
			mdPath := filepath.Join(cfg.StoriesDir, slug+".md")
			if store.Exists(mdPath) {
				if e := os.Remove(mdPath); e == nil {
					mdCount++
				}
			}
		}

		return purgeDoneMsg{deleted: len(result.Removed), mdFiles: mdCount}
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (p *purgePage) View(m *Model, width, height int) string {
	if !p.loaded {
		return styleDim.Render("  Loading stories…")
	}
	if p.err != nil {
		return styleError.Render("  Error: " + p.err.Error())
	}

	// Recompute filter if dirty (shouldn't normally be needed here, but safety)
	p.refilter()

	// Confirmation prompt overlay
	if p.confirmState == 1 {
		return p.confirmView(width, height)
	}

	var b strings.Builder

	// ── Top Controls strip ──────────────────────────────────────────────────
	b.WriteString(p.controlsView(width))
	b.WriteString("\n\n")

	// ── Stats bar ──────────────────────────────────────────────────────────
	b.WriteString(p.statsView())
	b.WriteString("\n\n")

	// ── Status / confirmation done line ───────────────────────────────────
	if p.lastMsg != "" {
		style := styleOK
		if p.lastErr {
			style = styleError
		}
		b.WriteString(style.Render("  "+p.lastMsg) + "\n\n")
	}

	// ── List ───────────────────────────────────────────────────────────────
	listHeight := height - 10 // rough chrome height above the list
	b.WriteString(p.listView(width, max(listHeight, 6)))

	return b.String()
}

func (p *purgePage) controlsView(width int) string {
	// Build filter row: [AGE] [TIER] [SCORE] [QUERY]
	fieldWidth := max((width-4)/4, 14)

	age := p.renderField(pfAge, "AGE", p.ageLabel(), fieldWidth)
	tier := p.renderField(pfTier, "TIER", p.tierLabel(), fieldWidth)
	score := p.renderField(pfScore, "SCORE", p.scoreLabel(), fieldWidth)
	query := p.renderField(pfQuery, "QUERY", p.queryLabel(), fieldWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, age, "  ", tier, "  ", score, "  ", query)
}

func (p *purgePage) renderField(id purgeField, label, value string, width int) string {
	active := p.focus == id
	labelStyle := styleLabel
	valStyle := styleValue
	border := lipgloss.NormalBorder()
	borderColor := colLine
	if active {
		labelStyle = styleCursor
		valStyle = styleSelected
		borderColor = colAccent
		border = lipgloss.RoundedBorder()
	}
	content := labelStyle.Render(label) + "\n" + valStyle.Render(value)
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(borderColor).
		Padding(0, 1).
		Width(width).
		Render(content)
}

func (p *purgePage) ageLabel() string {
	if p.ageDays == 0 {
		return "ALL TIME"
	}
	return fmt.Sprintf("> %d days old", p.ageDays)
}

func (p *purgePage) tierLabel() string {
	if p.tierFilter == "" {
		return "ALL TIERS"
	}
	return strings.ToUpper(p.tierFilter)
}

func (p *purgePage) scoreLabel() string {
	if p.maxScore == 0 {
		return "disabled"
	}
	return fmt.Sprintf("score ≤ %d", p.maxScore)
}

func (p *purgePage) queryLabel() string {
	if p.queryEdit {
		q := p.query
		if q == "" {
			q = " "
		}
		return styleError.Render(q) + styleCursor.Render("█")
	}
	if p.query == "" {
		return styleDim.Render("(none)")
	}
	return truncate(p.query, 16)
}

func (p *purgePage) statsView() string {
	total := len(p.stories)
	matched := len(p.matching)
	selected := p.selectedCount()

	totalStr := styleDim.Render(fmt.Sprintf("total %d", total))
	matchStr := styleWarn.Render(fmt.Sprintf("matched %d", matched))
	selStr := styleError.Render(fmt.Sprintf("selected %d", selected))
	keepStr := styleOK.Render(fmt.Sprintf("keeping %d", total-selected))

	actions := "  " + styleKey.Render("[a]") + styleDim.Render("all ") +
		styleKey.Render("[n]") + styleDim.Render("none ") +
		styleKey.Render("[i]") + styleDim.Render("invert ") +
		styleKey.Render("[s]") + styleDim.Render("view")

	if selected > 0 {
		actions += "  " + styleError.Render("[ENTER / CLICK TO PURGE]")
	}

	return "  " + totalStr + "  " + matchStr + "  " + selStr + "  " + keepStr + "  │" + actions
}

func (p *purgePage) listView(width, height int) string {
	list := p.visibleList()

	if len(list) == 0 {
		hint := "adjust the filters above or press r to reload"
		if p.showSelected {
			hint = "no stories selected — press a to select all matching"
		}
		return styleDim.Render("  " + hint)
	}

	// Clamp cursor
	if p.cursor >= len(list) {
		p.cursor = max(0, len(list)-1)
	}

	// Scroll
	if p.cursor < p.scroll {
		p.scroll = p.cursor
	}
	if p.cursor >= p.scroll+height {
		p.scroll = p.cursor - height + 1
	}

	var b strings.Builder

	// Header row
	hdr := pad("SEL", 5) + pad("DATE", 12) + pad("AGE", 8) +
		pad("SCORE", 7) + pad("TIER", 10) + "TITLE"
	if p.focus == pfList {
		b.WriteString(styleSelected.Render("  "+hdr) + "\n")
	} else {
		b.WriteString(styleDim.Render("  "+hdr) + "\n")
	}
	b.WriteString(styleRule.Render(repeat("─", max(width-2, 20))) + "\n")

	end := min(p.scroll+height-2, len(list))
	for i := p.scroll; i < end; i++ {
		s := list[i]
		isSel := p.selected[s.Slug]
		isCursor := (i == p.cursor) && p.focus == pfList
		ageDays := int(time.Since(s.Date).Hours() / 24)

		selMark := "  ○  "
		if isSel {
			selMark = styleError.Render("  ●  ")
		}

		dateStr := s.Date.Format("2006-01-02")
		ageStr := fmt.Sprintf("%dd", ageDays)
		scoreStr := fmt.Sprintf("%4d", s.Score)
		tierStr := pad(s.Tier, 9)

		row := selMark +
			pad(dateStr, 12) +
			pad(ageStr, 8) +
			styleValue.Render(pad(scoreStr, 7)) +
			tierStyle(tierToNum(s.Tier)).Render(tierStr) +
			truncate(s.Title, max(width-52, 20))

		if isCursor {
			b.WriteString(styleCursor.Render("▸") + styleSelected.Render(row) + "\n")
		} else if isSel {
			b.WriteString(" " + styleError.Render(row) + "\n")
		} else {
			b.WriteString(" " + row + "\n")
		}
	}

	// Scroll indicator
	if len(list) > height-2 {
		b.WriteString(styleDim.Render(fmt.Sprintf(
			"  … %d / %d", min(p.scroll+height-2, len(list)), len(list))))
	}

	return b.String()
}

func (p *purgePage) confirmView(width, _ int) string {
	count := p.selectedCount()
	keep := len(p.stories) - count

	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(styleError.Render(center("  ⚠  CONFIRM BULK DELETE  ⚠  ", width)) + "\n\n")
	b.WriteString(styleValue.Render(fmt.Sprintf(
		"  This will permanently delete %d stories (%d remaining).", count, keep)) + "\n")
	b.WriteString(styleValue.Render(
		"  Also removes matching .md files from site/src/content/stories/.\n"))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  This cannot be undone outside of git.\n\n"))
	b.WriteString(styleKey.Render("  y / enter") + styleFooter.Render("  confirm and delete") + "\n")
	b.WriteString(styleKey.Render("  n / esc") + styleFooter.Render("  cancel") + "\n")
	return b.String()
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func loadPurgeStories(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		stories, err := store.ReadJSONOr(cfg.StoriesPath(), []model.Story(nil))
		return purgeLoadMsg{stories: stories, err: err}
	}
}

// tierToNum maps a tier name to an approximate tier number for styling.
func tierToNum(tier string) int {
	switch tier {
	case model.TierMajor:
		return 1
	case model.TierNotable:
		return 3
	default:
		return 5
	}
}
