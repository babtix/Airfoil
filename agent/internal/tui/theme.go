package tui

import "github.com/charmbracelet/lipgloss"

// The palette mirrors site/src/styles/tokens.css so the terminal and the site
// read as one product.
var (
	colPrimary   = lipgloss.Color("#f5f5f5") // --prim
	colSecondary = lipgloss.Color("#a0a0a0") // --sec
	colMuted     = lipgloss.Color("#6b6b6b")
	colLine      = lipgloss.Color("#3a3a3a") // --line-light
	colAccent    = lipgloss.Color("#ff3b3b") // --kai
	colAccentDim = lipgloss.Color("#992323") // --kai-dim
	colOK        = lipgloss.Color("#3fb950")
	colWarn      = lipgloss.Color("#d29922")
	colSurface   = lipgloss.Color("#1a1a1a") // --surf-hover
)

var (
	// Chrome
	styleTitle = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleTab   = lipgloss.NewStyle().Foreground(colSecondary).Padding(0, 2)
	styleTabOn = lipgloss.NewStyle().Foreground(colPrimary).Background(colSurface).
			Bold(true).Padding(0, 2)
	styleFooter = lipgloss.NewStyle().Foreground(colMuted)
	styleKey    = lipgloss.NewStyle().Foreground(colAccent)

	// Text
	styleLabel  = lipgloss.NewStyle().Foreground(colSecondary)
	styleValue  = lipgloss.NewStyle().Foreground(colPrimary)
	styleDim    = lipgloss.NewStyle().Foreground(colMuted)
	styleHeader = lipgloss.NewStyle().Foreground(colPrimary).Bold(true)

	// Status
	styleOK    = lipgloss.NewStyle().Foreground(colOK)
	styleWarn  = lipgloss.NewStyle().Foreground(colWarn)
	styleError = lipgloss.NewStyle().Foreground(colAccent)

	// Selection
	styleSelected = lipgloss.NewStyle().Foreground(colPrimary).Background(colSurface).Bold(true)
	styleCursor   = lipgloss.NewStyle().Foreground(colAccent).Bold(true)

	// Containers
	stylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).BorderForeground(colLine).Padding(0, 1)
	styleEditing = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).BorderForeground(colAccent).Padding(0, 1)
	styleRule = lipgloss.NewStyle().Foreground(colLine)
)

// tierStyle colours a source tier the way the site does: labs in the accent,
// press and community progressively dimmer.
func tierStyle(tier int) lipgloss.Style {
	switch {
	case tier <= 1:
		return lipgloss.NewStyle().Foreground(colAccent)
	case tier <= 3:
		return lipgloss.NewStyle().Foreground(colPrimary)
	case tier == 4:
		return lipgloss.NewStyle().Foreground(colSecondary)
	default:
		return lipgloss.NewStyle().Foreground(colMuted)
	}
}

// boolStyle renders an on/off state.
func boolMark(on bool) string {
	if on {
		return styleOK.Render("on")
	}
	return styleDim.Render("off")
}

// rule draws a horizontal divider.
func rule(width int) string {
	if width < 1 {
		return ""
	}
	return styleRule.Render(repeat("─", width))
}

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, len(s)*n)
	for range n {
		out = append(out, s...)
	}
	return string(out)
}

// truncate shortens s to n display cells, with an ellipsis when it cuts.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}

// pad right-pads s to n display cells.
func pad(s string, n int) string {
	r := []rune(s)
	if len(r) >= n {
		return s
	}
	return s + repeat(" ", n-len(r))
}
