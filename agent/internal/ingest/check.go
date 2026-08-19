package ingest

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/papitsho/airfoil/internal/config"
)

// Check is the result of probing one source URL.
type Check struct {
	Source config.Source
	Status int
	Took   time.Duration
	Err    error
	// Via is the index of the candidate URL that answered. Anything above 0
	// means the source only works through a fallback path.
	Via int
}

// Fallback reports whether the source answered only on a secondary URL.
func (c Check) Fallback() bool { return c.OK() && c.Via > 0 }

// OK reports whether the source answered with a success status.
func (c Check) OK() bool {
	return c.Err == nil && c.Status >= 200 && c.Status < 300
}

// CheckSources probes every source's URL concurrently and reports what each one
// answered. This is how a feed that has quietly started 404ing gets found —
// the ingest run itself only logs a warning and carries on (R8).
//
// It deliberately issues a GET rather than a HEAD: several feed hosts answer
// HEAD with 405 while serving GET perfectly well, which would report a healthy
// feed as broken.
func CheckSources(ctx context.Context, cfg *config.Config, sources []config.Source) []Check {
	checks := make([]Check, len(sources))

	// Same bounds the real run uses, so a source that passes the probe is one
	// ingest could actually have fetched.
	client := &http.Client{Timeout: sourceTimeout}
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrent)

	for i, src := range sources {
		i, src := i, src
		g.Go(func() error {
			checks[i] = probe(ctx, client, cfg, src)
			return nil // a dead source is a finding, not a failure
		})
	}
	_ = g.Wait()

	sort.SliceStable(checks, func(a, b int) bool {
		if checks[a].Source.Tier != checks[b].Source.Tier {
			return checks[a].Source.Tier < checks[b].Source.Tier
		}
		return checks[a].Source.ID < checks[b].Source.ID
	})
	return checks
}

// The return is named so the deferred timing assignment lands in the returned
// value. With a bare `Check` return, the defer would fire after the value was
// already copied and every duration would report as zero.
func probe(ctx context.Context, client *http.Client, cfg *config.Config, src config.Source) (c Check) {
	c = Check{Source: src}
	started := time.Now()
	defer func() { c.Took = time.Since(started) }()

	// Probe every URL the adapter would accept, in the adapter's own order. A
	// probe of the bare configured URL reports healthy sources as broken:
	// the GitHub search base 422s without a query, and Reddit's JSON 403s on a
	// path the adapter recovers from by falling back to RSS.
	for i, target := range probeURLs(src) {
		c.Status, c.Err = fetchOnce(ctx, client, cfg, src, target)
		if c.OK() {
			c.Via = i // 0 is the primary path
			return c
		}
	}
	return c
}

// probeURLs returns the candidate URLs for a source, mirroring what the adapter
// for that type actually requests.
func probeURLs(src config.Source) []string {
	switch src.Type {
	case config.SourceGitHub:
		// github.go always appends a query; the bare search endpoint is a 422.
		return []string{src.URL + "?q=" + url.QueryEscape("language:python stars:>100") + "&per_page=1"}

	case config.SourceReddit:
		// reddit.go tries JSON, then falls back to the public RSS listing.
		return []string{src.URL, strings.TrimSuffix(src.URL, ".json") + "/.rss"}

	default:
		return []string{src.URL}
	}
}

func fetchOnce(ctx context.Context, client *http.Client, cfg *config.Config, src config.Source, target string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return 0, fmt.Errorf("bad url: %w", err)
	}

	ua := defaultUserAgent
	if src.Type == config.SourceReddit && cfg.Ingest.RedditUserAgent != "" {
		ua = cfg.Ingest.RedditUserAgent
	}
	req.Header.Set("User-Agent", ua)

	// Accept-Encoding is deliberately unset — see the comment in http.go.
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Read and discard: a server can answer 200 and then fail mid-body, and a
	// probe that never reads would call that healthy.
	if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyBytes)); err != nil {
		return resp.StatusCode, fmt.Errorf("read body: %w", err)
	}
	return resp.StatusCode, nil
}
