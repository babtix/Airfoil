package tui

import (
	"fmt"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestPreviewDashboard(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	m := testModel(t)
	m.width, m.height = 110, 40
	last := time.Now().Add(-3 * time.Hour)
	d := m.pages[viewDashboard].(*dashboard)
	d.loaded = true
	d.stats = dataStats{Items: 1441, ItemDays: 30, Clusters: 812, MultiSource: 96,
		Ranked: 812, Major: 41, Notable: 180, Stories: 120, Markdown: 120,
		DigestFiles: 3, SeenURLs: 5210, LastRun: &last}
	fmt.Println(m.View())
	fmt.Println("=== narrow 74x20 ===")
	m.width, m.height = 74, 20
	fmt.Println(m.View())
}
