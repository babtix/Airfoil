package score

import "testing"

func TestContainsPhrase(t *testing.T) {
	tests := []struct {
		name   string
		hay    string
		phrase string
		want   bool
	}{
		{"exact word", "the new api is live", "api", true},
		{"start of string", "api keys rotated", "api", true},
		{"end of string", "we shipped an api", "api", true},

		// The whole reason this function exists.
		{"not inside rapid", "rapid progress this year", "api", false},
		{"not inside capital", "raised capital", "api", false},
		{"not inside therapist", "an ai therapist", "api", false},
		{"not inside decode", "decode the stream", "code", false},
		{"not inside encoded", "encoded weights", "code", false},

		{"multi word phrase", "now available to everyone", "now available", true},
		{"phrase split by other words", "now it is available", "now available", false},

		{"hyphen is a boundary", "a non-open-source model", "open-source", true},
		{"suffix does not match", "fine-tuning the model", "fine-tune", false},
		{"exact hyphenated", "fine-tune it yourself", "fine-tune", true},

		{"punctuation boundary", "ships an API.", "api", false}, // corpus is lowercased upstream
		{"trailing punctuation", "ships an api.", "api", true},
		{"parenthesised", "(api) reference", "api", true},

		{"digit boundary blocks", "gpt5 model", "gpt", false},
		{"second occurrence matches", "rapid api rollout", "api", true},

		{"empty phrase", "anything", "", false},
		{"phrase longer than hay", "api", "api endpoint", false},
		{"no match at all", "nothing relevant here", "quantized", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsPhrase(tt.hay, tt.phrase); got != tt.want {
				t.Errorf("containsPhrase(%q, %q) = %v, want %v", tt.hay, tt.phrase, got, tt.want)
			}
		})
	}
}

func TestContainsPhraseUnicodeBoundary(t *testing.T) {
	// A multi-byte letter adjacent to the match must still count as a word
	// character, or byte-level boundary checks would produce false positives.
	if containsPhrase("caféapi", "api") {
		t.Error("matched across a multi-byte letter boundary")
	}
	if !containsPhrase("café api", "api") {
		t.Error("failed to match after a multi-byte letter and a space")
	}
}

func TestMatchAllDedupesAndSorts(t *testing.T) {
	got := matchAll("sdk and api and sdk again", []string{"sdk", "api", "SDK", "  sdk  ", "", "missing"})
	want := []string{"api", "sdk"}

	if len(got) != len(want) {
		t.Fatalf("matchAll = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("matchAll = %v, want %v", got, want)
		}
	}
}
