| Principle | Description |
| :--- | :--- |
| Grid Density | Google Images style. High information density, strict grid layout. |
| Visual Snippets | Each story is a "card" focusing on title, source, and score. Minimal text. |
| Fast Scanning | Left rail filters, top search, immediate visual feed of news items. |
| Kaioken Vibe | Dark mode, sharp edges, red accent for high scores. |

| Element | Hex | Usage |
| :--- | :--- | :--- |
| Background Deep | `#0a0a0a` | Main site background |
| Surface | `#121212` | Grid card background |
| Surface Hover | `#1a1a1a` | Card hover state |
| Text Primary | `#f5f5f5` | Card titles, headers |
| Text Secondary | `#a0a0a0` | Domains, dates, tags |
| Accent Kaioken | `#ff3b3b` | High scores, active filters |
| Border | `#262626` | Card outlines, dividers |

| Type | Font | Size / Weight |
| :--- | :--- | :--- |
| Header/Logo | Inter | 1.5rem / 700 |
| Card Title | Inter | 1rem / 600 (2 line clamp) |
| Card Meta | JetBrains Mono | 0.8rem / 500 |
| Filter Text | Inter | 0.9rem / 400 |

| Zone | Width | Content |
| :--- | :--- | :--- |
| Header | 100% | Logo, Search Bar, Dark mode toggle |
| Left Rail | 200px Fixed | Categories (Tags), Date Filters, Source Tiers |
| Grid Area | 1fr (Fills rest) | CSS Grid of `NewsCard` components (auto-fill, minmax 250px) |

| Component | Props | Visuals |
| :--- | :--- | :--- |
| NewsCard | title, domain, score, date | Square/4:3 ratio container. Title at top. Domain + Date at bottom. Score badge top-right. |
| ScoreBadge | score | `#ff3b3b` if >80, `#333` otherwise. Monospace font. |
| SearchBar | query | Minimal input, border-bottom only, expands on focus. |
| FilterChip | tag, count | Clickable text in left rail. Red left-border if active. |

| Breakpoint | Grid Columns | Left Rail |
| :--- | :--- | :--- |
| > 1024px | 3-4 (auto-fill minmax 250px) | Visible (200px) |
| 768px - 1024px | 2-3 (auto-fill minmax 200px) | Hidden (Toggle via menu) |
| < 768px | 1-2 (auto-fill minmax 150px) | Hidden (Toggle via menu) |