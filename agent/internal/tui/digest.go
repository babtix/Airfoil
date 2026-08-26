package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/digest"
)

type digestFormat int

const (
	formatNewsletter digestFormat = iota
	formatLinkedIn
	formatThread
	formatHTML
	formatCount
)

var formatLabels = [formatCount]string{
	"1 Newsletter (.txt)",
	"2 LinkedIn (.md)",
	"3 X Thread",
	"4 HTML Email (.html)",
}

type digestDayData struct {
	Date       string
	Newsletter string
	LinkedIn   string
	HTML       string
	ThreadRaw  string
	Thread     digest.Thread
}

type digestPage struct {
	cfg        *config.Config
	days       []string
	dayIdx     int
	format     digestFormat
	data       map[string]*digestDayData
	scroll     int
	postCursor int
	copiedMsg  string
	copiedAt   time.Time
	loaded     bool
	err        error
}

type digestDataMsg struct {
	days []string
	data map[string]*digestDayData
	err  error
}

func newDigestPage(cfg *config.Config) *digestPage {
	return &digestPage{
		cfg:  cfg,
		data: make(map[string]*digestDayData),
	}
}

func (p *digestPage) Init() tea.Cmd {
	p.loaded = true
	if p.cfg != nil {
		return loadDigestData(p.cfg)
	}
	return nil
}

func (p *digestPage) Footer() string {
	if p.format == formatThread {
		return styleKey.Render("←→ / 1-4") + styleFooter.Render(" format  ") +
			styleKey.Render("↑↓") + styleFooter.Render(" post  ") +
			styleKey.Render("y / c") + styleFooter.Render(" copy post  ") +
			styleKey.Render("Y / C") + styleFooter.Render(" copy thread  ") +
			styleKey.Render("[]") + styleFooter.Render(" date  ") +
			styleKey.Render("r") + styleFooter.Render(" reload")
	}
	return styleKey.Render("←→ / 1-4") + styleFooter.Render(" format  ") +
		styleKey.Render("↑↓") + styleFooter.Render(" scroll  ") +
		styleKey.Render("y / c") + styleFooter.Render(" copy to clipboard  ") +
		styleKey.Render("[]") + styleFooter.Render(" date  ") +
		styleKey.Render("r") + styleFooter.Render(" reload")
}

func (p *digestPage) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case digestDataMsg:
		p.days = msg.days
		p.data = msg.data
		p.err = msg.err
		p.loaded = true
		if p.dayIdx >= len(p.days) {
			p.dayIdx = 0
		}
		return nil, false

	case tea.KeyMsg:
		switch msg.String() {
		case "1":
			p.format = formatNewsletter
			p.scroll = 0
			return nil, true
		case "2":
			p.format = formatLinkedIn
			p.scroll = 0
			return nil, true
		case "3":
			p.format = formatThread
			p.scroll = 0
			p.postCursor = 0
			return nil, true
		case "4":
			p.format = formatHTML
			p.scroll = 0
			return nil, true

		case "left", "h":
			p.format = (p.format - 1 + formatCount) % formatCount
			p.scroll = 0
			return nil, true

		case "right", "l":
			p.format = (p.format + 1) % formatCount
			p.scroll = 0
			return nil, true

		case "[", "p":
			// Older date
			if p.dayIdx < len(p.days)-1 {
				p.dayIdx++
				p.scroll = 0
				p.postCursor = 0
				m.setStatus("viewing digest for %s", p.days[p.dayIdx])
			}
			return nil, true

		case "]", "n":
			// Newer date
			if p.dayIdx > 0 {
				p.dayIdx--
				p.scroll = 0
				p.postCursor = 0
				m.setStatus("viewing digest for %s", p.days[p.dayIdx])
			}
			return nil, true

		case "up", "k":
			if p.format == formatThread {
				if p.postCursor > 0 {
					p.postCursor--
				}
				return nil, true
			}
			if p.scroll > 0 {
				p.scroll--
			}
			return nil, true

		case "down", "j":
			if p.format == formatThread {
				cur := p.current()
				if cur != nil && p.postCursor < len(cur.Thread.Posts)-1 {
					p.postCursor++
				}
				return nil, true
			}
			p.scroll++
			return nil, true

		case "pgup", "ctrl+u", "ctrl+b":
			if p.scroll > 10 {
				p.scroll -= 10
			} else {
				p.scroll = 0
			}
			return nil, true

		case "pgdown", "ctrl+d", "ctrl+f", " ":
			p.scroll += 10
			return nil, true

		case "g", "home":
			p.scroll = 0
			p.postCursor = 0
			return nil, true

		case "G", "end":
			lines := p.currentLines(m.width)
			p.scroll = max(len(lines)-10, 0)
			return nil, true

		case "y", "c", "enter":
			return p.copyContent(m, false)

		case "Y", "C":
			return p.copyContent(m, true)

		case "r", "R":
			p.loaded = false
			m.setStatus("reloading digest from disk…")
			return loadDigestData(m.cfg), true
		}
	}

	if !p.loaded {
		p.loaded = true
		return loadDigestData(m.cfg), false
	}
	return nil, false
}

func (p *digestPage) current() *digestDayData {
	if len(p.days) == 0 || p.dayIdx < 0 || p.dayIdx >= len(p.days) {
		return nil
	}
	return p.data[p.days[p.dayIdx]]
}

func (p *digestPage) copyContent(m *Model, forceAllThread bool) (tea.Cmd, bool) {
	cur := p.current()
	if cur == nil {
		m.setError(fmt.Errorf("no digest content to copy"))
		return nil, true
	}

	var toCopy string
	var label string

	switch p.format {
	case formatNewsletter:
		toCopy = cur.Newsletter
		label = fmt.Sprintf("newsletter text (%d chars)", len(toCopy))
	case formatLinkedIn:
		toCopy = cur.LinkedIn
		label = fmt.Sprintf("LinkedIn draft (%d chars)", len(toCopy))
	case formatHTML:
		toCopy = cur.HTML
		label = fmt.Sprintf("HTML email (%d chars)", len(toCopy))
	case formatThread:
		if forceAllThread || len(cur.Thread.Posts) == 0 {
			var b strings.Builder
			for i, post := range cur.Thread.Posts {
				if i > 0 {
					b.WriteString("\n\n")
				}
				b.WriteString(post.Text)
				if post.URL != "" {
					b.WriteString(" " + post.URL)
				}
			}
			toCopy = b.String()
			label = fmt.Sprintf("full X thread (%d posts, %d chars)", len(cur.Thread.Posts), len(toCopy))
		} else {
			if p.postCursor >= 0 && p.postCursor < len(cur.Thread.Posts) {
				post := cur.Thread.Posts[p.postCursor]
				toCopy = post.Text
				if post.URL != "" {
					toCopy += " " + post.URL
				}
				label = fmt.Sprintf("X post %d/%d (%d chars)",
					p.postCursor+1, len(cur.Thread.Posts), len(toCopy))
			} else {
				toCopy = cur.ThreadRaw
				label = fmt.Sprintf("X thread raw JSON (%d chars)", len(toCopy))
			}
		}
	}

	if toCopy == "" {
		m.setError(fmt.Errorf("selected format is empty"))
		return nil, true
	}

	if err := clipboard.WriteAll(toCopy); err != nil {
		m.setError(fmt.Errorf("copy to clipboard failed: %w", err))
		return nil, true
	}

	p.copiedMsg = fmt.Sprintf("✓ Copied %s to clipboard — ready to paste", label)
	p.copiedAt = time.Now()
	m.setStatus("✓ copied %s to clipboard", label)
	return nil, true
}

func loadDigestData(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		dir := cfg.DigestDir()
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return digestDataMsg{days: nil, data: make(map[string]*digestDayData)}
			}
			return digestDataMsg{err: err}
		}

		daySet := make(map[string]bool)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if len(name) >= 10 {
				datePrefix := name[:10]
				if _, err := time.Parse(time.DateOnly, datePrefix); err == nil {
					daySet[datePrefix] = true
				}
			}
		}

		var days []string
		for day := range daySet {
			days = append(days, day)
		}
		sort.Slice(days, func(i, j int) bool {
			return days[i] > days[j] // descending order: newest first
		})

		dataMap := make(map[string]*digestDayData, len(days))
		for _, day := range days {
			d := &digestDayData{Date: day}

			// Read newsletter.txt
			txtPath := filepath.Join(dir, day+"-newsletter.txt")
			if txtBytes, err := os.ReadFile(txtPath); err == nil {
				d.Newsletter = string(txtBytes)
			}

			// Read linkedin.md
			liPath := filepath.Join(dir, day+"-linkedin.md")
			if liBytes, err := os.ReadFile(liPath); err == nil {
				d.LinkedIn = string(liBytes)
			}

			// Read newsletter.html
			htmlPath := filepath.Join(dir, day+"-newsletter.html")
			if htmlBytes, err := os.ReadFile(htmlPath); err == nil {
				d.HTML = string(htmlBytes)
			}

			// Read x.json
			xPath := filepath.Join(dir, day+"-x.json")
			if xBytes, err := os.ReadFile(xPath); err == nil {
				d.ThreadRaw = string(xBytes)
				var th digest.Thread
				if err := json.Unmarshal(xBytes, &th); err == nil {
					d.Thread = th
				}
			}

			dataMap[day] = d
		}

		return digestDataMsg{
			days: days,
			data: dataMap,
		}
	}
}

func (p *digestPage) currentLines(width int) []string {
	cur := p.current()
	if cur == nil {
		return nil
	}

	var raw string
	switch p.format {
	case formatNewsletter:
		raw = cur.Newsletter
	case formatLinkedIn:
		raw = cur.LinkedIn
	case formatHTML:
		raw = cur.HTML
	case formatThread:
		raw = cur.ThreadRaw
	}

	if raw == "" {
		return nil
	}

	lines := strings.Split(raw, "\n")
	return lines
}

func (p *digestPage) View(m *Model, width, height int) string {
	var b strings.Builder

	// 1. Top navigation bar: Formats + Date selector + Copied indicator
	for i, label := range formatLabels {
		if p.format == digestFormat(i) {
			b.WriteString(styleSelected.Render(" " + label + " "))
		} else {
			b.WriteString(styleDim.Render(" " + label + " "))
		}
		b.WriteString(" ")
	}

	if len(p.days) > 0 {
		dateStr := p.days[p.dayIdx]
		dateNav := fmt.Sprintf("   ◄ [%s] (%d/%d) ►", dateStr, p.dayIdx+1, len(p.days))
		b.WriteString(styleKey.Render(dateNav))
	}

	b.WriteString("\n")

	if time.Since(p.copiedAt) < 4*time.Second && p.copiedMsg != "" {
		b.WriteString(styleOK.Render("  " + p.copiedMsg) + "\n")
	} else {
		b.WriteString(styleDim.Render("  Press 'y' or 'c' to copy selected output directly to system clipboard") + "\n")
	}
	b.WriteString("\n")

	if p.err != nil {
		b.WriteString(styleError.Render("  " + p.err.Error()))
		return b.String()
	}

	if len(p.days) == 0 {
		b.WriteString(styleDim.Render("  no digest outputs found in data/digest/\n\n"))
		b.WriteString(styleLabel.Render("  To generate today's digest:\n"))
		b.WriteString(styleValue.Render("    1. Switch to tab 2 Run and execute the 'Digest' stage (or Run all)\n"))
		b.WriteString(styleValue.Render("    2. Or run from terminal: airfoil digest\n\n"))
		b.WriteString(styleDim.Render("  Press 'r' to reload once generated."))
		return b.String()
	}

	cur := p.current()
	if cur == nil {
		b.WriteString(styleDim.Render("  no data for selected day — press 'r' to reload"))
		return b.String()
	}

	contentHeight := max(height-6, 5)

	if p.format == formatThread {
		return b.String() + p.renderThreadView(cur, width, contentHeight)
	}

	return b.String() + p.renderTextView(width, contentHeight)
}

func (p *digestPage) renderTextView(width, height int) string {
	var b strings.Builder
	lines := p.currentLines(width)

	if len(lines) == 0 {
		b.WriteString(styleDim.Render("  (empty file for this format — run digest to generate)"))
		return b.String()
	}

	// Clamp scroll
	maxScroll := max(len(lines)-height, 0)
	if p.scroll > maxScroll {
		p.scroll = maxScroll
	}

	start := p.scroll
	end := min(start+height, len(lines))

	for i := start; i < end; i++ {
		line := lines[i]
		if strings.HasPrefix(line, "AIRFOIL —") || strings.HasPrefix(line, "# ") {
			b.WriteString(styleHeader.Render(line) + "\n")
		} else if strings.HasPrefix(line, "===") || strings.HasPrefix(line, "---") {
			b.WriteString(styleDim.Render(line) + "\n")
		} else if strings.HasPrefix(line, "1.") || strings.HasPrefix(line, "2.") || strings.HasPrefix(line, "3.") ||
			strings.HasPrefix(line, "4.") || strings.HasPrefix(line, "5.") {
			b.WriteString(styleKey.Render(line) + "\n")
		} else {
			b.WriteString(styleValue.Render(line) + "\n")
		}
	}

	if maxScroll > 0 {
		pct := int(float64(start) / float64(maxScroll) * 100)
		scrollInfo := fmt.Sprintf("  [%d/%d lines · %d%%] (use ↑↓ to scroll)", end, len(lines), pct)
		b.WriteString("\n" + styleDim.Render(scrollInfo))
	}

	return b.String()
}

func (p *digestPage) renderThreadView(cur *digestDayData, width, height int) string {
	var b strings.Builder

	posts := cur.Thread.Posts
	if len(posts) == 0 {
		b.WriteString(styleDim.Render("  (no thread posts parsed from x.json)"))
		return b.String()
	}

	if p.postCursor >= len(posts) {
		p.postCursor = len(posts) - 1
	}
	if p.postCursor < 0 {
		p.postCursor = 0
	}

	b.WriteString(styleLabel.Render(fmt.Sprintf("  X Thread (%d posts) — Navigate with ↑↓ / j k · Copy active post with 'y'/'c' · Copy full thread with 'Y'/'C'\n\n", len(posts))))

	cardWidth := max(min(width-6, 96), 40)

	// Display posts around the cursor
	start := max(p.postCursor-2, 0)
	end := min(start+4, len(posts))
	if end-start < 4 && start > 0 {
		start = max(end-4, 0)
	}

	for i := start; i < end; i++ {
		post := posts[i]
		isActive := (i == p.postCursor)

		charCount := len(post.Text)
		if post.URL != "" {
			charCount += 1 + len(post.URL)
		}

		countStyle := styleDim
		if charCount > 280 {
			countStyle = styleError
		}

		cardHeader := fmt.Sprintf("Post %d of %d  (%s)", i+1, len(posts), countStyle.Render(fmt.Sprintf("%d chars", charCount)))
		cursorMarker := "  "
		if isActive {
			cursorMarker = styleCursor.Render("▸ ")
		}

		b.WriteString(cursorMarker + styleHeader.Render(cardHeader) + "\n")

		// Render post body wrapped
		wrapped := wrapPostText(post.Text, cardWidth-4)
		if isActive {
			b.WriteString(styleSelected.Render(wrapped) + "\n")
		} else {
			b.WriteString(styleValue.Render(wrapped) + "\n")
		}

		if post.URL != "" {
			b.WriteString(styleDim.Render("    🔗 "+truncate(post.URL, cardWidth-8)) + "\n")
		}
		b.WriteString(styleDim.Render("    "+repeat("─", cardWidth-4)) + "\n\n")
	}

	return b.String()
}

func wrapPostText(s string, width int) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return "    (empty)"
	}

	var b strings.Builder
	line := "    "
	for _, w := range words {
		if len([]rune(line))+len([]rune(w))+1 > width {
			b.WriteString(line + "\n")
			line = "    "
		}
		line += w + " "
	}
	b.WriteString(strings.TrimRight(line, " "))
	return b.String()
}
