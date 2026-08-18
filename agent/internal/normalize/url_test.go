package normalize

import "testing"

func TestCanonicalURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"already canonical", "https://openai.com/news/gpt-5", "https://openai.com/news/gpt-5"},
		{"uppercase scheme and host", "HTTPS://OpenAI.com/News", "https://openai.com/News"},
		{"path case is preserved", "https://openai.com/News/GPT-5", "https://openai.com/News/GPT-5"},

		{"trailing slash", "https://openai.com/news/", "https://openai.com/news"},
		{"root slash", "https://openai.com/", "https://openai.com"},
		{"bare host", "openai.com", "https://openai.com"},
		{"protocol relative", "//openai.com/news", "https://openai.com/news"},
		{"duplicate slashes", "https://openai.com//news//gpt-5", "https://openai.com/news/gpt-5"},

		{"default https port", "https://openai.com:443/news", "https://openai.com/news"},
		{"default http port", "http://openai.com:80/news", "http://openai.com/news"},
		{"non default port survives", "http://localhost:8080/news", "http://localhost:8080/news"},

		{"fragment", "https://openai.com/news#section-2", "https://openai.com/news"},
		{"userinfo", "https://user:pass@openai.com/news", "https://openai.com/news"},

		{"utm params", "https://openai.com/news?utm_source=x&utm_medium=y", "https://openai.com/news"},
		{"ref param", "https://openai.com/news?ref=hn", "https://openai.com/news"},
		{"source param", "https://openai.com/news?source=rss", "https://openai.com/news"},
		{"fbclid", "https://openai.com/news?fbclid=abc123", "https://openai.com/news"},
		{"mixed tracking and real", "https://x.com/a?id=7&utm_source=n&ref=hn", "https://x.com/a?id=7"},
		{"tracking param case insensitive", "https://openai.com/news?UTM_Source=x", "https://openai.com/news"},

		{"query order is stable", "https://x.com/a?b=2&a=1", "https://x.com/a?a=1&b=2"},
		{"meaningful params survive", "https://youtube.com/watch?v=abc", "https://youtube.com/watch?v=abc"},

		{"www is preserved", "https://www.theverge.com/ai", "https://www.theverge.com/ai"},

		{"google news wrapper", "https://news.google.com/rss/articles/x?url=https://openai.com/news", "https://openai.com/news"},
		{"reddit outbound wrapper", "https://out.reddit.com/r/x?url=https://arxiv.org/abs/2401.12345", "https://arxiv.org/abs/2401.12345"},
		{"facebook wrapper", "https://l.facebook.com/l.php?u=https://openai.com/news", "https://openai.com/news"},
		{"wrapper strips tracking from target", "https://news.google.com/rss/articles/x?url=https://openai.com/news%3Futm_source%3Dn", "https://openai.com/news"},
		{"wrapper without a target is left alone", "https://news.google.com/rss/articles/xyz", "https://news.google.com/rss/articles/xyz"},

		{"whitespace is trimmed", "  https://openai.com/news  ", "https://openai.com/news"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CanonicalURL(tt.in)
			if err != nil {
				t.Fatalf("CanonicalURL(%q) = error %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("CanonicalURL(%q)\n got %q\nwant %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCanonicalURLErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"no host", "https://"},
		{"ftp scheme", "ftp://example.com/file"},
		{"mailto", "mailto:someone@example.com"},
		{"javascript", "javascript:alert(1)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := CanonicalURL(tt.in); err == nil {
				t.Errorf("CanonicalURL(%q) = %q, want an error", tt.in, got)
			}
		})
	}
}

// Two URLs that differ only in tracking must produce one identity, which is
// what makes cross-feed deduplication work.
func TestCanonicalURLCollapsesVariants(t *testing.T) {
	variants := []string{
		"https://openai.com/news/gpt-5",
		"https://openai.com/news/gpt-5/",
		"https://OpenAI.com/news/gpt-5",
		"https://openai.com/news/gpt-5?utm_source=hn&utm_campaign=x",
		"https://openai.com/news/gpt-5#intro",
		"https://openai.com:443/news/gpt-5",
		"HTTPS://openai.com//news//gpt-5/",
	}

	want, err := CanonicalURL(variants[0])
	if err != nil {
		t.Fatal(err)
	}
	wantID := ItemID(want)

	for _, v := range variants {
		got, err := CanonicalURL(v)
		if err != nil {
			t.Fatalf("CanonicalURL(%q) = error %v", v, err)
		}
		if got != want {
			t.Errorf("CanonicalURL(%q) = %q, want %q", v, got, want)
		}
		if id := ItemID(got); id != wantID {
			t.Errorf("ItemID for %q = %q, want %q", v, id, wantID)
		}
	}
}

// A wrapper that points at itself must not recurse forever.
func TestCanonicalURLBoundsWrapperRecursion(t *testing.T) {
	self := "https://news.google.com/rss/articles/x?url=https%3A%2F%2Fnews.google.com%2Frss%2Farticles%2Fx"

	done := make(chan struct{})
	go func() {
		defer close(done)
		if _, err := CanonicalURL(self); err != nil {
			t.Errorf("CanonicalURL() = %v, want nil", err)
		}
	}()

	<-done
}

func TestItemID(t *testing.T) {
	const url = "https://openai.com/news/gpt-5"

	id := ItemID(url)
	if len(id) != 16 {
		t.Errorf("ItemID(%q) = %q, want 16 characters", url, id)
	}
	if id != ItemID(url) {
		t.Error("ItemID is not deterministic")
	}
	if id == ItemID(url+"-2") {
		t.Error("different URLs produced the same ID")
	}
	for _, r := range id {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("ItemID(%q) = %q, want lowercase hex", url, id)
		}
	}
}
