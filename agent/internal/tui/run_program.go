package tui

import (
	"fmt"
	"log/slog"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/papitsho/airfoil/internal/config"
)

// Run starts the interactive interface and blocks until the user quits.
//
// Pipeline output is routed into the interface rather than to stderr, so the
// display is never corrupted by a stray log line.
func Run(cfg *config.Config, verbose bool) error {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	sink := NewLogSink(2000, level)
	log := slog.New(sink)

	program := tea.NewProgram(
		New(cfg, log, sink),
		tea.WithAltScreen(),
	)
	if _, err := program.Run(); err != nil {
		return fmt.Errorf("tui: %w", err)
	}
	return nil
}
