package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestTopCounts(t *testing.T) {
	tally := map[string]int{"hn": 40, "openai": 12, "reddit": 40, "arxiv": 3}

	tests := []struct {
		name string
		n    int
		want []counted
	}{
		{
			// Ties break by name, or the panel reshuffles on every refresh.
			name: "ranked, ties by name",
			n:    4,
			want: []counted{{"hn", 40}, {"reddit", 40}, {"openai", 12}, {"arxiv", 3}},
		},
		{
			name: "truncated to n",
			n:    2,
			want: []counted{{"hn", 40}, {"reddit", 40}},
		},
		{
			name: "n of zero keeps everything",
			n:    0,
			want: []counted{{"hn", 40}, {"reddit", 40}, {"openai", 12}, {"arxiv", 3}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := topCounts(tally, tt.n)
			if len(got) != len(tt.want) {
				t.Fatalf("topCounts() returned %d entries, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("entry %d = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}

	if got := topCounts(map[string]int{}, 5); len(got) != 0 {
		t.Errorf("an empty tally returned %d entries", len(got))
	}
}

func TestHumanBytes(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{"zero", 0, "0 B"},
		{"bytes", 512, "512 B"},
		{"kilobytes", 2048, "2.0 KB"},
		{"megabytes", 5 * 1024 * 1024, "5.0 MB"},
		{"gigabytes", 3 * 1024 * 1024 * 1024, "3.0 GB"},
		{"stops at terabytes", 4096 * 1024 * 1024 * 1024, "4.0 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := humanBytes(tt.n); got != tt.want {
				t.Errorf("humanBytes(%d) = %s, want %s", tt.n, got, tt.want)
			}
		})
	}
}

func TestPlural(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0 items"},
		{1, "1 item"},
		{2, "2 items"},
	}

	for _, tt := range tests {
		if got := plural(tt.n, "item"); got != tt.want {
			t.Errorf("plural(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// fitColumn is what keeps the dashboard honest: it must never place a panel it
// has no rows for, and must hand the remainder on rather than dropping it.
func TestFitColumn(t *testing.T) {
	panels := []string{"a\na\na", "b\nb", "c\nc\nc\nc"} // 3, 2, 4 rows

	tests := []struct {
		name    string
		height  int
		placed  int
		rest    int
		rows    int
		wantAll bool
	}{
		{"everything fits", 20, 3, 0, 9, true},
		{"two of three", 5, 2, 1, 5, false},
		{"one of three", 3, 1, 2, 3, false},
		{"never places nothing", 1, 1, 2, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, rest := fitColumn(panels, tt.height)

			if n := lipgloss.Height(got); n != tt.rows {
				t.Errorf("placed %d rows, want %d", n, tt.rows)
			}
			if n := strings.Count(got, "\n") + 1; n != tt.rows {
				t.Errorf("joined block has %d lines, want %d", n, tt.rows)
			}
			if len(rest) != tt.rest {
				t.Errorf("handed back %d panels, want %d", len(rest), tt.rest)
			}
			if tt.wantAll && rest != nil {
				t.Error("nothing should be left over when everything fits")
			}
		})
	}

	if got, rest := fitColumn(nil, 10); got != "" || rest != nil {
		t.Errorf("fitColumn(nil) = %q, %v; want empty", got, rest)
	}
}

// The stats panels are the dashboard's whole point; none of them may crash or
// overrun on empty data, which is what a first run looks like.
func TestStatsPanelsOnEmptyData(t *testing.T) {
	m := testModel(t)
	d := m.pages[viewDashboard].(*dashboard)
	d.loaded = true

	for _, width := range []int{34, 48, 80} {
		for _, p := range d.statsPanels(m.cfg, width) {
			for _, line := range strings.Split(p, "\n") {
				if w := lipgloss.Width(line); w > width {
					t.Errorf("at width %d a panel line is %d cells: %q", width, w, line)
				}
			}
		}
		for _, p := range d.dataPanels(m.cfg, width) {
			for _, line := range strings.Split(p, "\n") {
				if w := lipgloss.Width(line); w > width {
					t.Errorf("at width %d a data panel line is %d cells: %q", width, w, line)
				}
			}
		}
		for _, p := range d.systemPanels(m.cfg, width) {
			for _, line := range strings.Split(p, "\n") {
				if w := lipgloss.Width(line); w > width {
					t.Errorf("at width %d a system panel line is %d cells: %q", width, w, line)
				}
			}
		}
		for _, p := range d.railPanels(width) {
			for _, line := range strings.Split(p, "\n") {
				if w := lipgloss.Width(line); w > width {
					t.Errorf("at width %d a rail panel line is %d cells: %q", width, w, line)
				}
			}
		}
	}
}

func TestDashboardLayoutBalanced(t *testing.T) {
	m := testModel(t)
	d := m.pages[viewDashboard].(*dashboard)
	d.loaded = true
	d.stats = dataStats{
		Items:        1499,
		ItemDays:     5,
		ItemsToday:   1001,
		Items24h:     946,
		WithSignal:   87,
		Clusters:     1459,
		MultiSource:  30,
		Ranked:       1459,
		Notable:      54,
		Stories:      1446,
		Markdown:     23,
		DigestFiles:  8,
		SeenURLs:     1499,
		CacheBytes:   63 * 1024 * 1024,
		ItemsBytes:   1200 * 1024,
		StoryMajor:   0,
		StoryNotable: 54,
		StoryMinor:   1392,
		BuilderRel:   320,
		StoriesToday: 901,
		AvgCluster:   1.0,
		TopStory:     "Geolocating a random island",
		TopScore:     51,
		BySource: []counted{
			{"arxiv-lg", 681},
			{"arxiv-ai", 537},
			{"arxiv-cl", 108},
			{"hn", 72},
			{"reddit-locallama", 26},
			{"techcrunch-ai", 22},
		},
		Silent: []string{"anthropic", "deepmind"},
		TopTags: []counted{
			{"research", 1298},
			{"models", 297},
			{"agents", 135},
			{"infra", 100},
			{"safety", 82},
			{"rag", 78},
		},
	}

	for _, size := range []struct {
		w, h int
	}{
		{120, 35},
		{100, 30},
		{80, 25},
		{50, 30},
	} {
		out := d.View(m, size.w, size.h)
		lines := strings.Split(out, "\n")
		for _, line := range lines {
			if w := lipgloss.Width(line); w > size.w {
				t.Errorf("at %dx%d, line exceeds width (%d > %d): %q", size.w, size.h, w, size.w, line)
			}
		}
	}
}
