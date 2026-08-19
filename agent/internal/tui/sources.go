package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/papitsho/airfoil/internal/config"
)

// sourcesPage edits config/sources.json: the feed list, its tiers, and the
// per-adapter options.
//
// Edits are held in memory and marked dirty until saved, so an experiment can
// be abandoned with `r` without touching disk.
type sourcesPage struct {
	sources []config.Source
	cursor  int
	dirty   bool

	// editing is the detail form for the selected source, nil when browsing.
	editing *sourceForm
}

// sourceForm is the field-by-field editor for one source.
type sourceForm struct {
	index  int // -1 for a new source
	src    config.Source
	fields []formField
	cursor int
	input  textinput.Model
	active bool // true while a text field has focus
}

// formField describes one editable property.
type formField struct {
	label string
	// get renders the current value.
	get func(*config.Source) string
	// set parses a typed value. Returning an error rejects the edit.
	set func(*config.Source, string) error
	// cycle steps through a fixed set of choices, for enums and booleans.
	cycle func(*config.Source, int)
	// only shows the field for these source types; empty means always.
	only []string
}

func newSourcesPage(cfg *config.Config) *sourcesPage {
	return &sourcesPage{sources: cloneSources(cfg.Sources)}
}

func cloneSources(in []config.Source) []config.Source {
	out := make([]config.Source, len(in))
	copy(out, in)
	for i := range out {
		out[i].Tags = append([]string(nil), in[i].Tags...)
		out[i].Options.Queries = append([]string(nil), in[i].Options.Queries...)
	}
	return out
}

func (p *sourcesPage) Init() tea.Cmd { return nil }

func (p *sourcesPage) Footer() string {
	if p.editing != nil {
		if p.editing.active {
			return styleKey.Render("enter") + styleFooter.Render(" accept  ") +
				styleKey.Render("esc") + styleFooter.Render(" cancel")
		}
		return styleKey.Render("↑↓") + styleFooter.Render(" field  ") +
			styleKey.Render("enter") + styleFooter.Render(" edit  ") +
			styleKey.Render("←→") + styleFooter.Render(" cycle  ") +
			styleKey.Render("esc") + styleFooter.Render(" back")
	}

	dirty := ""
	if p.dirty {
		dirty = styleWarn.Render("  unsaved")
	}
	return styleKey.Render("↑↓") + styleFooter.Render(" move  ") +
		styleKey.Render("space") + styleFooter.Render(" toggle  ") +
		styleKey.Render("enter") + styleFooter.Render(" edit  ") +
		styleKey.Render("n") + styleFooter.Render(" new  ") +
		styleKey.Render("D") + styleFooter.Render(" delete  ") +
		styleKey.Render("s") + styleFooter.Render(" save  ") +
		styleKey.Render("r") + styleFooter.Render(" reload") + dirty
}

func (p *sourcesPage) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}
	if p.editing != nil {
		return p.updateForm(key, m)
	}
	return p.updateList(key, m)
}

func (p *sourcesPage) updateList(key tea.KeyMsg, m *Model) (tea.Cmd, bool) {
	switch key.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
		return nil, true

	case "down", "j":
		if p.cursor < len(p.sources)-1 {
			p.cursor++
		}
		return nil, true

	case " ":
		if len(p.sources) > 0 {
			p.sources[p.cursor].Enabled = !p.sources[p.cursor].Enabled
			p.dirty = true
		}
		return nil, true

	case "enter":
		if len(p.sources) > 0 {
			p.editing = newSourceForm(p.cursor, p.sources[p.cursor])
		}
		return nil, true

	case "n":
		p.editing = newSourceForm(-1, config.Source{
			Type: config.SourceRSS, Tier: 4, Enabled: true,
		})
		return nil, true

	case "D":
		if len(p.sources) > 0 {
			removed := p.sources[p.cursor].ID
			p.sources = append(p.sources[:p.cursor], p.sources[p.cursor+1:]...)
			p.cursor = min(p.cursor, max(len(p.sources)-1, 0))
			p.dirty = true
			m.setStatus("removed %s — press s to save", removed)
		}
		return nil, true

	case "s":
		if err := m.cfg.SaveSources(cloneSources(p.sources)); err != nil {
			m.setError(err)
			return nil, true
		}
		p.dirty = false
		m.setStatus("saved %d sources to %s", len(p.sources), m.cfg.SourcesPath())
		return nil, true

	case "r":
		if err := m.cfg.Reload(); err != nil {
			m.setError(err)
			return nil, true
		}
		p.sources = cloneSources(m.cfg.Sources)
		p.cursor = min(p.cursor, max(len(p.sources)-1, 0))
		p.dirty = false
		m.setStatus("reloaded from disk")
		return nil, true
	}
	return nil, false
}

func (p *sourcesPage) updateForm(key tea.KeyMsg, m *Model) (tea.Cmd, bool) {
	f := p.editing

	// While a text field has focus it consumes every key except enter and esc.
	if f.active {
		switch key.String() {
		case "enter":
			field := f.visibleFields()[f.cursor]
			if err := field.set(&f.src, f.input.Value()); err != nil {
				m.setError(err)
				return nil, true
			}
			f.active = false
			m.setStatus("")
			return nil, true

		case "esc":
			f.active = false
			return nil, true
		}

		var cmd tea.Cmd
		f.input, cmd = f.input.Update(key)
		return cmd, true
	}

	visible := f.visibleFields()

	switch key.String() {
	case "esc":
		p.editing = nil
		return nil, true

	case "up", "k":
		if f.cursor > 0 {
			f.cursor--
		}
		return nil, true

	case "down", "j":
		if f.cursor < len(visible)-1 {
			f.cursor++
		}
		return nil, true

	case "left":
		if c := visible[f.cursor].cycle; c != nil {
			c(&f.src, -1)
		}
		return nil, true

	case "right":
		if c := visible[f.cursor].cycle; c != nil {
			c(&f.src, +1)
		}
		return nil, true

	case "enter":
		field := visible[f.cursor]
		if field.cycle != nil && field.set == nil {
			field.cycle(&f.src, +1)
			return nil, true
		}
		f.input.SetValue(field.get(&f.src))
		f.input.CursorEnd()
		f.input.Focus()
		f.active = true
		return nil, true

	case "s":
		// Commit the form back into the list; saving to disk is still `s` on
		// the list, so an edit can be reviewed alongside the others first.
		if err := validateSource(f.src); err != nil {
			m.setError(err)
			return nil, true
		}
		if f.index < 0 {
			p.sources = append(p.sources, f.src)
			p.cursor = len(p.sources) - 1
		} else {
			p.sources[f.index] = f.src
		}
		p.dirty = true
		p.editing = nil
		m.setStatus("applied — press s again to write config/sources.json")
		return nil, true
	}
	return nil, true // the form swallows everything else
}

func validateSource(s config.Source) error {
	switch {
	case strings.TrimSpace(s.ID) == "":
		return fmt.Errorf("source needs an id")
	case strings.TrimSpace(s.Name) == "":
		return fmt.Errorf("source needs a name")
	case strings.TrimSpace(s.URL) == "":
		return fmt.Errorf("source needs a url")
	case s.Tier < 1 || s.Tier > 5:
		return fmt.Errorf("tier must be between 1 and 5")
	}
	return nil
}

func (p *sourcesPage) View(m *Model, width, height int) string {
	if p.editing != nil {
		return p.editing.view(width, height)
	}

	var b strings.Builder
	b.WriteString(styleHeader.Render(fmt.Sprintf("sources  (%d)", len(p.sources))))
	b.WriteString(styleDim.Render("   " + m.cfg.SourcesPath()))
	b.WriteString("\n\n")

	b.WriteString(styleLabel.Render("  " + pad("id", 22) + pad("type", 11) +
		pad("tier", 6) + pad("on", 5) + "url"))
	b.WriteString("\n")

	// Keep the cursor in view when the list is longer than the pane.
	visible := max(height-4, 3)
	start := 0
	if p.cursor >= visible {
		start = p.cursor - visible + 1
	}
	end := min(start+visible, len(p.sources))

	for i := start; i < end; i++ {
		s := p.sources[i]

		row := pad(truncate(s.ID, 21), 22) +
			pad(s.Type, 11) +
			pad(fmt.Sprintf("t%d", s.Tier), 6) +
			pad(boolMarkPlain(s.Enabled), 5) +
			truncate(s.URL, max(width-46, 12))

		if i == p.cursor {
			b.WriteString(styleCursor.Render("▸ ") + styleSelected.Render(row))
		} else {
			style := styleValue
			if !s.Enabled {
				style = styleDim
			}
			b.WriteString("  " + tierStyle(s.Tier).Render(pad(truncate(s.ID, 21), 22)) +
				style.Render(row[22:]))
		}
		b.WriteString("\n")
	}

	if len(p.sources) > visible {
		b.WriteString(styleDim.Render(fmt.Sprintf("\n  showing %d–%d of %d",
			start+1, end, len(p.sources))))
	}
	return b.String()
}

func boolMarkPlain(on bool) string {
	if on {
		return "yes"
	}
	return "no"
}

// --- the per-source form ----------------------------------------------------

func newSourceForm(index int, src config.Source) *sourceForm {
	in := textinput.New()
	in.Prompt = "› "
	in.CharLimit = 400

	return &sourceForm{
		index:  index,
		src:    src,
		fields: sourceFields(),
		input:  in,
	}
}

// visibleFields filters out options that do not apply to the chosen type, so
// the form only ever shows knobs that do something.
func (f *sourceForm) visibleFields() []formField {
	out := make([]formField, 0, len(f.fields))
	for _, field := range f.fields {
		if len(field.only) == 0 || contains(field.only, f.src.Type) {
			out = append(out, field)
		}
	}
	return out
}

func (f *sourceForm) view(width, height int) string {
	var b strings.Builder

	title := "edit source"
	if f.index < 0 {
		title = "new source"
	}
	b.WriteString(styleHeader.Render(title))
	b.WriteString(styleDim.Render("   " + f.src.Type))
	b.WriteString("\n\n")

	for i, field := range f.visibleFields() {
		label := styleLabel.Render(pad(field.label, 22))
		value := field.get(&f.src)

		switch {
		case i == f.cursor && f.active:
			b.WriteString(styleCursor.Render("▸ ") + label + f.input.View() + "\n")
		case i == f.cursor:
			b.WriteString(styleCursor.Render("▸ ") + label + styleSelected.Render(" "+value+" ") + "\n")
		default:
			b.WriteString("  " + label + styleValue.Render(value) + "\n")
		}
	}

	b.WriteString("\n" + styleDim.Render("  enter to edit · ← → to cycle · s to apply · esc to discard"))
	return b.String()
}

// sourceFields defines the editable schema of a source.
func sourceFields() []formField {
	return []formField{
		{
			label: "id",
			get:   func(s *config.Source) string { return s.ID },
			set: func(s *config.Source, v string) error {
				v = strings.TrimSpace(v)
				if v == "" {
					return fmt.Errorf("id cannot be empty")
				}
				s.ID = v
				return nil
			},
		},
		{
			label: "name",
			get:   func(s *config.Source) string { return s.Name },
			set: func(s *config.Source, v string) error {
				s.Name = strings.TrimSpace(v)
				return nil
			},
		},
		{
			label: "type",
			get:   func(s *config.Source) string { return s.Type },
			cycle: func(s *config.Source, d int) {
				types := config.ValidSourceTypes()
				s.Type = cycleString(types, s.Type, d)
			},
		},
		{
			label: "url",
			get:   func(s *config.Source) string { return s.URL },
			set: func(s *config.Source, v string) error {
				s.URL = strings.TrimSpace(v)
				return nil
			},
		},
		{
			label: "tier",
			get:   func(s *config.Source) string { return strconv.Itoa(s.Tier) },
			cycle: func(s *config.Source, d int) {
				s.Tier = clamp(s.Tier+d, 1, 5)
			},
		},
		{
			label: "enabled",
			get:   func(s *config.Source) string { return boolMarkPlain(s.Enabled) },
			cycle: func(s *config.Source, _ int) { s.Enabled = !s.Enabled },
		},
		{
			label: "tags",
			get:   func(s *config.Source) string { return strings.Join(s.Tags, ", ") },
			set: func(s *config.Source, v string) error {
				s.Tags = splitList(v)
				return nil
			},
		},
		{
			label: "note",
			get:   func(s *config.Source) string { return s.Comment },
			set: func(s *config.Source, v string) error {
				s.Comment = strings.TrimSpace(v)
				return nil
			},
		},

		// hn
		{
			label: "queries",
			only:  []string{config.SourceHN, config.SourceGitHub},
			get:   func(s *config.Source) string { return strings.Join(s.Options.Queries, ", ") },
			set: func(s *config.Source, v string) error {
				s.Options.Queries = splitList(v)
				return nil
			},
		},
		{
			label: "min points",
			only:  []string{config.SourceHN},
			get:   func(s *config.Source) string { return strconv.Itoa(s.Options.MinPoints) },
			set:   intSetter(func(s *config.Source) *int { return &s.Options.MinPoints }),
			cycle: intCycler(func(s *config.Source) *int { return &s.Options.MinPoints }, 10, 0, 100000),
		},
		{
			label: "hours back",
			only:  []string{config.SourceHN},
			get:   func(s *config.Source) string { return strconv.Itoa(s.Options.HoursBack) },
			set:   intSetter(func(s *config.Source) *int { return &s.Options.HoursBack }),
			cycle: intCycler(func(s *config.Source) *int { return &s.Options.HoursBack }, 6, 1, 720),
		},

		// reddit
		{
			label: "limit",
			only:  []string{config.SourceReddit},
			get:   func(s *config.Source) string { return strconv.Itoa(s.Options.Limit) },
			set:   intSetter(func(s *config.Source) *int { return &s.Options.Limit }),
			cycle: intCycler(func(s *config.Source) *int { return &s.Options.Limit }, 10, 1, 100),
		},
		{
			label: "min score",
			only:  []string{config.SourceReddit},
			get:   func(s *config.Source) string { return strconv.Itoa(s.Options.MinScore) },
			set:   intSetter(func(s *config.Source) *int { return &s.Options.MinScore }),
			cycle: intCycler(func(s *config.Source) *int { return &s.Options.MinScore }, 25, 0, 100000),
		},

		// hf_papers
		{
			label: "min upvotes",
			only:  []string{config.SourceHFPapers},
			get:   func(s *config.Source) string { return strconv.Itoa(s.Options.MinUpvotes) },
			set:   intSetter(func(s *config.Source) *int { return &s.Options.MinUpvotes }),
			cycle: intCycler(func(s *config.Source) *int { return &s.Options.MinUpvotes }, 1, 0, 1000),
		},

		// github
		{
			label: "created within days",
			only:  []string{config.SourceGitHub},
			get:   func(s *config.Source) string { return strconv.Itoa(s.Options.CreatedWithinDays) },
			set:   intSetter(func(s *config.Source) *int { return &s.Options.CreatedWithinDays }),
			cycle: intCycler(func(s *config.Source) *int { return &s.Options.CreatedWithinDays }, 1, 1, 365),
		},
		{
			label: "min stars",
			only:  []string{config.SourceGitHub},
			get:   func(s *config.Source) string { return strconv.Itoa(s.Options.MinStars) },
			set:   intSetter(func(s *config.Source) *int { return &s.Options.MinStars }),
			cycle: intCycler(func(s *config.Source) *int { return &s.Options.MinStars }, 50, 0, 100000),
		},
		{
			label: "max per query",
			only:  []string{config.SourceGitHub},
			get:   func(s *config.Source) string { return strconv.Itoa(s.Options.MaxPerQuery) },
			set:   intSetter(func(s *config.Source) *int { return &s.Options.MaxPerQuery }),
			cycle: intCycler(func(s *config.Source) *int { return &s.Options.MaxPerQuery }, 1, 1, 100),
		},
	}
}

func intSetter(ref func(*config.Source) *int) func(*config.Source, string) error {
	return func(s *config.Source, v string) error {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return fmt.Errorf("%q is not a number", v)
		}
		*ref(s) = n
		return nil
	}
}

func intCycler(ref func(*config.Source) *int, step, lo, hi int) func(*config.Source, int) {
	return func(s *config.Source, d int) {
		p := ref(s)
		*p = clamp(*p+d*step, lo, hi)
	}
}

func cycleString(options []string, current string, d int) string {
	at := 0
	for i, o := range options {
		if o == current {
			at = i
			break
		}
	}
	next := (at + d + len(options)) % len(options)
	return options[next]
}

func splitList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func clamp(v, lo, hi int) int { return min(max(v, lo), hi) }
