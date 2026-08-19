package embed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/papitsho/airfoil/internal/store"
)

// Cached wraps an Embedder with a disk cache.
//
// The cache file lives under data/cache/, which is gitignored (R5): vectors are
// large and fully regenerable. Entries are keyed by provider, model, and text
// hash, so switching models never mixes vector spaces.
type Cached struct {
	inner   Embedder
	path    string
	entries map[string]cacheEntry
	maxAge  time.Duration
	now     func() time.Time

	Hits, Misses int
}

type cacheEntry struct {
	Vector []float32 `json:"v"`
	At     time.Time `json:"at"`
}

type cacheFile struct {
	Entries map[string]cacheEntry `json:"entries"`
}

// NewCached loads the cache from disk. A missing or unreadable cache is not an
// error — it just means everything is a miss.
func NewCached(inner Embedder, path string, maxAgeDays int) *Cached {
	c := &Cached{
		inner:   inner,
		path:    path,
		entries: map[string]cacheEntry{},
		maxAge:  time.Duration(maxAgeDays) * 24 * time.Hour,
		now:     time.Now,
	}

	if f, err := store.ReadJSON[cacheFile](path); err == nil && f.Entries != nil {
		c.entries = f.Entries
	}
	return c
}

func (c *Cached) Name() string    { return c.inner.Name() }
func (c *Cached) Dimensions() int { return c.inner.Dimensions() }

// Embed returns vectors for texts, calling the underlying provider only for
// those not already cached.
func (c *Cached) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	cutoff := c.now().Add(-c.maxAge)

	// Distinct texts only: a run often contains the same title twice.
	missingAt := map[string][]int{}
	var missing []string

	for i, text := range texts {
		key := c.key(text)
		if e, ok := c.entries[key]; ok && e.At.After(cutoff) && len(e.Vector) > 0 {
			out[i] = e.Vector
			c.Hits++
			continue
		}
		c.Misses++
		if _, queued := missingAt[text]; !queued {
			missing = append(missing, text)
		}
		missingAt[text] = append(missingAt[text], i)
	}

	if len(missing) == 0 {
		return out, nil
	}

	vectors, err := c.inner.Embed(ctx, missing)
	if err != nil {
		return nil, err
	}
	if len(vectors) != len(missing) {
		return nil, fmt.Errorf("embed: provider returned %d vectors for %d texts", len(vectors), len(missing))
	}

	at := c.now().UTC()
	for i, text := range missing {
		c.entries[c.key(text)] = cacheEntry{Vector: vectors[i], At: at}
		for _, idx := range missingAt[text] {
			out[idx] = vectors[i]
		}
	}
	return out, nil
}

// Save prunes expired entries and writes the cache back to disk.
func (c *Cached) Save() error {
	cutoff := c.now().Add(-c.maxAge)
	for key, e := range c.entries {
		if !e.At.After(cutoff) {
			delete(c.entries, key)
		}
	}
	return store.WriteJSON(c.path, cacheFile{Entries: c.entries})
}

// key namespaces the text hash by provider and model, so that vectors from two
// providers can never be compared against each other.
func (c *Cached) key(text string) string {
	sum := sha256.Sum256([]byte(c.inner.Name() + "\x00" + text))
	return hex.EncodeToString(sum[:])[:32]
}
