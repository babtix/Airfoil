package ingest

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetcherGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	}))
	defer srv.Close()

	got, err := newFetcher(5*time.Second, "test").get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("get() = %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("body = %q, want %q", got, "hello")
	}
}

func TestFetcherSendsUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	const want = "airfoil/0.1 (+https://example.com)"
	if _, err := newFetcher(5*time.Second, want).get(context.Background(), srv.URL, nil); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("User-Agent = %q, want %q", got, want)
	}
}

func TestFetcherCustomHeadersOverrideDefaults(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
	}))
	defer srv.Close()

	f := newFetcher(5*time.Second, "default-agent")
	_, err := f.get(context.Background(), srv.URL, map[string]string{"User-Agent": "reddit-agent"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "reddit-agent" {
		t.Errorf("User-Agent = %q, want the per-request override", got)
	}
}

// Setting Accept-Encoding by hand disables the transport's transparent
// decompression, which silently hands every adapter gzip bytes to parse.
func TestFetcherDecompressesGzip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if enc := r.Header.Get("Accept-Encoding"); enc != "gzip" {
			t.Errorf("Accept-Encoding = %q, want the transport default of gzip", enc)
		}
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gz.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	got, err := newFetcher(5*time.Second, "test").get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("get() = %v", err)
	}
	if string(got) != `{"ok":true}` {
		t.Errorf("body = %q, want decompressed JSON", got)
	}
}

func TestFetcherRetries(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		wantAttempts int32
		wantErr      bool
	}{
		{"retries on 429", http.StatusTooManyRequests, 2, false},
		{"retries on 500", http.StatusInternalServerError, 2, false},
		{"retries on 503", http.StatusServiceUnavailable, 2, false},
		{"does not retry on 404", http.StatusNotFound, 1, true},
		{"does not retry on 403", http.StatusForbidden, 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Fail once, then succeed, so a retry is observable.
				if atomic.AddInt32(&attempts, 1) == 1 {
					w.WriteHeader(tt.status)
					return
				}
				w.Write([]byte("recovered"))
			}))
			defer srv.Close()

			body, err := newFetcher(5*time.Second, "test").get(context.Background(), srv.URL, nil)

			if got := atomic.LoadInt32(&attempts); got != tt.wantAttempts {
				t.Errorf("made %d attempts, want %d", got, tt.wantAttempts)
			}
			if tt.wantErr {
				if err == nil {
					t.Error("get() = nil, want an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("get() = %v, want the retry to succeed", err)
			}
			if string(body) != "recovered" {
				t.Errorf("body = %q, want %q", body, "recovered")
			}
		})
	}
}

// A source that never recovers must give up rather than retry forever.
func TestFetcherGivesUpAfterOneRetry(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := newFetcher(5*time.Second, "test").get(context.Background(), srv.URL, nil); err == nil {
		t.Fatal("get() = nil, want an error")
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Errorf("made %d attempts, want 2", got)
	}
}

func TestFetcherRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if _, err := newFetcher(5*time.Second, "test").get(ctx, srv.URL, nil); err == nil {
		t.Fatal("get() = nil, want a timeout error")
	}
}

func TestFetcherGetJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name":"airfoil","count":3}`))
	}))
	defer srv.Close()

	var got struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	if err := newFetcher(5*time.Second, "test").getJSON(context.Background(), srv.URL, nil, &got); err != nil {
		t.Fatalf("getJSON() = %v", err)
	}
	if got.Name != "airfoil" || got.Count != 3 {
		t.Errorf("decoded %+v, want {airfoil 3}", got)
	}
}

func TestFetcherGetJSONRejectsGarbage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>not json</html>"))
	}))
	defer srv.Close()

	var out map[string]any
	if err := newFetcher(5*time.Second, "test").getJSON(context.Background(), srv.URL, nil, &out); err == nil {
		t.Fatal("getJSON() = nil, want a decode error")
	}
}
