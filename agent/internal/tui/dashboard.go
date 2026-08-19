package tui

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/score"
	"github.com/papitsho/airfoil/internal/store"
)

// dashboard is the at-a-glance view: what is configured, what is on disk, and
// whether the agent can actually run right now.
type dashboard struct {
	stats  dataStats
	loaded bool
}

// counted is one name and how often it occurred, for the ranked breakdowns.
type counted struct {
	name string
	n    int
}

type dataStats struct {
	// Pipeline volumes.
	Items       int
	ItemDays    int
	ItemsToday  int
	Items24h    int
	WithSignal  int
	Clusters    int
	MultiSource int
	Ranked      int
	Major       int
	Notable     int
	Stories     int
	Markdown    int
	DigestFiles int

	// Where the items came from, and which enabled sources sent nothing.
	BySource []counted
	Silent   []string

	// What was published.
	StoryMajor   int
	StoryNotable int
	StoryMinor   int
	BuilderRel   int
	StoriesToday int
	AvgCluster   float64
	TopStory     string
	TopScore     int
	NewestStory  *time.Time
	TopTags      []counted

	// State and footprint.
	SeenURLs   int
	Cursors    int
	CacheBytes int64
	ItemsBytes int64
	LastRun    *time.Time

	Err error
}

type statsMsg dataStats

func newDashboard() *dashboard { return &dashboard{} }

func (d *dashboard) Init() tea.Cmd { return nil }

func (d *dashboard) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case statsMsg:
		d.stats, d.loaded = dataStats(msg), true
		return nil, false

	case tea.KeyMsg:
		if msg.String() == "R" {
			d.loaded = false
			return loadStats(m.cfg), true
		}
	}

	// Load lazily on the first render rather than blocking startup.
	if !d.loaded {
		d.loaded = true
		return loadStats(m.cfg), false
	}
	return nil, false
}

// loadStats reads what is on disk. It runs as a command so a large items file
// never stalls the interface, and it walks each file once: everything the
// panels show is derived here, not at render time.
func loadStats(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		var out dataStats
		now := time.Now().UTC()
		today := now.Format(time.DateOnly)

		state, err := store.LoadState(cfg.StatePath())
		if err != nil {
			out.Err = err
			return statsMsg(out)
		}
		out.SeenURLs = len(state.SeenURLs)
		out.Cursors = len(state.Cursors)
		out.LastRun = state.LastRun

		items, err := store.LoadItemsSince(cfg.ItemsDir(), now, 30)
		if err == nil {
			out.Items = len(items)

			days := map[string]bool{}
			bySource := map[string]int{}
			for _, it := range items {
				days[it.PublishedAt.Format(time.DateOnly)] = true
				bySource[it.SourceID]++

				if it.FetchedAt.UTC().Format(time.DateOnly) == today {
					out.ItemsToday++
				}
				if now.Sub(it.PublishedAt) < 24*time.Hour {
					out.Items24h++
				}
				if hasSignal(it.Metrics) {
					out.WithSignal++
				}
			}
			out.ItemDays = len(days)
			out.BySource = topCounts(bySource, 6)

			// An enabled source that returned nothing all window is the
			// failure this dashboard is most likely to catch first.
			for _, src := range cfg.EnabledSources() {
				if bySource[src.ID] == 0 {
					out.Silent = append(out.Silent, src.ID)
				}
			}
		}

		clusters, err := store.ReadJSONOr(cfg.DataDir+"/clusters.json", []model.Cluster(nil))
		if err == nil {
			out.Clusters = len(clusters)
			for _, c := range clusters {
				if len(c.Items) > 1 {
					out.MultiSource++
				}
			}
		}

		ranked, err := store.ReadJSONOr(cfg.DataDir+"/ranked.json", []score.Result(nil))
		if err == nil {
			out.Ranked = len(ranked)
			for _, r := range ranked {
				switch r.Tier {
				case model.TierMajor:
					out.Major++
				case model.TierNotable:
					out.Notable++
				}
			}
		}

		stories, err := store.ReadJSONOr(cfg.StoriesPath(), []model.Story(nil))
		if err == nil {
			out.Stories = len(stories)

			tags := map[string]int{}
			clusterSum := 0
			for _, st := range stories {
				switch st.Tier {
				case model.TierMajor:
					out.StoryMajor++
				case model.TierNotable:
					out.StoryNotable++
				default:
					out.StoryMinor++
				}
				if st.BuilderRelevant {
					out.BuilderRel++
				}
				if st.Date.UTC().Format(time.DateOnly) == today {
					out.StoriesToday++
				}
				for _, tag := range st.Tags {
					tags[tag]++
				}
				clusterSum += st.ClusterSize

				if st.Score > out.TopScore {
					out.TopScore, out.TopStory = st.Score, st.Title
				}
				if out.NewestStory == nil || st.Date.After(*out.NewestStory) {
					newest := st.Date
					out.NewestStory = &newest
				}
			}
			out.TopTags = topCounts(tags, 6)
			if len(stories) > 0 {
				out.AvgCluster = float64(clusterSum) / float64(len(stories))
			}
		}

		// The markdown collection is the product; count it directly rather
		// than trusting stories.json to match.
		if files, err := filepath.Glob(filepath.Join(cfg.StoriesDir, "*.md")); err == nil {
			out.Markdown = len(files)
		}
		if files, err := filepath.Glob(filepath.Join(cfg.DigestDir(), "*")); err == nil {
			out.DigestFiles = len(files)
		}

		out.CacheBytes = dirSize(cfg.CacheDir())
		out.ItemsBytes = dirSize(cfg.ItemsDir())

		return statsMsg(out)
	}
}

func hasSignal(m model.Metrics) bool {
	return m.HNPoints > 0 || m.RedditScore > 0 || m.HFUpvotes > 0 || m.GitHubStars > 0
}

// topCounts ranks a tally, largest first and ties broken by name so the panel
// does not reshuffle between refreshes.
func topCounts(tally map[string]int, n int) []counted {
	out := make([]counted, 0, len(tally))
	for name, count := range tally {
		out = append(out, counted{name, count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].n != out[j].n {
			return out[i].n > out[j].n
		}
		return out[i].name < out[j].name
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

// dirSize totals a tree, reporting 0 for a tree that is not there yet. The
// embedding cache is the one directory that grows without anyone watching.
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return nil
		}
		if info, err := e.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// humanBytes renders a size the way a person would say it.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 3 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

func (d *dashboard) Footer() string {
	return styleKey.Render("R") + styleFooter.Render(" refresh")
}

// statsMinWidth is the narrowest a readable stats column gets. Below the rail
// plus that, the rail has nothing to sit beside and everything stacks.
const statsMinWidth = 34

// The dashboard is a hero rail and a stats column: the mark, the verdict, and
// the funnel on the left — everything else on the right. The rail is exactly
// as wide as the wordmark, which is what makes the mark sit flush rather than
// float in a column sized for text.
//
// The panels read in the order the pipeline works — what came in, what runs
// it, what came out, and the rules that shaped it — and the columns are split
// to end at roughly the same row rather than filled one at a time.
func (d *dashboard) View(m *Model, width, height int) string {
	cfg, s := m.cfg, d.stats

	rail := logoWidth() + 4
	if width < rail+statsMinWidth {
		return d.stackedView(m, width, height)
	}

	statsWidth := width - rail - 1
	twoColumns := statsWidth >= 2*statsMinWidth+1
	if twoColumns {
		statsWidth = (statsWidth - 1) / 2
	}

	// The rail carries the storage panel under the hero, so the left column
	// is balanced and fills the terminal height.
	hero := box(d.heroBody(cfg, s, rail-4, height), rail)
	railColumn := hero
	if under, _ := fitColumn(d.railPanels(rail), height-lipgloss.Height(hero)); under != "" {
		railColumn = lipgloss.JoinVertical(lipgloss.Left, hero, under)
	}

	columns := []string{railColumn}
	if twoColumns {
		first := d.dataPanels(cfg, statsWidth)
		second := d.systemPanels(cfg, statsWidth)
		a, _ := fitColumn(first, height)
		b, _ := fitColumn(second, height)
		columns = append(columns, a, b)
	} else {
		all := d.statsPanels(cfg, statsWidth)
		only, _ := fitColumn(all, height)
		columns = append(columns, only)
	}

	return joinColumns(columns) + errorLine(s, width)
}

// dataPanels are the ingestion and story output cards for the center column.
func (d *dashboard) dataPanels(cfg *config.Config, width int) []string {
	inner := max(width-4, 12)
	s := d.stats

	metaIngest := ""
	if s.WithSignal > 0 {
		metaIngest = fmt.Sprintf("%d w/ signal", s.WithSignal)
	} else if len(s.BySource) > 0 {
		metaIngest = plural(len(s.BySource), "source")
	}

	metaTier := fmt.Sprintf("%d enabled", len(cfg.EnabledSources()))

	metaPublished := ""
	if s.Stories > 0 {
		metaPublished = plural(s.Stories, "story")
	}

	return []string{
		card("ingest by source", metaIngest, ingestBody(s, inner), width),
		card("sources by tier", metaTier, tierBody(cfg, inner), width),
		card("published", metaPublished, outputBody(s, inner), width),
	}
}

// systemPanels are the intelligence, engine, and scoring cards for the right column.
func (d *dashboard) systemPanels(cfg *config.Config, width int) []string {
	inner := max(width-4, 12)
	s := d.stats

	metaTopics := ""
	if len(s.TopTags) > 0 {
		metaTopics = fmt.Sprintf("%d top", len(s.TopTags))
	}

	metaEngine := fmt.Sprintf("%d ready", readyProviders(cfg))

	return []string{
		card("topics", metaTopics, topicsBody(s, inner), width),
		card("engine & providers", metaEngine, engineBody(cfg, inner), width),
		card("thresholds and limits", "scoring rules", limitsBody(cfg, inner), width),
	}
}

// statsPanels returns all panels beside the rail in pipeline order.
func (d *dashboard) statsPanels(cfg *config.Config, width int) []string {
	return append(d.dataPanels(cfg, width), d.systemPanels(cfg, width)...)
}

// railPanels go under the hero in the rail's own column: what the last run left behind on disk.
func (d *dashboard) railPanels(width int) []string {
	inner := max(width-4, 12)
	s := d.stats
	metaDisk := ""
	if s.ItemsBytes > 0 || s.CacheBytes > 0 {
		metaDisk = humanBytes(s.ItemsBytes + s.CacheBytes)
	}
	return []string{card("on disk", metaDisk, storageBody(s, inner), width)}
}

// joinColumns lays columns out left to right with one blank column between.
func joinColumns(columns []string) string {
	parts := make([]string, 0, len(columns)*2)
	for i, c := range columns {
		if c == "" {
			continue
		}
		if i > 0 {
			parts = append(parts, " ")
		}
		parts = append(parts, c)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// splitColumns divides panels in order between two columns so both end at
// roughly the same row. Filling the first column to the brim instead leaves
// the second one stubby, which reads as a mistake rather than a layout.
func splitColumns(panels []string, height int) (first, second []string) {
	total := 0
	for _, p := range panels {
		total += lipgloss.Height(p)
	}
	target := total / 2

	used := 0
	for i, p := range panels {
		h := lipgloss.Height(p)
		// Stop once this panel would take the column further past the
		// midpoint than stopping short of it would.
		if len(first) > 0 && (used+h > height || used+h-target > target-used) {
			return first, panels[i:]
		}
		first = append(first, p)
		used += h
	}
	return first, nil
}

// fitColumn stacks panels for as long as the rows last and hands back the ones
// it could not place, so a caller with a second column can continue there.
func fitColumn(panels []string, height int) (string, []string) {
	var placed []string
	used := 0

	for i, p := range panels {
		h := lipgloss.Height(p)
		if len(placed) > 0 && used+h > height {
			return lipgloss.JoinVertical(lipgloss.Left, placed...), panels[i:]
		}
		placed = append(placed, p)
		used += h
	}
	if len(placed) == 0 {
		return "", nil
	}
	return lipgloss.JoinVertical(lipgloss.Left, placed...), nil
}

// heroBody fills the left rail. The mark takes the top when the rail has rows
// to spare and shrinks to its one-line form when it does not, so the rail is
// never headless.
func (d *dashboard) heroBody(cfg *config.Config, s dataStats, inner, height int) string {
	var b strings.Builder
	b.WriteString(center(readiness(cfg, s, inner), inner) + "\n\n")
	b.WriteString(styleHeader.Render("pipeline") + "\n")
	b.WriteString(d.funnelTall(inner))
	rest := b.String()

	// The day's numbers are shown when there are spare rows beyond the funnel and storage.
	today := "\n\n" + styleHeader.Render("today") + "\n" +
		stat("fetched today", plural(s.ItemsToday, "item"), inner) +
		stat("published 24h", plural(s.Items24h, "item"), inner) +
		strings.TrimRight(stat("stories today", fmt.Sprint(s.StoriesToday), inner), "\n")
	if height >= lipgloss.Height(rest)+lipgloss.Height(today)+12 {
		rest += today
	}

	head := center(logoWordmark(), inner)
	if banner := logoBanner(inner); banner != "" &&
		height >= lipgloss.Height(rest)+logoRows+4 {
		head = banner
	}
	return head + "\n\n" + rest
}

// stackedView is the narrow fallback: one column, and the banner only when the
// rows are genuinely spare.
func (d *dashboard) stackedView(m *Model, width, height int) string {
	cfg, s := m.cfg, d.stats

	colWidth := max(width, 24)
	inner := max(colWidth-4, 12)

	panels := append(
		[]string{card("pipeline", "", d.funnelWide(inner), colWidth)},
		d.statsPanels(cfg, colWidth)...)
	panels = append(panels, d.railPanels(colWidth)...)

	// The banner competes with the panels for the same rows here, and the
	// panels win: it appears only once every one of them is already placed.
	body, unplaced := fitColumn(panels, height-2)
	var banner string
	if art := logoBanner(width); art != "" && len(unplaced) == 0 &&
		height-lipgloss.Height(body)-2 >= logoRows {
		banner = art + "\n"
	}

	var b strings.Builder
	b.WriteString(banner)
	b.WriteString(center(readiness(cfg, s, width), width) + "\n\n")
	b.WriteString(body)
	b.WriteString(errorLine(s, width))
	return b.String()
}

func errorLine(s dataStats, width int) string {
	if s.Err == nil {
		return ""
	}
	return "\n" + styleError.Render(truncate(s.Err.Error(), width))
}

// readiness answers the only question the dashboard exists to answer: can the
// agent run right now, and how stale is what is on disk.
func readiness(cfg *config.Config, s dataStats, width int) string {
	sources := len(cfg.EnabledSources())

	ready := 0
	for _, p := range providerStatuses(cfg) {
		if p.ready {
			ready++
		}
	}

	mark, label := styleOK.Render("●"), styleOK.Render("ready")
	switch {
	case ready == 0 || sources == 0:
		mark, label = styleError.Render("●"), styleError.Render("not runnable")
	case len(s.Silent) > 0:
		mark, label = styleWarn.Render("●"), styleWarn.Render("sources silent")
	case ready < len(providerStatuses(cfg)):
		mark, label = styleWarn.Render("●"), styleWarn.Render("degraded")
	}

	head := mark + " " + label
	sub := fmt.Sprintf("%d sources  ·  %s", sources, lastRun(s))
	combined := head + styleDim.Render("  ·  "+sub)
	if lipgloss.Width(combined) <= width {
		return combined
	}
	return head + "\n" + styleDim.Render(truncate(sub, width))
}

func lastRun(s dataStats) string {
	if s.LastRun == nil {
		return "never run"
	}
	return "last run " + relativeTime(*s.LastRun)
}

// funnelStage is one narrowing of the pipeline: how many survived it, and the
// one number that says what happened to them.
type funnelStage struct {
	label string
	n     int
	note  string
}

func (d *dashboard) funnel() []funnelStage {
	s := d.stats
	return []funnelStage{
		{"items", s.Items, fmt.Sprintf("%d days", s.ItemDays)},
		{"clusters", s.Clusters, fmt.Sprintf("%d multi", s.MultiSource)},
		{"ranked", s.Ranked, fmt.Sprintf("%d maj·%d not", s.Major, s.Notable)},
		{"stories", s.Stories, fmt.Sprintf("%d md", s.Markdown)},
	}
}

// peak is the widest stage, which every bar is drawn against.
func peak(stages []funnelStage) int {
	out := 1
	for _, st := range stages {
		out = max(out, st.n)
	}
	return out
}

const funnelNoteWidth = 14

// funnelTall stacks each stage over its own bar, for the rail.
func (d *dashboard) funnelTall(inner int) string {
	if !d.loaded {
		return styleDim.Render("reading data/…")
	}

	stages := d.funnel()
	top := peak(stages)
	barWidth := max(inner-funnelNoteWidth-1, 4)

	var b strings.Builder
	for _, st := range stages {
		b.WriteString(styleLabel.Render(st.label) +
			styleValue.Render(padLeft(fmt.Sprint(st.n), max(inner-len(st.label), 1))) + "\n" +
			bar(st.n, top, barWidth) + " " +
			styleDim.Render(truncate(st.note, funnelNoteWidth)) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// funnelWide puts each stage on one line, for a full-width column.
func (d *dashboard) funnelWide(inner int) string {
	if !d.loaded {
		return styleDim.Render("reading data/…")
	}

	stages := d.funnel()
	top := peak(stages)

	const labelWidth, countWidth = 9, 6
	barWidth := max(inner-labelWidth-countWidth-funnelNoteWidth-3, 4)

	var b strings.Builder
	for _, st := range stages {
		b.WriteString(styleLabel.Render(pad(st.label, labelWidth)) +
			styleValue.Render(padLeft(fmt.Sprint(st.n), countWidth)) + " " +
			bar(st.n, top, barWidth) + " " +
			styleDim.Render(truncate(st.note, funnelNoteWidth)) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// meterWidths is the one geometry every bar row uses: a label, a count right
// against it, and the rest for the bar. Sharing it is what makes rows in
// different panels line up when they are stacked in the same column.
func meterWidths(inner int) (labelWidth, countWidth, barWidth int) {
	labelWidth = min(16, max(inner/3, 8))
	countWidth = 6
	return labelWidth, countWidth, max(inner-labelWidth-countWidth-2, 4)
}

// meter renders one "label  count  bar" row. Pass draw as nil to spend the
// bar's room on trailing text instead — a row that has something to say rather
// than something to measure still lines up with the rows around it.
func meter(label, count string, style lipgloss.Style, inner int,
	draw func(width int) string, note string) string {

	labelWidth, countWidth, barWidth := meterWidths(inner)

	out := style.Render(pad(truncate(label, labelWidth), labelWidth)) +
		styleValue.Render(padLeft(count, countWidth)) + " "
	if draw != nil {
		return out + draw(barWidth) + "\n"
	}
	return out + styleDim.Render(truncate(note, barWidth)) + "\n"
}

// ingestBody ranks the sources actually feeding the pipeline, and names the
// enabled ones that fed it nothing.
func ingestBody(s dataStats, inner int) string {
	if len(s.BySource) == 0 && len(s.Silent) == 0 {
		return styleDim.Render("no items in the window")
	}

	top := 1
	for _, c := range s.BySource {
		top = max(top, c.n)
	}

	var b strings.Builder
	for _, c := range s.BySource {
		b.WriteString(meter(c.name, fmt.Sprint(c.n), styleLabel, inner,
			func(w int) string { return bar(c.n, top, w) }, ""))
	}
	if len(s.Silent) > 0 {
		b.WriteString(meter("silent", fmt.Sprint(len(s.Silent)), styleWarn, inner,
			nil, strings.Join(s.Silent, ", ")))
	}
	if s.WithSignal > 0 {
		b.WriteString(meter("with signal", fmt.Sprint(s.WithSignal), styleLabel, inner,
			nil, fmt.Sprintf("of %d items", s.Items)))
	}
	return strings.TrimRight(b.String(), "\n")
}

// outputBody is what actually reached the site: the tier split, and the story
// currently at the top of it.
func outputBody(s dataStats, inner int) string {
	if s.Stories == 0 {
		return styleDim.Render("nothing published yet")
	}

	tiers := []struct {
		label string
		n     int
		style lipgloss.Style
	}{
		{"major", s.StoryMajor, tierStyle(1)},
		{"notable", s.StoryNotable, tierStyle(3)},
		{"minor", s.StoryMinor, tierStyle(5)},
	}
	top := 1
	for _, t := range tiers {
		top = max(top, t.n)
	}

	var b strings.Builder
	for _, t := range tiers {
		b.WriteString(meter(t.label, fmt.Sprint(t.n), t.style, inner,
			func(w int) string {
				return t.style.Render(pad(repeat("█", scale(t.n, top, w)), w))
			}, ""))
	}

	b.WriteString(meter("builder-relevant", fmt.Sprint(s.BuilderRel), styleLabel, inner,
		nil, fmt.Sprintf("of %d stories", s.Stories)))
	b.WriteString(meter("avg cluster", fmt.Sprintf("%.1f", s.AvgCluster), styleLabel, inner,
		nil, "sources per story"))
	if s.NewestStory != nil {
		b.WriteString(meter("newest", "", styleLabel, inner, nil, relativeTime(*s.NewestStory)))
	}
	if s.TopStory != "" {
		b.WriteString(meter("top", fmt.Sprint(s.TopScore), styleLabel, inner, nil, s.TopStory))
	}
	return strings.TrimRight(b.String(), "\n")
}

// topicsBody shows what the tag extractor is actually finding.
func topicsBody(s dataStats, inner int) string {
	if len(s.TopTags) == 0 {
		return styleDim.Render("no tags on the published stories")
	}

	top := 1
	for _, c := range s.TopTags {
		top = max(top, c.n)
	}

	var b strings.Builder
	for _, c := range s.TopTags {
		b.WriteString(meter(c.name, fmt.Sprint(c.n), styleLabel, inner,
			func(w int) string { return bar(c.n, top, w) }, ""))
	}
	return strings.TrimRight(b.String(), "\n")
}

func readyProviders(cfg *config.Config) int {
	ready := 0
	for _, p := range providerStatuses(cfg) {
		if p.ready {
			ready++
		}
	}
	return ready
}

// engineBody combines provider health and core engine parameters into a single
// high-density panel.
func engineBody(cfg *config.Config, inner int) string {
	var b strings.Builder
	for _, p := range providerStatuses(cfg) {
		mark := styleDim.Render(pad("missing", 8))
		if p.ready {
			mark = styleOK.Render(pad("ready", 8))
		}
		nameW := min(12, max(inner/3, 8))
		b.WriteString(styleLabel.Render(pad(p.name, nameW)) + mark +
			styleDim.Render(truncate(p.detail, max(inner-nameW-8, 4))) + "\n")
	}
	b.WriteString(stat("embedder", cfg.Embed.Provider+" · "+cfg.Embed.Model(), inner))
	b.WriteString(stat("threshold", fmt.Sprintf("%.2f over %dh",
		cfg.Scoring.Clustering.SimilarityThreshold, cfg.Scoring.Clustering.WindowHours), inner))
	b.WriteString(stat("half-life", fmt.Sprintf("%.0fh", cfg.Scoring.Recency.HalfLifeHours), inner))
	b.WriteString(stat("llm budget", fmt.Sprintf("%d calls/run",
		cfg.Scoring.Caps.LLMCallsPerRun), inner))
	b.WriteString(stat("paths", fmt.Sprintf("%s / %s", cfg.ConfigDir, cfg.DataDir), inner))
	return strings.TrimRight(b.String(), "\n")
}

func configBody(cfg *config.Config, inner int) string {
	var b strings.Builder
	b.WriteString(stat("sources", fmt.Sprintf("%d of %d enabled",
		len(cfg.EnabledSources()), len(cfg.Sources)), inner))
	b.WriteString(stat("embedder", cfg.Embed.Provider+" · "+cfg.Embed.Model(), inner))
	b.WriteString(stat("threshold", fmt.Sprintf("%.2f over %dh",
		cfg.Scoring.Clustering.SimilarityThreshold, cfg.Scoring.Clustering.WindowHours), inner))
	b.WriteString(stat("half-life", fmt.Sprintf("%.0fh", cfg.Scoring.Recency.HalfLifeHours), inner))
	b.WriteString(stat("llm budget", fmt.Sprintf("%d calls per run",
		cfg.Scoring.Caps.LLMCallsPerRun), inner))
	b.WriteString(stat("config dir", cfg.ConfigDir, inner))
	b.WriteString(stat("data dir", cfg.DataDir, inner))
	return strings.TrimRight(b.String(), "\n")
}

// limitsBody is the shape the rules pin down: where a tier starts, what the
// summarizer may write, and how long anything is kept.
func limitsBody(cfg *config.Config, inner int) string {
	s := cfg.Scoring

	var b strings.Builder
	b.WriteString(stat("major at", fmt.Sprintf("score ≥ %d", s.Tiers.MajorMinScore), inner))
	b.WriteString(stat("notable at", fmt.Sprintf("score ≥ %d", s.Tiers.NotableMinScore), inner))
	b.WriteString(stat("builder at", plural(s.BuilderRelevantMinSignals, "signal"), inner))
	b.WriteString(stat("summary", plural(s.Limits.SummaryMaxWords, "word")+" max", inner))
	b.WriteString(stat("excerpt", plural(s.Limits.ExcerptMaxChars, "char")+" max", inner))
	b.WriteString(stat("quotes", fmt.Sprintf("%d × %d words",
		s.Limits.QuotesMax, s.Limits.QuoteMaxWords), inner))
	b.WriteString(stat("retention", fmt.Sprintf("items %dd · urls %dd · cache %dd",
		s.Retention.ItemsDays, s.Retention.SeenURLsDays, s.Retention.EmbeddingCacheDays), inner))
	return strings.TrimRight(b.String(), "\n")
}

// storageBody is the footprint, including the cache that must never be
// committed and therefore never gets looked at.
func storageBody(s dataStats, inner int) string {
	var b strings.Builder
	b.WriteString(stat("last run", lastRun(s), inner))
	b.WriteString(stat("items", humanBytes(s.ItemsBytes), inner))
	b.WriteString(stat("embed cache", humanBytes(s.CacheBytes), inner))
	b.WriteString(stat("seen urls", fmt.Sprint(s.SeenURLs), inner))
	b.WriteString(stat("cursors", fmt.Sprint(s.Cursors), inner))
	b.WriteString(stat("markdown", plural(s.Markdown, "file"), inner))
	b.WriteString(stat("digest", plural(s.DigestFiles, "file"), inner))
	return strings.TrimRight(b.String(), "\n")
}

func providersBody(cfg *config.Config, inner int) string {
	var b strings.Builder
	for _, p := range providerStatuses(cfg) {
		mark := styleDim.Render(pad("missing", 8))
		if p.ready {
			mark = styleOK.Render(pad("ready", 8))
		}
		b.WriteString(styleLabel.Render(pad(p.name, 12)) + mark +
			styleDim.Render(truncate(p.detail, max(inner-20, 6))) + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// tierBody groups the enabled sources the way scoring weights them, so a tier
// that has quietly emptied out is visible at a glance.
func tierBody(cfg *config.Config, inner int) string {
	byTier := map[int][]string{}
	for _, src := range cfg.EnabledSources() {
		byTier[src.Tier] = append(byTier[src.Tier], src.ID)
	}
	top := 1
	for _, ids := range byTier {
		top = max(top, len(ids))
	}

	var b strings.Builder
	for tier := 1; tier <= 5; tier++ {
		ids := byTier[tier]
		if len(ids) == 0 {
			continue
		}
		// The bar is capped short here so the weight and the source names —
		// the part an operator actually reads — keep their room.
		b.WriteString(meter(fmt.Sprintf("tier %d", tier), fmt.Sprint(len(ids)),
			tierStyle(tier), inner, func(w int) string {
				const weightWidth = 7
				cells := min(8, max(w-weightWidth-4, 1))
				return tierStyle(tier).Render(pad(repeat("█", scale(len(ids), top, cells)), cells)) +
					styleDim.Render(pad(fmt.Sprintf(" ×%.2f ", cfg.Scoring.TierWeight(tier)), weightWidth)) +
					styleDim.Render(truncate(strings.Join(ids, ", "), max(w-cells-weightWidth, 4)))
			}, ""))
	}
	if b.Len() == 0 {
		return styleWarn.Render("no sources enabled")
	}
	return strings.TrimRight(b.String(), "\n")
}

type providerStatus struct {
	name   string
	ready  bool
	detail string
}

// providerStatuses reports which providers are configured. This checks that a
// key is present, not that it works — `doctor` makes the live calls.
func providerStatuses(cfg *config.Config) []providerStatus {
	return []providerStatus{
		{"nvidia_nim", cfg.LLM.NvidiaNIM.Configured(), cfg.LLM.NvidiaNIM.Model},
		{"openrouter", cfg.LLM.OpenRouter.Configured(), cfg.LLM.OpenRouter.Model},
	}
}

// stat renders one "label  value" row, giving the label at most half the room.
func stat(label, value string, inner int) string {
	labelWidth := min(18, max(inner/2, 6))
	return styleLabel.Render(pad(label, labelWidth)) +
		styleValue.Render(truncate(value, max(inner-labelWidth, 4))) + "\n"
}

// plural writes "1 item" but "2 items".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
