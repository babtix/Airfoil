package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestProvidersPageNavigation(t *testing.T) {
	cfg := testConfig(t)
	p := newProvidersPage(cfg)

	if p.cursor != 1 {
		t.Fatalf("initial cursor = %d, want 1 (first editable row)", p.cursor)
	}

	// Move down
	p.Update(tea.KeyMsg{Type: tea.KeyDown}, nil)
	if p.cursor != 2 {
		t.Errorf("cursor after down = %d, want 2", p.cursor)
	}

	// Move past section header
	for i := 0; i < 5; i++ {
		p.Update(tea.KeyMsg{Type: tea.KeyDown}, nil)
	}
	if p.rows[p.cursor].isHeading() {
		t.Errorf("cursor landed on heading at index %d", p.cursor)
	}
}

func TestProvidersPageEditAndMask(t *testing.T) {
	cfg := testConfig(t)
	p := newProvidersPage(cfg)
	m := testModel(t)

	// Select first editable row (OPENROUTER_API_KEY)
	p.cursor = 1
	p.Update(tea.KeyMsg{Type: tea.KeyEnter}, m)
	if !p.active {
		t.Fatal("expected active=true after enter")
	}

	// Type new key
	p.input.SetValue("sk-or-v1-secret-key-123456")
	p.Update(tea.KeyMsg{Type: tea.KeyEnter}, m)

	if p.active {
		t.Fatal("expected active=false after enter submit")
	}
	if !p.dirty {
		t.Fatal("expected dirty=true after edit")
	}
	if p.values["OPENROUTER_API_KEY"] != "sk-or-v1-secret-key-123456" {
		t.Errorf("value = %q, want %q", p.values["OPENROUTER_API_KEY"], "sk-or-v1-secret-key-123456")
	}

	// Verify display masking
	rendered := formatDisplayValue("nvapi-secret-key-123456", true)
	if !strings.Contains(rendered, "••••") || !strings.HasSuffix(rendered, "3456") {
		t.Errorf("formatDisplayValue() = %q, want masked with 3456 suffix", rendered)
	}
}

func TestProvidersPageCycle(t *testing.T) {
	cfg := testConfig(t)
	p := newProvidersPage(cfg)
	m := testModel(t)

	// Find log level row
	logLevelIdx := -1
	for i, r := range p.rows {
		if r.key == "AIRFOIL_LOG_LEVEL" {
			logLevelIdx = i
			break
		}
	}
	if logLevelIdx == -1 {
		t.Fatal("AIRFOIL_LOG_LEVEL row not found")
	}

	p.cursor = logLevelIdx
	p.values["AIRFOIL_LOG_LEVEL"] = "info"

	p.Update(tea.KeyMsg{Type: tea.KeyRight}, m)
	if p.values["AIRFOIL_LOG_LEVEL"] != "warn" {
		t.Errorf("log level after cycle right = %q, want 'warn'", p.values["AIRFOIL_LOG_LEVEL"])
	}

	p.Update(tea.KeyMsg{Type: tea.KeyLeft}, m)
	if p.values["AIRFOIL_LOG_LEVEL"] != "info" {
		t.Errorf("log level after cycle left = %q, want 'info'", p.values["AIRFOIL_LOG_LEVEL"])
	}
}

func TestProvidersPageSaveAndReload(t *testing.T) {
	cfg := testConfig(t)
	m := testModel(t)
	m.cfg = cfg

	envPath := filepath.Join(cfg.ConfigDir, "..", ".env")
	_ = os.WriteFile(envPath, []byte("NVIDIA_NIM_MODEL=old-model\n"), 0o644)

	p := newProvidersPage(cfg)
	p.values["NVIDIA_NIM_MODEL"] = "meta/llama-3.3-70b-instruct"
	p.dirty = true

	// Save with 's'
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}, m)

	if p.dirty {
		t.Error("expected dirty=false after save")
	}
	if m.cfg.LLM.NvidiaNIM.Model != "meta/llama-3.3-70b-instruct" {
		t.Errorf("cfg model = %q, want 'meta/llama-3.3-70b-instruct'", m.cfg.LLM.NvidiaNIM.Model)
	}

	// Edit without saving and reload with 'r'
	p.values["NVIDIA_NIM_MODEL"] = "temporary-model"
	p.dirty = true
	p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}, m)

	if p.dirty {
		t.Error("expected dirty=false after reload")
	}
	if p.values["NVIDIA_NIM_MODEL"] != "meta/llama-3.3-70b-instruct" {
		t.Errorf("reloaded value = %q, want 'meta/llama-3.3-70b-instruct'", p.values["NVIDIA_NIM_MODEL"])
	}
}
