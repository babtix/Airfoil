package purge

import (
	"testing"
	"time"

	"github.com/papitsho/airfoil/internal/model"
)

func TestFilter(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)
	s1 := model.Story{
		ID:    "1",
		Slug:  "2026-08-19-gpt5-release",
		Title: "OpenAI Announces GPT-5",
		Score: 85,
		Tier:  "major",
		Date:  now.AddDate(0, 0, -1),
		Tags:  []string{"llm", "openai"},
		Sources: []model.StorySource{
			{Name: "Hacker News", URL: "https://news.ycombinator.com/item?id=1"},
		},
	}
	s2 := model.Story{
		ID:    "2",
		Slug:  "2026-05-10-old-infra-tool",
		Title: "Minor Infra Tooling Update",
		Score: 20,
		Tier:  "minor",
		Date:  now.AddDate(0, -3, -10), // ~100 days ago
		Tags:  []string{"infra", "tooling"},
		Sources: []model.StorySource{
			{Name: "GitHub", URL: "https://github.com/tools/infra"},
		},
	}
	s3 := model.Story{
		ID:    "3",
		Slug:  "2026-02-01-ancient-robotics",
		Title: "Robotics Hardware v1",
		Score: 40,
		Tier:  "notable",
		Date:  now.AddDate(0, -6, 0), // ~6 months ago
		Tags:  []string{"robotics", "hardware"},
		Sources: []model.StorySource{
			{Name: "ArXiv", URL: "https://arxiv.org/abs/2602.0001"},
		},
	}

	allStories := []model.Story{s1, s2, s3}

	intPtr := func(v int) *int { return &v }

	tests := []struct {
		name        string
		opts        Options
		wantKept    int
		wantRemoved int
		checkSlug   string
	}{
		{
			name:        "empty options keeps all",
			opts:        Options{},
			wantKept:    3,
			wantRemoved: 0,
		},
		{
			name: "before 90 days removes older stories",
			opts: Options{
				Before: now.AddDate(0, 0, -90),
			},
			wantKept:    1,
			wantRemoved: 2, // s2 and s3 are older than 90d
		},
		{
			name: "filter by tag robotics",
			opts: Options{
				Tags: []string{"robotics"},
			},
			wantKept:    2,
			wantRemoved: 1,
		},
		{
			name: "filter by tier minor",
			opts: Options{
				Tier: "minor",
			},
			wantKept:    2,
			wantRemoved: 1,
		},
		{
			name: "filter by max score",
			opts: Options{
				MaxScore: intPtr(30),
			},
			wantKept:    2,
			wantRemoved: 1, // s2 score is 20 <= 30
		},
		{
			name: "filter by slugs manifest",
			opts: Options{
				Slugs: []string{"2026-08-19-gpt5-release"},
			},
			wantKept:    2,
			wantRemoved: 1,
		},
		{
			name: "filter by combined before and tier",
			opts: Options{
				Before: now.AddDate(0, -2, 0),
				Tier:   "minor",
			},
			wantKept:    2,
			wantRemoved: 1, // only s2 is both older than 2 months and minor
		},
		{
			name: "filter by source",
			opts: Options{
				Sources: []string{"GitHub"},
			},
			wantKept:    2,
			wantRemoved: 1,
		},
		{
			name: "filter by query text",
			opts: Options{
				Query: "gpt-5",
			},
			wantKept:    2,
			wantRemoved: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Filter(allStories, tt.opts)
			if len(res.Kept) != tt.wantKept {
				t.Errorf("Filter() Kept count = %d, want %d", len(res.Kept), tt.wantKept)
			}
			if len(res.Removed) != tt.wantRemoved {
				t.Errorf("Filter() Removed count = %d, want %d", len(res.Removed), tt.wantRemoved)
			}
		})
	}
}

func TestParseBefore(t *testing.T) {
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		input   string
		wantErr bool
		want    time.Time
	}{
		{input: "", wantErr: false, want: time.Time{}},
		{input: "90d", wantErr: false, want: now.AddDate(0, 0, -90)},
		{input: "30days", wantErr: false, want: now.AddDate(0, 0, -30)},
		{input: "3m", wantErr: false, want: now.AddDate(0, -3, 0)},
		{input: "6months", wantErr: false, want: now.AddDate(0, -6, 0)},
		{input: "1y", wantErr: false, want: now.AddDate(-1, 0, 0)},
		{input: "2w", wantErr: false, want: now.AddDate(0, 0, -14)},
		{input: "72h", wantErr: false, want: now.Add(-72 * time.Hour)},
		{input: "2026-01-01", wantErr: false, want: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{input: "invalid-string-abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseBefore(tt.input, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseBefore(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("ParseBefore(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
