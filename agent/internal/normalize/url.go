package normalize

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

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

// trackingPrefixes are query parameter prefixes that are always analytics.
var trackingPrefixes = []string{"utm_", "ga_", "hsa_", "mtm_", "pk_", "piwik_"}

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

// maxUnwrapDepth bounds wrapper resolution so a self-referencing redirect
// cannot loop forever.
const maxUnwrapDepth = 3

// CanonicalURL reduces a URL to a stable identity: no tracking parameters, no
// fragment, no default port, lowercase scheme and host, sorted query, and no
// trailing slash.
//
// The `www.` prefix is deliberately preserved. Stripping it would catch a rare
// class of duplicate, but the result is also the outbound link on every story
// page (R3), and a host that does not answer without `www.` would break it.
func CanonicalURL(raw string) (string, error) {
	return canonicalURL(raw, 0)
}

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

// ItemID is the stable identity of an article: the first 16 hex characters of
// the SHA-256 of its canonical URL.
func ItemID(canonicalURL string) string {
	sum := sha256.Sum256([]byte(canonicalURL))
	return hex.EncodeToString(sum[:])[:16]
}
