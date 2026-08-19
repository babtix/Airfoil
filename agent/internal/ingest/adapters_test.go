package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/papitsho/airfoil/internal/config"
)

func testFetcher() *fetcher { return newFetcher(5*time.Second, "test") }

// serve returns a server whose handler is chosen by request path.
func serve(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

const atomFeed = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Example Lab</title>
  <entry>
    <title>Model 5 released with a longer context window</title>
    <link href="https://example.com/news/model-5?utm_source=feed"/>
    <published>2026-08-17T09:00:00Z</published>
    <author><name>Research Team</name></author>
    <content type="html">&lt;p&gt;Weights at https://github.com/example/model-5&lt;/p&gt;</content>
  </entry>
  <entry>
    <title>An entry with no link</title>
    <published>2026-08-17T08:00:00Z</published>
  </entry>
</feed>`

func TestRSSAdapter(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Write([]byte(atomFeed))
	})

	src := config.Source{ID: "lab", Name: "Example Lab", Type: config.SourceRSS, URL: srv.URL, Tier: 1}
	got, err := newRSSAdapter(testFetcher()).Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch() = %v", err)
	}

	// The linkless entry cannot be an item.
	if len(got) != 1 {
		t.Fatalf("got %d raws, want 1", len(got))
	}
	raw := got[0]
	if raw.SourceID != "lab" || raw.SourceTier != 1 {
		t.Errorf("source = %q tier %d, want lab tier 1", raw.SourceID, raw.SourceTier)
	}
	if !strings.Contains(raw.Title, "Model 5") {
		t.Errorf("Title = %q", raw.Title)
	}
	if raw.Author != "Research Team" {
		t.Errorf("Author = %q, want %q", raw.Author, "Research Team")
	}
	if want := time.Date(2026, 8, 17, 9, 0, 0, 0, time.UTC); !raw.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v, want %v", raw.PublishedAt, want)
	}
	if !strings.Contains(raw.Body, "github.com/example/model-5") {
		t.Errorf("Body = %q, want the content element", raw.Body)
	}
}

func TestRSSAdapterRejectsNonFeed(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html><body>not a feed</body></html>"))
	})

	src := config.Source{ID: "lab", Name: "Lab", URL: srv.URL, Tier: 1}
	if _, err := newRSSAdapter(testFetcher()).Fetch(context.Background(), src); err == nil {
		t.Fatal("Fetch() = nil, want a parse error")
	}
}

func TestHNAdapter(t *testing.T) {
	var queries []string
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Get("query"))
		w.Write([]byte(`{"hits":[
			{"objectID":"1","title":"Llama 5 open weights","url":"https://example.com/llama-5",
			 "author":"pg","points":420,"num_comments":88,"created_at_i":1786000000},
			{"objectID":"2","title":"Ask HN: best local model?","url":"",
			 "author":"dang","story_text":"Looking for recommendations","points":95,"num_comments":40,"created_at_i":1786000100}
		]}`))
	})

	src := config.Source{
		ID: "hn", Name: "Hacker News", Type: config.SourceHN, URL: srv.URL, Tier: 5,
		Options: config.SourceOptions{Queries: []string{"LLM", "open weights"}, MinPoints: 50, HoursBack: 48},
	}

	got, err := newHNAdapter(testFetcher()).Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch() = %v", err)
	}

	// Two queries ran, but a submission matching both appears once.
	if len(queries) != 2 {
		t.Errorf("ran %d queries, want 2", len(queries))
	}
	if len(got) != 2 {
		t.Fatalf("got %d raws, want 2 deduplicated submissions", len(got))
	}

	if got[0].Metrics.HNPoints != 420 || got[0].Metrics.HNComments != 88 {
		t.Errorf("metrics = %+v, want 420 points / 88 comments", got[0].Metrics)
	}
	if got[0].URL != "https://example.com/llama-5" {
		t.Errorf("URL = %q, want the outbound link", got[0].URL)
	}
	// A text post has no outbound link, so it points at the discussion.
	if want := "https://news.ycombinator.com/item?id=2"; got[1].URL != want {
		t.Errorf("URL = %q, want the HN permalink %q", got[1].URL, want)
	}
}

func TestHNAdapterRequiresQueries(t *testing.T) {
	src := config.Source{ID: "hn", Type: config.SourceHN, URL: "https://example.com", Tier: 5}
	if _, err := newHNAdapter(testFetcher()).Fetch(context.Background(), src); err == nil {
		t.Fatal("Fetch() = nil, want an error about missing queries")
	}
}

const redditJSON = `{"data":{"children":[
	{"data":{"id":"a","title":"New 7B model beats GPT-4","url":"https://example.com/model",
	         "permalink":"/r/x/a","author":"u1","score":900,"num_comments":120,"created_utc":1786000000}},
	{"data":{"id":"b","title":"Weekly discussion thread","url":"","permalink":"/r/x/b",
	         "author":"mod","score":5000,"created_utc":1786000100,"is_self":true,"stickied":true}},
	{"data":{"id":"c","title":"Low effort question","url":"https://example.com/q",
	         "permalink":"/r/x/c","author":"u2","score":3,"created_utc":1786000200}},
	{"data":{"id":"d","title":"My fine-tune results","url":"","permalink":"/r/x/d",
	         "author":"u3","selftext":"Details inside","score":300,"created_utc":1786000300,"is_self":true}}
]}}`

func TestRedditAdapterJSON(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(redditJSON))
	})

	src := config.Source{
		ID: "reddit-x", Name: "r/x", Type: config.SourceReddit, URL: srv.URL, Tier: 5,
		Options: config.SourceOptions{Limit: 50, MinScore: 100},
	}

	got, err := newRedditAdapter(testFetcher(), "airfoil/0.1 (by /u/tester)").Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch() = %v", err)
	}

	// The stickied thread and the below-threshold post are both dropped.
	if len(got) != 2 {
		t.Fatalf("got %d raws, want 2", len(got))
	}
	if got[0].URL != "https://example.com/model" {
		t.Errorf("URL = %q, want the outbound link", got[0].URL)
	}
	if got[0].Metrics.RedditScore != 900 {
		t.Errorf("RedditScore = %d, want 900", got[0].Metrics.RedditScore)
	}
	// A self post points at the discussion.
	if want := "https://www.reddit.com/r/x/d"; got[1].URL != want {
		t.Errorf("URL = %q, want %q", got[1].URL, want)
	}
}

// Reddit answers 403 to unauthenticated JSON from many networks, so the public
// RSS listing has to carry the source rather than losing it entirely.
func TestRedditAdapterFallsBackToRSS(t *testing.T) {
	var rssHits int
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/.rss") {
			rssHits++
			w.Header().Set("Content-Type", "application/atom+xml")
			w.Write([]byte(atomFeed))
			return
		}
		w.WriteHeader(http.StatusForbidden)
	})

	src := config.Source{
		ID: "reddit-x", Name: "r/x", Type: config.SourceReddit, URL: srv.URL + "/r/x/hot.json", Tier: 5,
		Options: config.SourceOptions{MinScore: 100},
	}

	got, err := newRedditAdapter(testFetcher(), "airfoil/0.1 (by /u/tester)").Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch() = %v, want the RSS fallback to succeed", err)
	}
	if rssHits != 1 {
		t.Errorf("hit the RSS endpoint %d times, want 1", rssHits)
	}
	if len(got) != 1 {
		t.Fatalf("got %d raws, want 1", len(got))
	}
	// The feed carries no score, so min_score cannot be applied here.
	if got[0].Metrics.RedditScore != 0 {
		t.Errorf("RedditScore = %d, want 0 from the RSS path", got[0].Metrics.RedditScore)
	}
}

func TestRedditAdapterFailsWhenBothPathsFail(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	src := config.Source{ID: "reddit-x", Name: "r/x", URL: srv.URL + "/r/x/hot.json", Tier: 5}
	_, err := newRedditAdapter(testFetcher(), "airfoil/0.1 (by /u/tester)").Fetch(context.Background(), src)
	if err == nil {
		t.Fatal("Fetch() = nil, want an error naming both attempts")
	}
	for _, want := range []string{"json", "rss"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %v, want it to mention %q", err, want)
		}
	}
}

func TestHFAdapter(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[
			{"paper":{"id":"2401.12345","title":"Scaling Laws Revisited","summary":"We show that...",
			          "upvotes":42,"publishedAt":"2026-08-17T00:00:00.000Z",
			          "authors":[{"name":"A. Researcher"}]}},
			{"paper":{"id":"2401.99999","title":"Barely noticed","summary":"x","upvotes":1,
			          "publishedAt":"2026-08-17T00:00:00.000Z"}}
		]`))
	})

	src := config.Source{
		ID: "hf-papers", Name: "HF Daily Papers", Type: config.SourceHFPapers, URL: srv.URL, Tier: 2,
		Options: config.SourceOptions{MinUpvotes: 5},
	}

	got, err := newHFAdapter(testFetcher()).Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch() = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d raws, want 1 after the upvote filter", len(got))
	}

	raw := got[0]
	if want := "https://huggingface.co/papers/2401.12345"; raw.URL != want {
		t.Errorf("URL = %q, want %q", raw.URL, want)
	}
	// The paper URL is known up front, which is what forces coverage of one
	// paper into a single cluster.
	if want := "https://arxiv.org/abs/2401.12345"; raw.PaperURL != want {
		t.Errorf("PaperURL = %q, want %q", raw.PaperURL, want)
	}
	if raw.Metrics.HFUpvotes != 42 {
		t.Errorf("HFUpvotes = %d, want 42", raw.Metrics.HFUpvotes)
	}
	if raw.Author != "A. Researcher" {
		t.Errorf("Author = %q", raw.Author)
	}
}

func TestGitHubAdapter(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("q"); !strings.Contains(got, "created:>") {
			t.Errorf("query %q, want a created: bound", got)
		}
		w.Write([]byte(`{"items":[
			{"full_name":"acme/fastllm","html_url":"https://github.com/acme/fastllm",
			 "description":"Fast inference","stargazers_count":1200,
			 "created_at":"2026-08-15T00:00:00Z","owner":{"login":"acme"}}
		]}`))
	})

	src := config.Source{
		ID: "gh", Name: "GitHub Trending", Type: config.SourceGitHub, URL: srv.URL, Tier: 3,
		Options: config.SourceOptions{Queries: []string{"llm", "rag"}, MinStars: 100, MaxPerQuery: 5},
	}

	got, err := newGitHubAdapter(testFetcher(), "").Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch() = %v", err)
	}
	// The same repo matched both queries.
	if len(got) != 1 {
		t.Fatalf("got %d raws, want 1 deduplicated repo", len(got))
	}
	if got[0].RepoURL != "https://github.com/acme/fastllm" {
		t.Errorf("RepoURL = %q", got[0].RepoURL)
	}
	if got[0].Metrics.GitHubStars != 1200 {
		t.Errorf("GitHubStars = %d, want 1200", got[0].Metrics.GitHubStars)
	}
	if !strings.Contains(got[0].Title, "Fast inference") {
		t.Errorf("Title = %q, want the description folded in", got[0].Title)
	}
}

func TestGitHubAdapterSendsToken(t *testing.T) {
	var auth string
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		w.Write([]byte(`{"items":[]}`))
	})

	src := config.Source{
		ID: "gh", Name: "GitHub", URL: srv.URL, Tier: 3,
		Options: config.SourceOptions{Queries: []string{"llm"}},
	}
	if _, err := newGitHubAdapter(testFetcher(), "secret-token").Fetch(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	if auth != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want the bearer token", auth)
	}
}

// The unauthenticated search limit is low and every query shares one timeout,
// so a later query failing must not discard what the earlier ones returned.
func TestGitHubAdapterKeepsPartialResults(t *testing.T) {
	var calls int
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls > 1 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Write([]byte(`{"items":[
			{"full_name":"acme/first","html_url":"https://github.com/acme/first",
			 "description":"Found before the limit","stargazers_count":300,
			 "created_at":"2026-08-15T00:00:00Z","owner":{"login":"acme"}}
		]}`))
	})

	src := config.Source{
		ID: "gh", Name: "GitHub", URL: srv.URL, Tier: 3,
		Options: config.SourceOptions{Queries: []string{"llm", "rag", "agents"}},
	}

	got, err := newGitHubAdapter(testFetcher(), "").Fetch(context.Background(), src)
	if err != nil {
		t.Fatalf("Fetch() = %v, want partial success", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d raws, want the 1 result from the query that succeeded", len(got))
	}
}

func TestGitHubAdapterFailsWhenEveryQueryFails(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	src := config.Source{
		ID: "gh", Name: "GitHub", URL: srv.URL, Tier: 3,
		Options: config.SourceOptions{Queries: []string{"llm", "rag"}},
	}
	if _, err := newGitHubAdapter(testFetcher(), "").Fetch(context.Background(), src); err == nil {
		t.Fatal("Fetch() = nil, want an error when no query succeeded")
	}
}
