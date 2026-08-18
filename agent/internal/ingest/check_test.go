package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/papitsho/airfoil/internal/config"
)

func TestProbeURLsMirrorAdapters(t *testing.T) {
	tests := []struct {
		name  string
		src   config.Source
		want  int
		check func(t *testing.T, urls []string)
	}{
		{
			name: "rss probes the configured url only",
			src:  config.Source{Type: config.SourceRSS, URL: "https://example.com/feed.xml"},
			want: 1,
			check: func(t *testing.T, urls []string) {
				if urls[0] != "https://example.com/feed.xml" {
					t.Errorf("got %q", urls[0])
				}
			},
		},
		{
			name: "github appends a query because the bare endpoint 422s",
			src:  config.Source{Type: config.SourceGitHub, URL: "https://api.github.com/search/repositories"},
			want: 1,
			check: func(t *testing.T, urls []string) {
				if !strings.Contains(urls[0], "q=") {
					t.Errorf("no query appended: %q", urls[0])
				}
			},
		},
		{
			name: "reddit falls back to the rss listing",
			src:  config.Source{Type: config.SourceReddit, URL: "https://www.reddit.com/r/LocalLLaMA/hot.json"},
			want: 2,
			check: func(t *testing.T, urls []string) {
				if urls[1] != "https://www.reddit.com/r/LocalLLaMA/hot/.rss" {
					t.Errorf("fallback = %q", urls[1])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := probeURLs(tt.src)
			if len(got) != tt.want {
				t.Fatalf("len = %d, want %d (%v)", len(got), tt.want, got)
			}
			tt.check(t, got)
		})
	}
}

func TestCheckSourcesReportsStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/ok"):
			w.Write([]byte("<rss/>"))
		case strings.HasPrefix(r.URL.Path, "/gone"):
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	cfg := &config.Config{}
	sources := []config.Source{
		{ID: "good", Tier: 1, Type: config.SourceRSS, URL: srv.URL + "/ok"},
		{ID: "dead", Tier: 4, Type: config.SourceRSS, URL: srv.URL + "/gone"},
	}

	checks := CheckSources(context.Background(), cfg, sources)
	if len(checks) != 2 {
		t.Fatalf("len = %d, want 2", len(checks))
	}

	// Sorted by tier, so the tier-1 source comes first.
	if !checks[0].OK() {
		t.Errorf("good source: OK() = false, status %d err %v", checks[0].Status, checks[0].Err)
	}
	if checks[0].Took <= 0 {
		t.Error("Took not recorded — the deferred timing assignment is not reaching the caller")
	}
	if checks[1].OK() {
		t.Error("dead source reported OK")
	}
	if checks[1].Status != http.StatusNotFound {
		t.Errorf("dead status = %d, want 404", checks[1].Status)
	}
}

func TestCheckSourcesUsesFallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Mirrors Reddit: JSON is forbidden, the RSS listing is public.
		if strings.HasSuffix(r.URL.Path, "/.rss") {
			w.Write([]byte("<rss/>"))
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	cfg := &config.Config{}
	sources := []config.Source{
		{ID: "reddit-x", Tier: 5, Type: config.SourceReddit, URL: srv.URL + "/r/x/hot.json"},
	}

	got := CheckSources(context.Background(), cfg, sources)[0]

	if !got.OK() {
		t.Fatalf("OK() = false, status %d err %v", got.Status, got.Err)
	}
	if !got.Fallback() {
		t.Errorf("Fallback() = false, want true (Via = %d)", got.Via)
	}
}

func TestCheckSourcesUnreachableHost(t *testing.T) {
	cfg := &config.Config{}
	// Reserved TEST-NET-1 address; nothing answers here.
	sources := []config.Source{
		{ID: "nowhere", Type: config.SourceRSS, URL: "http://192.0.2.1:9/feed"},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // fail fast rather than waiting out the dial timeout

	got := CheckSources(ctx, cfg, sources)[0]
	if got.OK() {
		t.Error("unreachable host reported OK")
	}
	if got.Err == nil {
		t.Error("Err = nil, want a transport error")
	}
}
