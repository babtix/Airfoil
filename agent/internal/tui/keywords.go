package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/papitsho/airfoil/internal/config"
)

// keywordsPage edits config/keywords.json: the phrases that raise a story's
// score, the ones that sink it, and the topic tags.
//
// The left pane picks a list; the right pane edits its entries.
type keywordsPage struct {
	keywords config.Keywords
	lists    []keywordList
	list     int // selected list on the left
	entry    int // selected entry on the right
	focus    int // 0 = list pane, 1 = entry pane
	dirty    bool

	input  textinput.Model
	active bool
	adding bool
}

// keywordList names one editable slice inside the config.
type keywordList struct {
	label string
	help  string
	get   func(*config.Keywords) []string
	set   func(*config.Keywords, []string)
}

func newKeywordsPage(cfg *config.Config) *keywordsPage {
	in := textinput.New()
	in.Prompt = "› "
	in.CharLimit = 120

	p := &keywordsPage{keywords: cloneKeywords(cfg.Keywords), input: in}
	p.lists = keywordLists(&p.keywords)
	return p
}

func cloneKeywords(k config.Keywords) config.Keywords {
	out := k
	out.BuilderSignals = append([]string(nil), k.BuilderSignals...)
	out.BuilderStructuralSignals = append([]string(nil), k.BuilderStructuralSignals...)
	out.HypeSignals = append([]string(nil), k.HypeSignals...)

	out.TopicTags = make(map[string][]string, len(k.TopicTags))
	for tag, words := range k.TopicTags {
		out.TopicTags[tag] = append([]string(nil), words...)
	}
	return out
}

// keywordLists builds the selectable lists, including one per topic tag so
// every phrase in the file is reachable.
func keywordLists(k *config.Keywords) []keywordList {
	lists := []keywordList{
		{
			label: "builder signals",
			help:  "each match adds to the score, capped in scoring.json",
			get:   func(k *config.Keywords) []string { return k.BuilderSignals },
			set:   func(k *config.Keywords, v []string) { k.BuilderSignals = v },
		},
		{
			label: "hype signals",
			help:  "each match subtracts from the score",
			get:   func(k *config.Keywords) []string { return k.HypeSignals },
			set:   func(k *config.Keywords, v []string) { k.HypeSignals = v },
		},
		{
			label: "structural signals",
			help:  "properties rather than phrases, e.g. has_repo_url",
			get:   func(k *config.Keywords) []string { return k.BuilderStructuralSignals },
			set:   func(k *config.Keywords, v []string) { k.BuilderStructuralSignals = v },
		},
	}

	tags := make([]string, 0, len(k.TopicTags))
	for tag := range k.TopicTags {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	for _, tag := range tags {
		lists = append(lists, keywordList{
			label: "tag: " + tag,
			help:  "phrases that assign the " + tag + " tag",
			get:   func(k *config.Keywords) []string { return k.TopicTags[tag] },
			set: func(k *config.Keywords, v []string) {
				if k.TopicTags == nil {
					k.TopicTags = map[string][]string{}
				}
				k.TopicTags[tag] = v
			},
		})
	}
	return lists
}

func (p *keywordsPage) Init() tea.Cmd { return nil }

func (p *keywordsPage) Footer() string {
	if p.active {
		return styleKey.Render("enter") + styleFooter.Render(" accept  ") +
			styleKey.Render("esc") + styleFooter.Render(" cancel")
	}
	dirty := ""
	if p.dirty {
		dirty = styleWarn.Render("  unsaved")
	}
	return styleKey.Render("←→") + styleFooter.Render(" pane  ") +
		styleKey.Render("↑↓") + styleFooter.Render(" move  ") +
		styleKey.Render("enter") + styleFooter.Render(" edit  ") +
		styleKey.Render("n") + styleFooter.Render(" add  ") +
		styleKey.Render("D") + styleFooter.Render(" delete  ") +
		styleKey.Render("s") + styleFooter.Render(" save  ") +
		styleKey.Render("r") + styleFooter.Render(" reload") + dirty
}

func (p *keywordsPage) current() keywordList { return p.lists[p.list] }

func (p *keywordsPage) entries() []string { return p.current().get(&p.keywords) }

func (p *keywordsPage) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}

	if p.active {
		switch key.String() {
		case "enter":
			value := strings.TrimSpace(p.input.Value())
			if value == "" {
				m.setError(fmt.Errorf("entry cannot be empty"))
				return nil, true
			}
			entries := append([]string(nil), p.entries()...)

			if p.adding {
				entries = append(entries, value)
				p.entry = len(entries) - 1
			} else {
				entries[p.entry] = value
			}
			p.current().set(&p.keywords, entries)

			p.active, p.adding, p.dirty = false, false, true
			m.setStatus("")
			return nil, true

		case "esc":
			p.active, p.adding = false, false
			return nil, true
		}

		var cmd tea.Cmd
		p.input, cmd = p.input.Update(key)
		return cmd, true
	}

	switch key.String() {
	case "left", "h":
		p.focus = 0
		return nil, true

	case "right", "l":
		p.focus = 1
		return nil, true

	case "up", "k":
		if p.focus == 0 {
			if p.list > 0 {
				p.list--
				p.entry = 0
			}
		} else if p.entry > 0 {
			p.entry--
		}
		return nil, true

	case "down", "j":
		if p.focus == 0 {
			if p.list < len(p.lists)-1 {
				p.list++
				p.entry = 0
			}
		} else if p.entry < len(p.entries())-1 {
			p.entry++
		}
		return nil, true

	case "enter":
		if p.focus == 0 {
			p.focus = 1
			return nil, true
		}
		if len(p.entries()) == 0 {
			return nil, true
		}
		p.input.SetValue(p.entries()[p.entry])
		p.input.CursorEnd()
		p.input.Focus()
		p.active = true
		return nil, true

	case "n":
		p.focus = 1
		p.adding, p.active = true, true
		p.input.SetValue("")
		p.input.Focus()
		return nil, true

	case "D":
		entries := p.entries()
		if len(entries) == 0 {
			return nil, true
		}
		next := append(append([]string(nil), entries[:p.entry]...), entries[p.entry+1:]...)
		p.current().set(&p.keywords, next)
		p.entry = min(p.entry, max(len(next)-1, 0))
		p.dirty = true
		return nil, true

	case "s":
		if err := m.cfg.SaveKeywords(p.keywords); err != nil {
			m.setError(err)
			return nil, true
		}
		p.dirty = false
		m.setStatus("saved %s", m.cfg.KeywordsPath())
		return nil, true

	case "r":
		if err := m.cfg.Reload(); err != nil {
			m.setError(err)
			return nil, true
		}
		p.keywords = cloneKeywords(m.cfg.Keywords)
		p.lists = keywordLists(&p.keywords)
		p.list = min(p.list, len(p.lists)-1)
		p.entry = 0
		p.dirty = false
		m.setStatus("reloaded from disk")
		return nil, true
	}
	return nil, false
}

func (p *keywordsPage) View(m *Model, width, height int) string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("keywords"))
	b.WriteString(styleDim.Render("   " + m.cfg.KeywordsPath()))
	b.WriteString("\n\n")

	listWidth := 26
	rows := max(height-4, 4)

	// Left pane: which list.
	var left strings.Builder
	for i, l := range p.lists {
		if i >= rows {
			break
		}
		count := styleDim.Render(fmt.Sprintf(" %d", len(l.get(&p.keywords))))
		label := truncate(l.label, listWidth-6)

		if i == p.list {
			marker := "  "
			if p.focus == 0 {
				marker = styleCursor.Render("▸ ")
			}
			left.WriteString(marker + styleSelected.Render(pad(label, listWidth-6)) + count + "\n")
		} else {
			left.WriteString("  " + styleValue.Render(pad(label, listWidth-6)) + count + "\n")
		}
	}

	// Right pane: the entries of the selected list.
	var right strings.Builder
	right.WriteString(styleDim.Render(p.current().help) + "\n\n")

	entries := p.entries()
	if len(entries) == 0 && !p.adding {
		right.WriteString(styleDim.Render("empty — press n to add an entry"))
	}

	start := 0
	if p.entry >= rows-2 {
		start = p.entry - (rows - 3)
	}
	end := min(start+rows-2, len(entries))

	for i := start; i < end; i++ {
		if i == p.entry && p.active && !p.adding {
			right.WriteString(styleCursor.Render("▸ ") + p.input.View() + "\n")
			continue
		}
		if i == p.entry && p.focus == 1 {
			right.WriteString(styleCursor.Render("▸ ") + styleSelected.Render(entries[i]) + "\n")
			continue
		}
		right.WriteString("  " + styleValue.Render(truncate(entries[i], max(width-listWidth-6, 10))) + "\n")
	}

	if p.adding {
		right.WriteString(styleCursor.Render("▸ ") + p.input.View() + "\n")
	}
	if len(entries) > end {
		right.WriteString(styleDim.Render(fmt.Sprintf("  … %d more", len(entries)-end)))
	}

	b.WriteString(columns(left.String(), right.String(), listWidth, 2))
	return b.String()
}
