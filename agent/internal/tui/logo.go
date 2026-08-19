package tui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
)

// The wordmark is the logo exported in agent/AIRFOIL-settings.json: the block
// face filled solid, coloured #ff0000 → #ffffff along the diagonal, on a
// transparent background, centred, with no tagline and no rule. Those settings
// live here as constants so the terminal and the exported PNG stay the same
// mark; re-export and update both together.
const (
	logoWord = "AIRFOIL"
	logoFrom = "#ff0000" // customColor1
	logoTo   = "#ffffff" // customColor2
	logoRows = 5
)

// logoGlyphs is the solid block face. Every row of a glyph is padded to the
// glyph's own width when the banner is assembled, so the columns cannot drift.
var logoGlyphs = map[rune][logoRows]string{
	'A': {
		" █████ ",
		"██   ██",
		"███████",
		"██   ██",
		"██   ██",
	},
	'I': {
		"██",
		"██",
		"██",
		"██",
		"██",
	},
	'R': {
		"██████ ",
		"██   ██",
		"██████ ",
		"██  ██ ",
		"██   ██",
	},
	'F': {
		"███████",
		"██     ",
		"█████  ",
		"██     ",
		"██     ",
	},
	'O': {
		" ██████ ",
		"██    ██",
		"██    ██",
		"██    ██",
		" ██████ ",
	},
	'L': {
		"██     ",
		"██     ",
		"██     ",
		"██     ",
		"███████",
	},
}

// logoArt lays the glyphs out side by side, one blank column between letters.
// It returns plain text: colour is applied separately so the geometry stays
// testable.
func logoArt(word string) []string {
	lines := make([]string, logoRows)
	for i, r := range word {
		g, ok := logoGlyphs[r]
		if !ok {
			continue
		}
		w := 0
		for _, row := range g {
			w = max(w, len([]rune(row)))
		}
		for row := range logoRows {
			if i > 0 {
				lines[row] += " "
			}
			lines[row] += pad(g[row], w)
		}
	}
	return lines
}

// logoWidth is the banner's width in cells.
func logoWidth() int {
	art := logoArt(logoWord)
	if len(art) == 0 {
		return 0
	}
	return len([]rune(art[0]))
}

// gradient renders lines with a diagonal ramp from logoFrom at the top left to
// logoTo at the bottom right. Blank cells are left untouched, which is what
// "transparent background" means in a terminal.
func gradient(lines []string) string {
	from, to := hexRGB(logoFrom), hexRGB(logoTo)

	height := len(lines)
	width := 0
	for _, l := range lines {
		width = max(width, len([]rune(l)))
	}

	var b strings.Builder
	for y, line := range lines {
		if y > 0 {
			b.WriteByte('\n')
		}
		for x, r := range []rune(line) {
			if r == ' ' {
				b.WriteRune(r)
				continue
			}
			t := (axis(x, width) + axis(y, height)) / 2
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(mixHex(from, to, t))).
				Render(string(r)))
		}
	}
	return b.String()
}

// axis maps an index onto 0…1 along a span, so a one-cell span sits at the
// start of the ramp rather than dividing by zero.
func axis(i, span int) float64 {
	if span <= 1 {
		return 0
	}
	return float64(i) / float64(span-1)
}

func hexRGB(s string) [3]float64 {
	var r, g, b int
	fmt.Sscanf(s, "#%02x%02x%02x", &r, &g, &b)
	return [3]float64{float64(r), float64(g), float64(b)}
}

func mixHex(from, to [3]float64, t float64) string {
	t = min(max(t, 0), 1)
	c := [3]int{}
	for i := range c {
		c[i] = int(from[i] + (to[i]-from[i])*t + 0.5)
	}
	return fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2])
}

// The banner and the wordmark are fixed art, so render them once. lipgloss
// resolves the colour profile at first use, which is why this is lazy rather
// than a package variable.
var (
	bannerOnce    sync.Once
	bannerCache   string
	wordmarkOnce  sync.Once
	wordmarkCache string
)

// logoBanner is the full block wordmark, centred in width. It returns "" when
// the terminal is too narrow to hold the mark, so callers can simply skip it.
func logoBanner(width int) string {
	bannerOnce.Do(func() { bannerCache = gradient(logoArt(logoWord)) })

	art := logoWidth()
	if width < art {
		return ""
	}
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(bannerCache)
}

// logoWordmark is the one-line form for the header bar: the same ramp read left
// to right, so the chrome and the banner share an identity.
func logoWordmark() string {
	wordmarkOnce.Do(func() {
		from, to := hexRGB(logoFrom), hexRGB(logoTo)
		runes := []rune(logoWord)

		var b strings.Builder
		for i, r := range runes {
			b.WriteString(lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(mixHex(from, to, axis(i, len(runes))))).
				Render(string(r)))
		}
		wordmarkCache = b.String()
	})
	return wordmarkCache
}

// scale maps n against peak onto width cells, keeping any non-zero value
// visible as at least one cell.
func scale(n, peak, width int) int {
	switch {
	case n <= 0 || peak <= 0 || width <= 0:
		return 0
	case n >= peak:
		return width
	}
	return max(int(float64(n)/float64(peak)*float64(width)+0.5), 1)
}

// bar draws a proportional block bar in the wordmark's ramp, so the dashboard's
// figures are cut from the same material as the logo.
func bar(n, peak, width int) string {
	if width <= 0 {
		return ""
	}
	from, to := hexRGB(logoFrom), hexRGB(logoTo)
	filled := scale(n, peak, width)

	var b strings.Builder
	for i := range width {
		if i < filled {
			b.WriteString(lipgloss.NewStyle().
				Foreground(lipgloss.Color(mixHex(from, to, axis(i, width)))).
				Render("█"))
			continue
		}
		b.WriteString(styleRule.Render("░"))
	}
	return b.String()
}
