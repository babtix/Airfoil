package tui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// logLineMsg wakes the program when new log output has arrived.
type logLineMsg struct{}

// ringBuffer holds the retained output. It lives apart from the handler so that
// every handler derived by WithAttrs or WithGroup writes into the same display
// rather than a private buffer of its own.
type ringBuffer struct {
	mu     sync.Mutex
	lines  []string
	max    int
	notify chan struct{}
}

func (b *ringBuffer) append(line string) {
	b.mu.Lock()
	b.lines = append(b.lines, line)
	if len(b.lines) > b.max {
		b.lines = b.lines[len(b.lines)-b.max:]
	}
	b.mu.Unlock()

	// Non-blocking: if a wake-up is already pending, one is enough.
	select {
	case b.notify <- struct{}{}:
	default:
	}
}

// LogSink is an slog.Handler that captures pipeline output for display.
//
// Stages log through the ordinary slog interface, so nothing in the pipeline
// knows whether it is running under the CLI or the TUI.
type LogSink struct {
	buf    *ringBuffer
	level  slog.Level
	attrs  []slog.Attr
	groups []string
}

// NewLogSink returns a sink retaining the most recent max lines.
func NewLogSink(max int, level slog.Level) *LogSink {
	return &LogSink{
		buf: &ringBuffer{
			max:    max,
			notify: make(chan struct{}, 1),
		},
		level: level,
	}
}

func (s *LogSink) Enabled(_ context.Context, level slog.Level) bool {
	return level >= s.level
}

func (s *LogSink) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder

	b.WriteString(styleDim.Render(r.Time.Format(time.TimeOnly)))
	b.WriteString(" ")
	b.WriteString(levelStyle(r.Level).Render(pad(levelLabel(r.Level), 5)))
	b.WriteString(" ")
	b.WriteString(styleValue.Render(r.Message))

	prefix := ""
	if len(s.groups) > 0 {
		prefix = strings.Join(s.groups, ".") + "."
	}
	writeAttr := func(a slog.Attr) {
		b.WriteString("  ")
		b.WriteString(styleLabel.Render(prefix + a.Key + "="))
		b.WriteString(styleValue.Render(fmt.Sprint(a.Value.Any())))
	}

	for _, a := range s.attrs {
		writeAttr(a)
	}
	r.Attrs(func(a slog.Attr) bool {
		writeAttr(a)
		return true
	})

	s.buf.append(b.String())
	return nil
}

func (s *LogSink) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *s
	next.attrs = append(append([]slog.Attr{}, s.attrs...), attrs...)
	return &next
}

func (s *LogSink) WithGroup(name string) slog.Handler {
	next := *s
	next.groups = append(append([]string{}, s.groups...), name)
	return &next
}

// Append adds a line directly, for output that does not come through slog.
func (s *LogSink) Append(line string) { s.buf.append(line) }

// Lines returns a copy of the retained output.
func (s *LogSink) Lines() []string {
	s.buf.mu.Lock()
	defer s.buf.mu.Unlock()

	out := make([]string, len(s.buf.lines))
	copy(out, s.buf.lines)
	return out
}

// Clear empties the buffer.
func (s *LogSink) Clear() {
	s.buf.mu.Lock()
	s.buf.lines = nil
	s.buf.mu.Unlock()
}

// Wait returns a command that completes when new output arrives.
func (s *LogSink) Wait() tea.Cmd {
	return func() tea.Msg {
		<-s.buf.notify
		return logLineMsg{}
	}
}

func levelLabel(l slog.Level) string {
	switch {
	case l >= slog.LevelError:
		return "ERR"
	case l >= slog.LevelWarn:
		return "WARN"
	case l >= slog.LevelInfo:
		return "INFO"
	default:
		return "DBG"
	}
}

func levelStyle(l slog.Level) interface{ Render(...string) string } {
	switch {
	case l >= slog.LevelError:
		return styleError
	case l >= slog.LevelWarn:
		return styleWarn
	case l >= slog.LevelInfo:
		return styleOK
	default:
		return styleDim
	}
}
