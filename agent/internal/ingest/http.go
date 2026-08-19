package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

// maxBodyBytes bounds what a single source can make us read into memory. Feeds
// are text; anything past this is a misconfigured URL.
const maxBodyBytes = 16 << 20 // 16 MiB

// fetcher performs the HTTP requests every adapter needs, with one retry on the
// status codes that mean "try again" (R8).
type fetcher struct {
	client    *http.Client
	userAgent string
}

func newFetcher(timeout time.Duration, userAgent string) *fetcher {
	return &fetcher{
		client:    &http.Client{Timeout: timeout},
		userAgent: userAgent,
	}
}

// get returns the response body for url. headers may be nil.
func (f *fetcher) get(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	var lastErr error

	for attempt := range 2 {
		if attempt > 0 {
			// Jittered backoff so that several sources hitting the same host
			// do not retry in lockstep.
			delay := 500*time.Millisecond + time.Duration(rand.N(500))*time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		body, retryable, err := f.tryGet(ctx, url, headers)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retryable {
			return nil, err
		}
	}
	return nil, lastErr
}

// tryGet performs one request. retryable reports whether another attempt could
// plausibly succeed.
func (f *fetcher) tryGet(ctx context.Context, url string, headers map[string]string) (body []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("request %s: %w", url, err)
	}
	// Accept-Encoding is deliberately not set: the transport adds gzip itself
	// and decompresses transparently, but only while the header is unset. Set
	// it by hand and every response arrives still compressed.
	req.Header.Set("User-Agent", f.userAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		// A refused connection or a timeout is worth one more attempt, but a
		// cancelled context is not.
		return nil, ctx.Err() == nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests, resp.StatusCode >= 500:
		return nil, true, fmt.Errorf("get %s: %s", url, resp.Status)
	case resp.StatusCode != http.StatusOK:
		return nil, false, fmt.Errorf("get %s: %s", url, resp.Status)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, true, fmt.Errorf("read %s: %w", url, err)
	}
	return b, false, nil
}

// getJSON fetches url and decodes the response into out.
func (f *fetcher) getJSON(ctx context.Context, url string, headers map[string]string, out any) error {
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["Accept"]; !ok {
		headers["Accept"] = "application/json"
	}

	body, err := f.get(ctx, url, headers)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}
