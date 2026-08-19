# Repository & Paper URL Extraction

This chapter details the two URL extraction functions in the normalization pipeline: `ExtractRepoURL` for GitHub repository detection and `ExtractPaperURL` for arXiv paper identification. Both operate on raw ingest text (title, excerpt, source URL) and return canonicalized URLs used for clustering and deduplication.

## Table of Contents

- [Architecture & Role in Pipeline](#architecture--role-in-pipeline)
- [ExtractRepoURL — GitHub Repository Detection](#extractrepourl---github-repository-detection)
  - [Regex Pattern](#regex-pattern)
  - [Reserved Path Filtering](#reserved-path-filtering)
  - [Reserved Repository Filtering](#reserved-repository-filtering)
  - [Canonicalization Rules](#canonicalization-rules)
  - [Algorithm Walkthrough](#algorithm-walkthrough)
- [ExtractPaperURL — arXiv Paper Detection](#extractpaperurl---arxiv-paper-detection)
  - [Modern arXiv ID Pattern](#modern-arxiv-id-pattern)
  - [Legacy arXiv ID Pattern](#legacy-arxiv-id-pattern)
  - [Version Suffix Handling](#version-suffix-handling)
  - [Canonicalization Rules](#canonicalization-rules-1)
  - [Algorithm Walkthrough](#algorithm-walkthrough-1)
- [Integration with Normalization](#integration-with-normalization)
- [Edge Cases & Error Handling](#edge-cases--error-handling)
- [Referenced Files](#referenced-files)

---

## Architecture & Role in Pipeline

```
┌─────────────────────────────────────────────────────────────────┐
│                        NORMALIZATION STAGE                       │
├─────────────────────────────────────────────────────────────────┤
│  Raw Item (title, link, content, published, source)             │
│                           │                                      │
│                           ▼                                      │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ normalize.Build(Raw, now, maxExcerpt) → model.Item       │    │
│  │                                                          │    │
│  │  • StripHTML(content) → plain text                       │    │
│  │  • Excerpt(text, 300) → capped excerpt                   │    │
│  │  • CanonicalURL(link) → canonical URL                    │    │
│  │  • ItemID(canonicalURL) → stable hash ID                 │    │
│  │  • ExtractRepoURL(title, excerpt, link) → repo URL       │    │
│  │  • ExtractPaperURL(title, excerpt, link) → paper URL     │    │
│  └─────────────────────────────────────────────────────────┘    │
│                           │                                      │
│                           ▼                                      │
│  model.Item { Title, URL, Excerpt, RepoURL, PaperURL, ... }    │
└─────────────────────────────────────────────────────────────────┘
```

Both extraction functions are **pure, deterministic, and side-effect-free** — they take variadic string inputs and return a single canonical URL or empty string. They are called during `normalize.Build` after HTML stripping and excerpt generation, ensuring they operate on clean text.

---

## ExtractRepoURL — GitHub Repository Detection

### Purpose

Identifies the first GitHub repository reference in the provided texts and returns a canonical `https://github.com/owner/repo` URL. This URL serves as a **strong clustering signal**: two items pointing at the same repository are forced into the same cluster regardless of embedding similarity.

### Signature

```go
func ExtractRepoURL(texts ...string) string
```

`agent/internal/normalize/extract.go:49-69`

### Regex Pattern

```go
githubRepoRE = regexp.MustCompile(`(?i)github\.com/([A-Za-z0-9][A-Za-z0-9_.-]*)/([A-Za-z0-9][A-Za-z0-9_.-]*)`)
```

`agent/internal/normalize/extract.go:10-12`

| Component | Pattern | Description |
|-----------|---------|-------------|
| `(?i)` | Case-insensitive flag | Matches `github.com`, `GitHub.com`, etc. |
| `github\.com/` | Literal domain | Anchors to GitHub domain |
| `([A-Za-z0-9][A-Za-z0-9_.-]*)` | Capture group 1 (owner) | Starts with alphanumeric, then alphanumeric/underscore/dot/hyphen |
| `/` | Literal slash | Path separator |
| `([A-Za-z0-9][A-Za-z0-9_.-]*)` | Capture group 2 (repo) | Same character class as owner |

**Matches:**
- `github.com/owner/repo`
- `https://github.com/owner/repo`
- `github.com/owner/repo.git`
- `See github.com/owner/repo for details`

**Does not match:**
- `github.com/owner` (no repo segment)
- `github.com/owner/repo/` (trailing slash not in pattern, but captured repo would include it — handled by trimming)

### Reserved Path Filtering

First-path-segment filtering prevents GitHub site features from being misidentified as repositories.

`agent/internal/normalize/extract.go:22-35`

```go
var githubReservedPaths = map[string]bool{
	"about": true, "account": true, "actions": true, "apps": true, "blog": true,
	"careers": true, "changelog": true, "codespaces": true, "collections": true,
	"contact": true, "copilot": true, "customer-stories": true, "dashboard": true,
	"discussions": true, "education": true, "enterprise": true, "events": true,
	"explore": true, "features": true, "git-guides": true, "github": true,
	"issues": true, "join": true, "login": true, "logout": true, "marketplace": true,
	"mobile": true, "new": true, "nonprofit": true, "notifications": true,
	"organizations": true, "orgs": true, "packages": true, "premium-support": true,
	"pricing": true, "pulls": true, "readme": true, "releases": true, "search": true,
	"security": true, "settings": true, "signup": true, "site": true, "sponsors": true,
	"stars": true, "team": true, "topics": true, "trending": true, "users": true,
	"watching": true,
}
```

| Category | Reserved Paths |
|----------|----------------|
| Auth/Account | `account`, `login`, `logout`, `signup`, `join`, `settings` |
| Product Features | `actions`, `codespaces`, `copilot`, `packages`, `projects`, `pulls`, `issues`, `discussions`, `releases`, `security` |
| Discovery | `explore`, `trending`, `topics`, `collections`, `marketplace`, `search` |
| Organization | `organizations`, `orgs`, `teams`, `enterprise`, `education`, `nonprofit` |
| User Profile | `users`, `stars`, `watching`, `followers`, `following`, `sponsors` |
| Static Content | `about`, `blog`, `changelog`, `readme`, `git-guides`, `features`, `pricing`, `careers`, `contact`, `site` |
| Dashboard/Notifications | `dashboard`, `notifications`, `new`, `mobile` |

**Example filtered:**
- `github.com/features/copilot` → `owner="features"` (reserved) → **skipped**
- `github.com/blog/announcement` → `owner="blog"` (reserved) → **skipped**
- `github.com/orgs/myorg` → `owner="orgs"` (reserved) → **skipped**

### Reserved Repository Filtering

Second-path-segment filtering prevents account-level pages from being misidentified as repositories.

`agent/internal/normalize/extract.go:39-42`

```go
var githubReservedRepos = map[string]bool{
	"followers": true, "following": true, "repositories": true, "projects": true,
	"packages": true, "stars": true, "sponsors": true, "settings": true,
}
```

| Reserved Repo | Page Type |
|---------------|-----------|
| `followers` | User's followers list |
| `following` | User's following list |
| `repositories` | User's repository listing |
| `projects` | User's projects page |
| `packages` | User's packages page |
| `stars` | User's starred repositories |
| `sponsors` | User's sponsorship page |
| `settings` | User/account settings |

**Example filtered:**
- `github.com/torvalds/followers` → `repo="followers"` (reserved) → **skipped**
- `github.com/microsoft/repositories` → `repo="repositories"` (reserved) → **skipped**

### Canonicalization Rules

1. **Scheme normalization** → Always `https://github.com/`
2. **Owner case preservation** → Original casing kept (GitHub is case-insensitive but displays original)
3. **Repo `.git` suffix removal** → `strings.TrimSuffix(m[2], ".git")`
4. **Trailing punctuation stripping** → `strings.TrimRight(repo, ".-_")` handles prose like `"see github.com/a/b."`
5. **Empty repo guard** → If trimming yields empty string, continue to next match

### Algorithm Walkthrough

```mermaid
flowchart TD
    A[Start: ExtractRepoURL(texts...)] --> B{For each text}
    B --> C[FindAllStringSubmatch githubRepoRE]
    C --> D{For each match}
    D --> E[owner = m[1], repo = m[2] with .git stripped]
    E --> F{owner in githubReservedPaths?}
    F -- Yes --> D
    F -- No --> G{repo in githubReservedRepos?}
    G -- Yes --> D
    G -- No --> H[repo = TrimRight(repo, .-_)]
    H --> I{repo == ""?}
    I -- Yes --> D
    I -- No --> J[Return https://github.com/owner/repo]
    J --> K[End]
    D -->|No more matches| B
    B -->|No more texts| L[Return ""]
    L --> K
```

`agent/internal/normalize/extract.go:49-69`

```go
func ExtractRepoURL(texts ...string) string {
	for _, text := range texts {
		for _, m := range githubRepoRE.FindAllStringSubmatch(text, -1) {
			owner, repo := m[1], strings.TrimSuffix(m[2], ".git")

			if githubReservedPaths[strings.ToLower(owner)] {
				continue
			}
			if githubReservedRepos[strings.ToLower(repo)] {
				continue
			}
			// Trailing punctuation from prose: "see github.com/a/b."
			repo = strings.TrimRight(repo, ".-_")
			if repo == "" {
				continue
			}
			return "https://github.com/" + owner + "/" + repo
		}
	}
	return ""
}
```

**Key behaviors:**
- **First-match-wins**: Returns immediately on first valid repository
- **Case-insensitive reserved checks**: `strings.ToLower` before map lookup
- **Variadic input**: Checks title, then excerpt, then source URL in order passed
- **No match → empty string**: Caller treats empty as "no repository found"

---

## ExtractPaperURL — arXiv Paper Detection

### Purpose

Identifies the first arXiv paper reference in the provided texts and returns a canonical `https://arxiv.org/abs/ID` URL. The version suffix (`v1`, `v2`, etc.) is **deliberately dropped** so that different versions of the same paper collapse into one cluster.

### Signature

```go
func ExtractPaperURL(texts ...string) string
```

`agent/internal/normalize/extract.go:76-86`

### Modern arXiv ID Pattern

```go
arxivModernRE = regexp.MustCompile(`(?i)(?:arxiv\.org/(?:abs|pdf|html)/|arxiv[:\s]+|huggingface\.co/papers/)(\d{4}\.\d{4,5})(v\d+)?`)
```

`agent/internal/normalize/extract.go:13-15`

| Component | Pattern | Description |
|-----------|---------|-------------|
| `(?i)` | Case-insensitive | Matches `arXiv`, `ARXIV`, `Arxiv`, etc. |
| `(?:arxiv\.org/(?:abs|pdf|html)/` | Non-capture group | Direct arXiv.org URLs: `/abs/`, `/pdf/`, `/html/` |
| `\|arxiv[:\s]+` | OR | `arxiv:` or `arxiv ` prefix in prose |
| `\|huggingface\.co/papers/` | OR | Hugging Face Papers integration URLs |
| `)` | End non-capture | |
| `(\d{4}\.\d{4,5})` | Capture group 1 (ID) | Year (4 digits) + dot + 4-5 digits (e.g., `2401.12345`) |
| `(v\d+)?` | Capture group 2 (version) | Optional `v` + digits (e.g., `v1`, `v2`) |

**Matches:**
- `arxiv.org/abs/2401.12345` → ID=`2401.12345`
- `arxiv.org/pdf/2401.12345v2` → ID=`2401.12345`, version=`v2` (dropped)
- `arxiv:2401.12345` → ID=`2401.12345`
- `arxiv 2401.12345v3` → ID=`2401.12345`
- `huggingface.co/papers/2401.12345` → ID=`2401.12345`

### Legacy arXiv ID Pattern

```go
arxivLegacyRE = regexp.MustCompile(`(?i)arxiv\.org/(?:abs|pdf)/([a-z-]+(?:\.[a-z]{2})?/\d{7})`)
```

`agent/internal/normalize/extract.go:16-18`

| Component | Pattern | Description |
|-----------|---------|-------------|
| `(?i)` | Case-insensitive | |
| `arxiv\.org/(?:abs|pdf)/` | Literal prefix | Legacy URLs only on `/abs/` or `/pdf/` |
| `([a-z-]+(?:\.[a-z]{2})?/\d{7})` | Capture group 1 (ID) | Category + optional subcategory + 7-digit ID |

**Legacy ID format:** `category/subcategory/YYYYMM` or `category/YYYYMM` (7 digits total after slash)

**Examples:**
- `cs.AI/0301001` → category `cs.AI`, ID `0301001` (Jan 2003)
- `math/0211159` → category `math`, ID `0211159` (Nov 2002)
- `physics.hep-th/0301001` → category `physics.hep-th`, ID `0301001`

**Note:** Legacy pattern only matches `arxiv.org/abs/` or `arxiv.org/pdf/` URLs — not prose references.

### Version Suffix Handling

```go
// The version suffix is dropped deliberately: v1 and v2 of a paper are the same
// event, so they must collapse into one cluster.
```

`agent/internal/normalize/extract.go:76-78`

- Modern regex captures version in group 2 (`(v\d+)?`) but **only group 1 (ID) is used**
- Legacy IDs have no version concept (pre-2007)
- This ensures `2401.12345v1` and `2401.12345v2` → same canonical URL → same cluster

### Canonicalization Rules

1. **Scheme + host** → Always `https://arxiv.org/abs/`
2. **ID extraction** → Only the numeric ID (modern) or category/ID (legacy)
3. **Case normalization (legacy)** → `strings.ToLower(m[1])` for legacy IDs
4. **Version dropped** → Modern regex captures but discards version

### Algorithm Walkthrough

```mermaid
flowchart TD
    A[Start: ExtractPaperURL(texts...)] --> B{For each text}
    B --> C[Try arxivModernRE.FindStringSubmatch]
    C --> D{Match found?}
    D -- Yes --> E[Return https://arxiv.org/abs/ + m[1]]
    D -- No --> F[Try arxivLegacyRE.FindStringSubmatch]
    F --> G{Match found?}
    G -- Yes --> H[Return https://arxiv.org/abs/ + Lower(m[1])]
    G -- No --> I[Continue to next text]
    I --> B
    B -->|No more texts| J[Return ""]
    E --> K[End]
    H --> K
    J --> K
```

`agent/internal/normalize/extract.go:76-86`

```go
func ExtractPaperURL(texts ...string) string {
	for _, text := range texts {
		if m := arxivModernRE.FindStringSubmatch(text); m != nil {
			return "https://arxiv.org/abs/" + m[1]
		}
		if m := arxivLegacyRE.FindStringSubmatch(text); m != nil {
			return "https://arxiv.org/abs/" + strings.ToLower(m[1])
		}
	}
	return ""
```

**Key behaviors:**
- **Modern first, then legacy**: Modern pattern checked before legacy
- **First-match-wins**: Returns immediately on first valid paper
- **Case normalization for legacy**: Legacy categories lowercased (arXiv canonical form)
- **No match → empty string**: Caller treats empty as "no paper found"

---

## Integration with Normalization

Both functions are invoked from `normalize.Build`:

`agent/internal/normalize/build.go` (not in scope but called from here)

```go
func Build(raw Raw, now time.Time, maxExcerpt int) Item {
	text := StripHTML(raw.Content)
	excerpt := Excerpt(text, maxExcerpt)
	canonical := CanonicalURL(raw.Link)
	id := ItemID(canonical)

	return Item{
		ID:          id,
		Title:       CleanTitle(raw.Title),
		URL:         canonical,
		Excerpt:     excerpt,
		Source:      raw.Source,
		SourceType:  raw.SourceType,
		Published:   raw.Published,
		RepoURL:     ExtractRepoURL(raw.Title, excerpt, raw.Link),
		PaperURL:    ExtractPaperURL(raw.Title, excerpt, raw.Link),
	}
}
```

**Input order matters**: Title → Excerpt → Source Link. Title is checked first (highest signal), then excerpt, then the original link.

**Item fields populated:**
| Field | Type | Source |
|-------|------|--------|
| `RepoURL` | `string` | `ExtractRepoURL(title, excerpt, link)` |
| `PaperURL` | `string` | `ExtractPaperURL(title, excerpt, link)` |

These URLs flow into:
- **Clustering**: `RepoURL` forces same-repo items into one cluster
- **Deduplication**: `PaperURL` used as secondary dedup key (via `ItemID` which hashes canonical URL, but paper URL provides semantic equivalence)
- **Story generation**: LLM receives both URLs for citation

---

## Edge Cases & Error Handling

| Scenario | Behavior |
|----------|----------|
| Multiple GitHub repos in text | First valid (non-reserved) repo returned |
| Multiple arXiv papers in text | First valid paper returned |
| `github.com/owner/repo.git` | `.git` stripped → `https://github.com/owner/repo` |
| `github.com/owner/repo.` (trailing period) | Period stripped via `TrimRight(repo, ".-_")` |
| `github.com/owner/repo-` (trailing hyphen) | Hyphen stripped |
| `github.com/owner/repo_` (trailing underscore) | Underscore stripped |
| `github.com/owner/` (no repo) | Regex requires two segments → no match |
| `github.com/Features/Copilot` | `Features` in reserved paths → skipped |
| `github.com/torvalds/followers` | `followers` in reserved repos → skipped |
| `arxiv:2401.12345v2` | Version `v2` captured but dropped → `https://arxiv.org/abs/2401.12345` |
| `ARXIV.ORG/ABS/2401.12345` | Case-insensitive → matched |
| `huggingface.co/papers/2401.12345` | Matched via modern regex alternative |
| `cs.AI/0301001` in prose (no URL) | **Not matched** — legacy requires `arxiv.org/` prefix |
| Empty/nil input texts | Loop executes zero times → returns `""` |
| Very long text | `FindAllStringSubmatch` / `FindStringSubmatch` scan entire string — no length limit |

**No errors returned** — both functions return `string` (empty on no match). This is intentional: absence of a repo/paper URL is not an error condition.

---

## Referenced Files

| File | Description |
|------|-------------|
| `agent/internal/normalize/extract.go` | Contains `ExtractRepoURL`, `ExtractPaperURL`, regex definitions, and reserved path/repo maps |
| `agent/internal/normalize/build.go` | Calls both extraction functions during `Item` construction (referenced, not in scope) |
| `agent/internal/model/item.go` | Defines `Item` struct with `RepoURL` and `PaperURL` fields (referenced, not in scope) |

<!-- kaioken:files agent/internal/normalize/extract.go -->
