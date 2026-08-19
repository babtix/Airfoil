package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// The banner is art, and art that drifts a column is worse than none. These
// tests pin the geometry, not the pixels.
func TestLogoArtIsRectangular(t *testing.T) {
	art := logoArt(logoWord)

	if len(art) != logoRows {
		t.Fatalf("logoArt() returned %d rows, want %d", len(art), logoRows)
	}
	want := len([]rune(art[0]))
	for i, row := range art {
		if got := len([]rune(row)); got != want {
			t.Errorf("row %d is %d cells wide, want %d", i, got, want)
		}
	}
	if want != logoWidth() {
		t.Errorf("logoWidth() = %d, rows are %d wide", logoWidth(), want)
	}
}

func TestLogoArtSpellsTheWord(t *testing.T) {
	for _, r := range logoWord {
		if _, ok := logoGlyphs[r]; !ok {
			t.Errorf("no glyph for %q — the banner would silently drop a letter", r)
		}
	}

	// Every letter contributes ink, and the blank columns between them survive.
	art := logoArt(logoWord)
	for _, row := range art {
		if strings.TrimSpace(row) == "" {
			continue
		}
		if !strings.Contains(row, "█") {
			t.Errorf("row %q has no block fill", row)
		}
	}
}

func TestLogoBannerFitsItsWidth(t *testing.T) {
	art := logoWidth()

	tests := []struct {
		name  string
		width int
		want  bool // a banner is drawn
	}{
		{"exact fit", art, true},
		{"roomy", art + 40, true},
		{"one cell short", art - 1, false},
		{"unset", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := logoBanner(tt.width)
			if (got != "") != tt.want {
				t.Fatalf("logoBanner(%d) drawn = %v, want %v", tt.width, got != "", tt.want)
			}
			if got == "" {
				return
			}
			for _, line := range strings.Split(got, "\n") {
				if w := lipgloss.Width(line); w > tt.width {
					t.Errorf("line is %d cells wide, over the %d given", w, tt.width)
				}
			}
		})
	}
}

func TestScale(t *testing.T) {
	tests := []struct {
		name           string
		n, peak, width int
		want           int
	}{
		{"empty", 0, 100, 20, 0},
		{"the peak fills", 100, 100, 20, 20},
		{"half", 50, 100, 20, 10},
		{"a trace still shows", 1, 1000, 20, 1},
		{"over the peak clamps", 200, 100, 20, 20},
		{"no room", 50, 100, 0, 0},
		{"no peak", 50, 0, 20, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scale(tt.n, tt.peak, tt.width); got != tt.want {
				t.Errorf("scale(%d, %d, %d) = %d, want %d",
					tt.n, tt.peak, tt.width, got, tt.want)
			}
		})
	}
}

func TestBarKeepsItsWidth(t *testing.T) {
	for _, n := range []int{0, 1, 50, 100, 500} {
		if got := lipgloss.Width(bar(n, 100, 16)); got != 16 {
			t.Errorf("bar(%d, 100, 16) is %d cells, want 16", n, got)
		}
	}
	if bar(1, 10, 0) != "" {
		t.Error("a zero-width bar should render nothing")
	}
}

func TestMixHex(t *testing.T) {
	from, to := hexRGB(logoFrom), hexRGB(logoTo)

	tests := []struct {
		name string
		t    float64
		want string
	}{
		{"start", 0, "#ff0000"},
		{"end", 1, "#ffffff"},
		{"midpoint", 0.5, "#ff8080"},
		{"under clamps", -1, "#ff0000"},
		{"over clamps", 2, "#ffffff"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mixHex(from, to, tt.t); got != tt.want {
				t.Errorf("mixHex(%v) = %s, want %s", tt.t, got, tt.want)
			}
		})
	}
}

func TestGradientLeavesBlanksTransparent(t *testing.T) {
	// A space carries no colour, so the terminal background shows through.
	if got := gradient([]string{"  "}); got != "  " {
		t.Errorf("gradient(blank) = %q, want two plain spaces", got)
	}
}
