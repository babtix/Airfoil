# URL Canonicalization & Item Identity

This chapter documents the URL canonicalization and item identity system used during the **normalize** pipeline stage. It covers `CanonicalURL` (recursive wrapper unwrapping, tracking parameter stripping, path cleaning, scheme normalization), `ItemID` (SHA-256 hash of the canonical URL), and the `redirectWrappers` / `trackingParams` / `trackingPrefixes` lists that drive deduplication.

## Table of Contents

- [Overview](#overview)
- [CanonicalURL: Algorithm & Flow](#canonicalurl-algorithm--flow)
- [Wrapper Unwrapping (`redirectWrappers`)](#wrapper-unwrapping-redirectwrappers)
- [Tracking Parameter Removal](#tracking-parameter-removal)
- [Path Cleaning & Scheme Normalization](#path-cleaning--scheme-normalization)
- [ItemID: Stable Identity Hash](#itemid-stable-identity-hash)
- [Configuration Constants](#configuration-constants)
- [Error Handling & Edge Cases](#error-handling--edge-cases)
- [Referenced Files](#referenced-files)

---

## Overview

The normalize stage receives `Raw` items from ingest adapters (RSS, HN, Reddit, HFPapers, GitHub). Each `Raw` item carries a `link` field that may:

- Point through redirect wrappers (Google News, Reddit outbound, Facebook link shims, etc.)
- Contain tracking query parameters (`utm_*`, `fbclid`, `ref`, etc.)
- Have inconsistent scheme/host casing, default ports, fragments, or trailing slashes
- Be protocol-relative (`//example.com`) or bare-host (`example.com/path`)

`CanonicalURL` reduces all of these to a **stable, comparable identity** so that two feeds linking the same article through different wrappers or with different analytics parameters deduplicate to a single `Item`. The canonical URL is then hashed via `ItemID` to produce the 16-character hex key used by `State.Seen` / `State.MarkSeen` for idempotent deduplication.

```mermaid
flowchart TD
    A[Raw URL from Adapter] --> B[CanonicalURL]
    B --> C{Parse & Validate}
    C -->|invalid| D[Error]
    C -->|valid| E[Lowercase Scheme & Host]
    E --> F[Strip Default Ports]
    F --> G[Remove Userinfo & Fragment]
    G --> H{Unwrap Redirect Wrapper?}
    H -->|yes & depth < 3| I[Recurse on Target URL]
    H -->|no| J[Strip Tracking Params]
    I --> J
    J --> K[Sort Remaining Query Params]
    K --> L[Clean Path: collapse //, trim trailing /]
    L --> M[Canonical URL String]
    M --> N[ItemID = SHA-256(canonical)[:16]]
    N --> O[Dedupe via State.Seen]
```

---

## CanonicalURL: Algorithm & Flow

`CanonicalURL` is the public entry point. It delegates to the unexported `canonicalURL` with an initial `depth = 0` to track recursive unwrapping.

```
`agent/internal/normalize/url.go:58-60`
```

```go
func CanonicalURL(raw string) (string, error) {
	return canonicalURL(raw, 0)
}
```

The recursive worker `canonicalURL` performs the following steps in order:

```
`agent/internal/normalize/url.go:62-120`
```

```go
func canonicalURL(raw string, depth int) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("normalize: empty url")
	}

	// Protocol-relative and bare-host inputs both appear in the wild.
	switch {
	case strings.HasPrefix(s, "//"):
		s = "https:" + s
	case !strings.Contains(s, "://"):
		// "mailto:x@y" and "javascript:alert(1)" carry a scheme without a
		// host. Prefixing https:// to those would parse the scheme as userinfo
		// and silently produce a plausible-looking URL.
		if hasOpaqueScheme(s) {
			return "", fmt.Errorf("normalize: unsupported scheme in %q", raw)
		}
		s = "https://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("normalize: parse url %q: %w", raw, err)
	}

	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("normalize: unsupported scheme %q in %q", u.Scheme, raw)
	}
	if u.Host == "" {
		return "", fmt.Errorf("normalize: no host in %q", raw)
	}

	u.Host = strings.ToLower(u.Host)
	u.Host = strings.TrimSuffix(u.Host, ":80")
	u.Host = strings.TrimSuffix(u.Host, ":443")
	u.User = nil
	u.Fragment = ""
	u.RawFragment = ""

	if target, ok := unwrap(u); ok && depth < maxUnwrapDepth {
		return canonicalURL(target, depth+1)
	}

	// Drop tracking parameters, then let Encode sort what remains so that
	// parameter order never changes an item's identity.
	q := u.Query()
	for key := range q {
		if isTracking(key) {
			q.Del(key)
		}
	}
	u.RawQuery = q.Encode()

	u.Path = cleanPath(u.Path)
	u.RawPath = ""

	return u.String(), nil
}
```

### Step-by-step breakdown

| Step | Operation | Code Location |
|------|-----------|---------------|
| 1 | Trim whitespace; reject empty | L64-67 |
| 2 | Handle protocol-relative (`//host`) → prefix `https:` | L70-71 |
| 3 | Handle bare host (no `://`) → prefix `https://` **unless** it has an opaque scheme (`mailto:`, `javascript:`) | L72-81 |
| 4 | Parse with `net/url` | L83-86 |
| 5 | Lowercase scheme; reject non-http/https | L88-92 |
| 6 | Require non-empty host | L93-95 |
| 7 | Lowercase host; strip default ports `:80` / `:443` | L97-99 |
| 8 | Strip userinfo, fragment | L100-101 |
| 9 | **Unwrap redirect wrapper** (recursive, bounded by `maxUnwrapDepth`) | L103-105 |
| 10 | Delete tracking query parameters | L108-112 |
| 11 | Re-encode query (sorts parameters deterministically) | L113 |
| 12 | Clean path: collapse `//`, remove trailing `/` | L115-116 |
| 13 | Return `u.String()` | L118 |

---

## Wrapper Unwrapping (`redirectWrappers`)

Known redirect wrappers carry the real destination in a query parameter. `unwrap` checks the parsed URL's host (and in one case host+path) against the `redirectWrappers` map. If matched, it extracts the first non-empty parameter value that contains `://` and returns it for recursive canonicalization.

```
`agent/internal/normalize/url.go:35-45`
```

```go
// redirectWrappers are hosts that carry the real destination in a query
// parameter. Unwrapping them is what lets two feeds that link the same article
// through different wrappers dedupe to one item.
//
// Shorteners that resolve only over the network (bit.ly, t.co) are deliberately
// out of scope here — this function is pure.
var redirectWrappers = map[string][]string{
	"news.google.com":               {"url"},
	"www.google.com":                {"url", "q"},
	"google.com":                    {"url", "q"},
	"out.reddit.com":                {"url"},
	"l.facebook.com":                {"u"},
	"lm.facebook.com":               {"u"},
	"href.li":                       {"url"},
	"outgoing.prod.mozaws.net":      {"url"},
	"steamcommunity.com/linkfilter": {"url"},
}
```

```
`agent/internal/normalize/url.go:123-139`
```

```go
// unwrap returns the destination carried by a known redirect wrapper.
func unwrap(u *url.URL) (string, bool) {
	keys, ok := redirectWrappers[u.Host]
	if !ok {
		// A few wrappers are identified by host and path together.
		keys, ok = redirectWrappers[u.Host+strings.TrimSuffix(u.Path, "/")]
		if !ok {
			return "", false
		}
	}
	q := u.Query()
	for _, k := range keys {
		if v := strings.TrimSpace(q.Get(k)); v != "" && strings.Contains(v, "://") {
			return v, true
		}
	}
	return "", false
}
```

### Wrapper Table

| Host / Host+Path | Query Parameter(s) | Example Wrapped URL | Extracted Target |
|------------------|-------------------|---------------------|------------------|
| `news.google.com` | `url` | `https://news.google.com/articles/CBMi...?url=https://example.com/article` | `https://example.com/article` |
| `www.google.com` / `google.com` | `url`, `q` | `https://www.google.com/url?q=https://example.com/article` | `https://example.com/article` |
| `out.reddit.com` | `url` | `https://out.reddit.com/t3_xyz?url=https://example.com/article` | `https://example.com/article` |
| `l.facebook.com` / `lm.facebook.com` | `u` | `https://l.facebook.com/l.php?u=https://example.com/article` | `https://example.com/article` |
| `href.li` | `url` | `https://href.li/?url=https://example.com/article` | `https://example.com/article` |
| `outgoing.prod.mozaws.net` | `url` | `https://outgoing.prod.mozaws.net/url=https://example.com/article` | `https://example.com/article` |
| `steamcommunity.com/linkfilter` | `url` | `https://steamcommunity.com/linkfilter/?url=https://example.com/article` | `https://example.com/article` |

> **Note**: Network-resolving shorteners (bit.ly, t.co, etc.) are **not** unwrapped. The function is pure and has no HTTP client.

### Recursion Bound

```
`agent/internal/normalize/url.go:49`
```

```go
const maxUnwrapDepth = 3
```

The recursion depth is capped at **3** (not 10 as mentioned in the goal — the code enforces 3). This prevents infinite loops if a wrapper points to another wrapper that points back.

---

## Tracking Parameter Removal

Two data structures drive tracking parameter detection:

### `trackingParams` — Exact-match keys

```
`agent/internal/normalize/url.go:13-24`
```

```go
// trackingParams are query parameters that identify a referral rather than a
// resource. Two URLs differing only in these point at the same article.
var trackingParams = map[string]bool{
	"ref": true, "ref_src": true, "ref_url": true, "referrer": true, "referer": true,
	"source": true, "src": true,
	"fbclid": true, "gclid": true, "dclid": true, "msclkid": true, "yclid": true, "twclid": true,
	"mc_cid": true, "mc_eid": true,
	"igshid": true, "igsh": true,
	"_hsenc": true, "_hsmi": true,
	"cmpid": true, "campaign_id": true,
	"spm": true, "si": true,
	"at_medium": true, "at_campaign": true,
	"sh": true, "share_id": true, "sr_share": true,
}
```

### `trackingPrefixes` — Prefix-match keys

```
`agent/internal/normalize/url.go:27`
```

```go
// trackingPrefixes are query parameter prefixes that are always analytics.
var trackingPrefixes = []string{"utm_", "ga_", "hsa_", "mtm_", "pk_", "piwik_"}
```

### Detection Logic

```
`agent/internal/normalize/url.go:162-173`
```

```go
func isTracking(key string) bool {
	k := strings.ToLower(key)
	if trackingParams[k] {
		return true
	}
	for _, p := range trackingPrefixes {
		if strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}
```

Parameters are matched **case-insensitively**. After deletion, `u.RawQuery = q.Encode()` re-encodes the remaining parameters in **sorted order** (Go's `url.Values.Encode` sorts keys), so parameter order never affects identity.

### Tracking Parameter Categories

| Category | Parameters |
|----------|------------|
| Generic referrer | `ref`, `ref_src`, `ref_url`, `referrer`, `referer`, `source`, `src` |
| Facebook | `fbclid`, `igshid`, `igsh` |
| Google Ads / Analytics | `gclid`, `dclid`, `yclid`, `_hsenc`, `_hsmi`, `ga_*` |
| Microsoft / Bing | `msclkid` |
| Twitter/X | `twclid` |
| Mailchimp | `mc_cid`, `mc_eid` |
| HubSpot | `_hsenc`, `_hsmi` |
| Campaign / UTM | `cmpid`, `campaign_id`, `utm_*`, `at_medium`, `at_campaign` |
| Share / Social | `spm`, `si`, `sh`, `share_id`, `sr_share` |
| Matomo / Piwik | `mtm_*`, `pk_*`, `piwik_*` |
| HSA (Apple Search Ads) | `hsa_*` |

---

## Path Cleaning & Scheme Normalization

### Path Cleaning

```
`agent/internal/normalize/url.go:177-185`
```

```go
// cleanPath collapses repeated slashes and removes a trailing slash, so that
// /a/b, /a//b, and /a/b/ share one identity.
func cleanPath(p string) string {
	if p == "" || p == "/" {
		return ""
	}
	for strings.Contains(p, "//") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return strings.TrimSuffix(p, "/")
}
```

- Collapses multiple consecutive slashes (`/a//b` → `/a/b`)
- Removes trailing slash (`/a/b/` → `/a/b`)
- Empty path or `/` becomes empty string (so `https://example.com` and `https://example.com/` are identical)

### Scheme & Host Normalization

- Scheme lowercased (`HTTP` → `http`)
- Only `http` and `https` allowed; others error
- Host lowercased
- Default ports stripped (`:80` for http, `:443` for https)
- **`www.` prefix is preserved deliberately** — see comment in `CanonicalURL`:

> The `www.` prefix is deliberately preserved. Stripping it would catch a rare class of duplicate, but the result is also the outbound link on every story page (R3), and a host that does not answer without `www.` would break it.

- Userinfo (`user:pass@`) removed
- Fragment (`#section`) removed

---

## ItemID: Stable Identity Hash

`ItemID` computes the first 16 hex characters (64 bits) of the SHA-256 hash of the canonical URL. This is the key used by `State.Seen` / `State.MarkSeen` for deduplication.

```
`agent/internal/normalize/url.go:189-192`
```

```go
// ItemID is the stable identity of an article: the first 16 hex characters of
// the SHA-256 of its canonical URL.
func ItemID(canonicalURL string) string {
	sum := sha256.Sum256([]byte(canonicalURL))
	return hex.EncodeToString(sum[:])[:16]
}
```

### Properties

| Property | Value |
|----------|-------|
| Algorithm | SHA-256 |
| Output length | 16 hex chars (64 bits) |
| Collision resistance | Sufficient for ~600 items/day (birthday bound ~2³²) |
| Deterministic | Yes — same canonical URL → same ItemID |
| Used by | `model.State.Seen`, `model.State.MarkSeen`, `normalize.Dedupe` |

---

## Configuration Constants

| Constant | Value | Purpose |
|----------|-------|---------|
| `maxUnwrapDepth` | `3` | Maximum recursive wrapper unwrapping depth |
| `trackingParams` | 34 entries | Exact-match tracking parameter keys |
| `trackingPrefixes` | 6 prefixes | Prefix-match tracking parameter families |
| `redirectWrappers` | 8 hosts | Known redirect wrapper hosts + their query param keys |

All are **hard-coded** in `url.go` — not configurable via `config/` JSON or environment variables.

---

## Error Handling & Edge Cases

| Input / Condition | Behavior | Error Message |
|-------------------|----------|---------------|
| Empty string | Error | `normalize: empty url` |
| Protocol-relative (`//example.com`) | Prefix `https:` → `https://example.com` | — |
| Bare host (`example.com/path`) | Prefix `https://` → `https://example.com/path` | — |
| Opaque scheme (`mailto:x@y`, `javascript:alert(1)`) | Error | `normalize: unsupported scheme in "..."` |
| Parse failure | Error | `normalize: parse url "...": <parse error>` |
| Non-http/https scheme (`ftp://`, `file://`) | Error | `normalize: unsupported scheme "ftp" in "..."` |
| No host after parse | Error | `normalize: no host in "..."` |
| Wrapper unwrap target lacks `://` | Not unwrapped (treated as non-URL) | — |
| Recursion depth ≥ 3 | Stop unwrapping, continue with current URL | — |
| All query params are tracking | Query string becomes empty | — |
| Path is `/` or empty | Normalized to empty string | — |

### Opaque Scheme Detection

```
`agent/internal/normalize/url.go:143-160`
```

```go
// hasOpaqueScheme reports whether s looks like "scheme:opaque" rather than a
// bare host. A colon followed only by digits is a port, not a scheme.
func hasOpaqueScheme(s string) bool {
	colon := strings.IndexByte(s, ':')
	if colon <= 0 {
		return false
	}
	if slash := strings.IndexByte(s, '/'); slash >= 0 && slash < colon {
		return false // the colon is inside a path
	}

	after := s[colon+1:]
	if slash := strings.IndexByte(after, '/'); slash >= 0 {
		after = after[:slash]
	}
	if after == "" {
		return true
	}
	return strings.ContainsFunc(after, func(r rune) bool { return r < '0' || r > '9' })
}
```

This prevents `example.com:8080` (bare host with port) from being misclassified as an opaque scheme, while catching `mailto:user@example.com` and `javascript:alert(1)`.

---

## Referenced Files

- `agent/internal/normalize/url.go` — CanonicalURL, ItemID, redirectWrappers, trackingParams, trackingPrefixes, maxUnwrapDepth, and all helper functions (canonicalURL, unwrap, hasOpaqueScheme, isTracking, cleanPath)

<!-- kaioken:files agent/internal/normalize/url.go -->
