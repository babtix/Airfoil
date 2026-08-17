# Dark:

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

# Claire


|Principle|Description|
|---|---|
|Clean & Bright|White background, soft grays for borders and secondary text. High readability.|
|Serif Headlines|Uses Georgia/Times New Roman for story titles to give a journalistic feel.|
|Soft Elevation|Subtle shadows (`0 1px 3px rgba(0,0,0,0.06)`) that deepen on hover.|
|Signal Ranking|Color-coded score badges (Green/Orange/Red) to indicate news importance.|

|Element|Hex / Value|Usage|
|---|---|---|
|Background|`#f8f9fb`|Main site background|
|Surface|`#ffffff`|Cards, header, modal background|
|Border|`#e8eaed`|Dividers, card outlines, input borders|
|Text Primary|`#1a1a1a`|Headlines, primary text|
|Text Secondary|`#5f6368`|Summaries, nav items|
|Text Muted|`#9aa0a6`|Dates, meta info, icons|
|Primary (Indigo)|`#4f46e5`|Active states, tags, links, focus rings|
|Primary Light|`#eef2ff`|Active nav background, tag backgrounds|
|Success (High)|`#059669` / `#ecfdf5`|Score >= 80 (Text / Background)|
|Warning (Mid)|`#d97706` / `#fffbeb`|Score >= 60 (Text / Background)|
|Danger (Low)|`#dc2626` / `#fef2f2`|Score < 60 (Text / Background)|

|Type|Font|Size / Weight|
|---|---|---|
|Logo Text|Inter|20px / 700|
|Nav & Tags|Inter|13px - 14px / 500 - 600|
|Card Title|Georgia (Serif)|18px / 700|
|Card Summary|Inter|14px / 400|
|Modal Title|Georgia (Serif)|26px / 700|
|Meta / Dates|Inter|12px / 400|

|Zone|Layout / Size|Content|
|---|---|---|
|Header|Sticky, Blur(12px), 64px height|Logo, Nav (Feed, Top 24h, Top 7d, Digest), Search Bar|
|Controls|Flex wrap, 24px top margin|Tag filter pills, Sort dropdown|
|Feed Grid|`auto-fill, minmax(340px, 1fr)`|`StoryCard` components|
|Modal|Fixed overlay, Blur(4px), 720px max|Full summary, source list, tags|

|Component|Props|Visuals|
|---|---|---|
|StoryCard|title, summary, score, date, tags, clusterSize|White bg, 1px border, 12px radius. Hover: lifts up (-2px), shadow deepens.|
|ScoreBadge|score|Color-coded pill. Displays number + "signal" label.|
|TagPill|tag|Indigo text, light indigo bg, 10px radius.|
|TagFilter|tag, active|Outline pill. Active state: solid indigo bg, white text.|
|Modal|story_data|White card, 16px radius. Close button top-right. Serif title, source list with dotted links.|