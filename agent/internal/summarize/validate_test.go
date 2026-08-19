package summarize

import (
	"strings"
	"testing"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
)

// testLimits mirrors config/scoring.json's limits block.
func testLimits() config.Scoring {
	var s config.Scoring
	s.Limits.ExcerptMaxChars = 300
	s.Limits.SummaryMaxWords = 80
	s.Limits.TakeawaysMax = 3
	s.Limits.VerbatimOverlapMaxWords = 12
	s.Limits.QuotesMax = 1
	s.Limits.QuoteMaxWords = 15
	return s
}

func items(excerpts ...string) []model.Item {
	out := make([]model.Item, len(excerpts))
	for i, e := range excerpts {
		out[i] = model.Item{ID: string(rune('a' + i)), Excerpt: e}
	}
	return out
}

func TestParse(t *testing.T) {
	want := `{"title":"T","summary":"S","confidence":"high"}`

	tests := []struct {
		name string
		raw  string
		ok   bool
	}{
		{"bare json", want, true},
		{"json fence", "```json\n" + want + "\n```", true},
		{"plain fence", "```\n" + want + "\n```", true},
		{"leading prose", "Here is the JSON:\n" + want, true},
		{"trailing prose", want + "\n\nHope that helps!", true},
		{"whitespace", "\n\n  " + want + "  \n", true},
		{"not json", "I cannot summarize this.", false},
		{"empty", "", false},
		{"truncated", `{"title":"T","summary":`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.raw)
			if tt.ok {
				if err != nil {
					t.Fatalf("Parse() error = %v", err)
				}
				if got.Title != "T" || got.Summary != "S" {
					t.Errorf("Parse() = %+v", got)
				}
				return
			}
			if err == nil {
				t.Errorf("Parse() = %+v, want error", got)
			}
		})
	}
}

func TestVerbatimOverlap(t *testing.T) {
	source := "The company announced today that its newest large language model " +
		"will be available to all developers starting next month."

	tests := []struct {
		name    string
		summary string
		want    bool
	}{
		{
			name:    "original prose passes",
			summary: "Developers get access to the new model in a month.",
			want:    false,
		},
		{
			name:    "twelve word lift is caught",
			summary: "The company announced today that its newest large language model will be available.",
			want:    true,
		},
		{
			name:    "eleven words is under the limit",
			summary: "The company announced today that its newest large language model will.",
			want:    false,
		},
		{
			name: "punctuation changes do not evade the check",
			summary: "The company, announced today: that its newest large language model — " +
				"will be available!",
			want: true,
		},
		{
			name:    "case changes do not evade the check",
			summary: "THE COMPANY ANNOUNCED TODAY THAT ITS NEWEST LARGE LANGUAGE MODEL WILL BE AVAILABLE.",
			want:    true,
		},
		{
			name:    "short summary cannot overlap",
			summary: "Short.",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := VerbatimOverlap(tt.summary, items(source), 12)
			if ok != tt.want {
				t.Errorf("VerbatimOverlap() = %q, %v; want match=%v", got, ok, tt.want)
			}
		})
	}
}

func TestVerbatimOverlapAcrossMultipleSources(t *testing.T) {
	// The run appears only in the second excerpt — every source must be checked.
	srcs := items(
		"Something entirely unrelated about a different subject altogether here.",
		"Researchers released the weights under a permissive licence for anyone to use freely.",
	)

	if _, ok := VerbatimOverlap(
		"Researchers released the weights under a permissive licence for anyone to use freely.",
		srcs, 12); !ok {
		t.Error("overlap with the second source was missed")
	}
}

func TestVerbatimOverlapDisabled(t *testing.T) {
	if _, ok := VerbatimOverlap("anything at all", items("anything at all"), 0); ok {
		t.Error("a zero limit should disable the check, not match everything")
	}
}

func TestQuotedSpans(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"none", "No quotes here at all.", nil},
		{"one straight", `They called it "a major step" today.`, []string{"a major step"}},
		{"two straight", `Both "first" and "second" appeared.`, []string{"first", "second"}},
		{"curly doubles", "It was “a major step” today.", []string{"a major step"}},
		{"curly singles", "It was ‘quite good’ overall.", []string{"quite good"}},
		{"guillemets", "«bonjour» was printed.", []string{"bonjour"}},
		{"empty quotes ignored", `An empty "" quote.`, nil},
		{"unterminated ignored", `He said "unfinished`, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QuotedSpans(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("QuotedSpans(%q) = %v, want %v", tt.in, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("QuotedSpans(%q) = %v, want %v", tt.in, got, tt.want)
				}
			}
		})
	}
}

func TestValidate(t *testing.T) {
	src := items("A source excerpt with words that are not reused by the summary below.")
	base := Response{
		Title:      "A factual title",
		Summary:    "An original summary of the event.",
		Tags:       []string{"models"},
		Confidence: ConfidenceHigh,
	}

	with := func(mut func(*Response)) Response {
		r := base
		mut(&r)
		return r
	}

	tests := []struct {
		name    string
		in      Response
		wantErr bool
		check   func(t *testing.T, got Response)
	}{
		{
			name: "clean response passes",
			in:   base,
		},
		{
			name:    "empty title fails",
			in:      with(func(r *Response) { r.Title = "  " }),
			wantErr: true,
		},
		{
			name:    "empty summary fails",
			in:      with(func(r *Response) { r.Summary = "" }),
			wantErr: true,
		},
		{
			name:    "over-long summary fails",
			in:      with(func(r *Response) { r.Summary = strings.Repeat("word ", 81) }),
			wantErr: true,
		},
		{
			name: "summary at the limit passes",
			in:   with(func(r *Response) { r.Summary = strings.Repeat("word ", 80) }),
		},
		{
			name:    "two quotes fail",
			in:      with(func(r *Response) { r.Summary = `It was "one" and also "two".` }),
			wantErr: true,
		},
		{
			name: "one short quote passes",
			in:   with(func(r *Response) { r.Summary = `It was "a big step" overall.` }),
		},
		{
			name: "over-long quote fails",
			in: with(func(r *Response) {
				r.Summary = `He said "` + strings.Repeat("word ", 15) + `" today.`
			}),
			wantErr: true,
		},
		{
			name: "long title is trimmed, not rejected",
			in:   with(func(r *Response) { r.Title = strings.Repeat("a", 200) }),
			check: func(t *testing.T, got Response) {
				if len([]rune(got.Title)) > titleMaxChars {
					t.Errorf("title not trimmed: %d runes", len([]rune(got.Title)))
				}
			},
		},
		{
			name: "trailing period is stripped from the title",
			in:   with(func(r *Response) { r.Title = "A factual title." }),
			check: func(t *testing.T, got Response) {
				if strings.HasSuffix(got.Title, ".") {
					t.Errorf("title keeps a trailing period: %q", got.Title)
				}
			},
		},
		{
			name: "unknown tags are dropped, not fatal",
			in:   with(func(r *Response) { r.Tags = []string{"models", "invented", "AGENTS"} }),
			check: func(t *testing.T, got Response) {
				want := []string{"models", "agents"}
				if len(got.Tags) != len(want) {
					t.Fatalf("Tags = %v, want %v", got.Tags, want)
				}
				for i := range want {
					if got.Tags[i] != want[i] {
						t.Errorf("Tags = %v, want %v", got.Tags, want)
					}
				}
			},
		},
		{
			name: "duplicate tags collapse",
			in:   with(func(r *Response) { r.Tags = []string{"models", "Models", " models "} }),
			check: func(t *testing.T, got Response) {
				if len(got.Tags) != 1 {
					t.Errorf("Tags = %v, want one entry", got.Tags)
				}
			},
		},
		{
			name: "extra takeaways are capped",
			in:   with(func(r *Response) { r.Takeaways = []string{"a", "b", "c", "d", "e"} }),
			check: func(t *testing.T, got Response) {
				if len(got.Takeaways) != 3 {
					t.Errorf("Takeaways = %v, want 3", got.Takeaways)
				}
			},
		},
		{
			name: "unknown confidence becomes low",
			in:   with(func(r *Response) { r.Confidence = "pretty sure" }),
			check: func(t *testing.T, got Response) {
				if got.Confidence != ConfidenceLow {
					t.Errorf("Confidence = %q, want %q", got.Confidence, ConfidenceLow)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Validate(tt.in, src, testLimits())
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Validate() = %+v, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestValidateRejectsVerbatimSummary(t *testing.T) {
	excerpt := "The company announced today that its newest large language model " +
		"will be available to all developers starting next month."

	// A summary that lifts a full sentence must never reach disk (R1).
	_, err := Validate(Response{
		Title:      "Model ships",
		Summary:    excerpt,
		Confidence: ConfidenceHigh,
	}, items(excerpt), testLimits())

	if err == nil {
		t.Fatal("a verbatim summary was accepted")
	}
	if !strings.Contains(err.Error(), "verbatim") {
		t.Errorf("error = %v, want it to name the verbatim overlap", err)
	}
}

func TestDemoteTier(t *testing.T) {
	tests := []struct{ in, want string }{
		{model.TierMajor, model.TierNotable},
		{model.TierNotable, model.TierMinor},
		{model.TierMinor, model.TierMinor},
	}
	for _, tt := range tests {
		if got := DemoteTier(tt.in); got != tt.want {
			t.Errorf("DemoteTier(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
