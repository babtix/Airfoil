package tui

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/papitsho/airfoil/internal/config"
)

// testConfig writes a minimal but realistic config tree to a temp dir and loads
// it, so save paths are exercised against real files.
func testConfig(t *testing.T) *config.Config {
	t.Helper()

	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}

	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(configDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("sources.json", `{
	  "$comment": "operator note that must survive an edit",
	  "sources": [
	    {"id":"openai","name":"OpenAI","type":"rss","url":"https://openai.com/rss","tier":1,"tags":["labs"],"enabled":true},
	    {"$comment":"no public feed","id":"anthropic","name":"Anthropic","type":"rss","url":"https://x/rss","tier":1,"enabled":false},
	    {"id":"hn","name":"Hacker News","type":"hn","url":"https://hn.algolia.com/api/v1/search_by_date","tier":5,
	     "enabled":true,"options":{"queries":["AI","LLM"],"min_points":50,"hours_back":48}}
	  ]
	}`)

	write("scoring.json", `{
	  "$comment": "tune against real data",
	  "clustering": {"similarity_threshold":0.82,"window_hours":48,"debug_range":[0.75,0.90]},
	  "weights": {"sources":22,"tier":25,"hn":6,"reddit":5,"builder":4,"hype":6},
	  "tier_weights": {"1":1.0,"2":0.85,"3":0.8,"4":0.5,"5":0.3},
	  "recency": {"half_life_hours":48},
	  "caps": {"builder_signal_max":5,"hype_signal_max":5,"llm_calls_per_run":15},
	  "tiers": {"major_min_score":70,"notable_min_score":40},
	  "builder_relevant_min_signals": 2,
	  "limits": {"excerpt_max_chars":300,"summary_max_words":80,"takeaways_max":3,
	             "verbatim_overlap_max_words":12,"quotes_max":1,"quote_max_words":15},
	  "retention": {"items_days":30,"seen_urls_days":30,"embedding_cache_days":7}
	}`)

	write("keywords.json", `{
	  "$comment": "matched case-insensitively",
	  "builder_signals": ["release","open source","api"],
	  "builder_structural_signals": ["has_repo_url"],
	  "hype_signals": ["insane","game-changer"],
	  "topic_tags": {"models":["gpt","claude"],"agents":["agent","mcp"]}
	}`)

	t.Setenv("AIRFOIL_CONFIG_DIR", configDir)
	t.Setenv("AIRFOIL_DATA_DIR", filepath.Join(dir, "data"))

	cfg, err := config.Load(config.Options{ConfigDir: configDir, DataDir: filepath.Join(dir, "data")})
	if err != nil {
		t.Fatalf("config.Load() = %v", err)
	}
	return cfg
}

func testModel(t *testing.T) *Model {
	t.Helper()

	cfg := testConfig(t)
	sink := NewLogSink(100, slog.LevelInfo)
	m := New(cfg, slog.New(sink), sink)

	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return m
}

// key sends a keypress to the model.
func key(m *Model, s string) {
	switch s {
	case "enter":
		m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	case "esc":
		m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	case "up":
		m.Update(tea.KeyMsg{Type: tea.KeyUp})
	case "down":
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	case "left":
		m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	case "right":
		m.Update(tea.KeyMsg{Type: tea.KeyRight})
	case " ":
		m.Update(tea.KeyMsg{Type: tea.KeySpace})
	case "tab":
		m.Update(tea.KeyMsg{Type: tea.KeyTab})
	case "ctrl+u":
		m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	default:
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	}
}

func typeText(m *Model, s string) {
	for _, r := range s {
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// --- rendering --------------------------------------------------------------

func TestEveryPageRenders(t *testing.T) {
	m := testModel(t)

	for _, tab := range tabs {
		t.Run(tab.label, func(t *testing.T) {
			m.view = tab.id
			out := m.View()

			if out == "" {
				t.Fatal("View() returned nothing")
			}
			if !strings.Contains(out, "AIRFOIL") {
				t.Error("View() is missing the header")
			}
		})
	}
}

// A narrow or short terminal must not panic or produce runaway lines.
func TestRendersAtAwkwardSizes(t *testing.T) {
	sizes := []struct{ w, h int }{
		{20, 6}, {40, 10}, {80, 24}, {200, 60}, {1, 1},
	}

	for _, size := range sizes {
		m := testModel(t)
		m.Update(tea.WindowSizeMsg{Width: size.w, Height: size.h})

		for _, tab := range tabs {
			m.view = tab.id
			out := m.View() // must not panic

			for _, line := range strings.Split(out, "\n") {
				if got := len([]rune(stripANSI(line))); got > size.w+2 {
					t.Errorf("%dx%d tab %q: line of %d cells exceeds width",
						size.w, size.h, tab.label, got)
					break
				}
			}
		}
	}
}

func TestHelpOverlay(t *testing.T) {
	m := testModel(t)

	key(m, "?")
	if !m.showHelp {
		t.Fatal("? did not open help")
	}
	if out := m.View(); !strings.Contains(out, "keys") {
		t.Error("help view is missing its heading")
	}

	key(m, "?")
	if m.showHelp {
		t.Error("? did not close help")
	}
}

// --- navigation -------------------------------------------------------------

func TestTabNavigation(t *testing.T) {
	m := testModel(t)

	if m.view != viewDashboard {
		t.Fatalf("started on %v, want the dashboard", m.view)
	}

	key(m, "3")
	if m.view != viewSources {
		t.Errorf("3 went to %v, want sources", m.view)
	}

	key(m, "tab")
	if m.view != viewScoring {
		t.Errorf("tab went to %v, want scoring", m.view)
	}

	key(m, "9")
	if m.view != viewDigest {
		t.Errorf("9 went to %v, want digest", m.view)
	}

	key(m, "0")
	if m.view != viewPurge {
		t.Errorf("0 went to %v, want purge", m.view)
	}

	key(m, "1")
	if m.view != viewDashboard {
		t.Errorf("1 went to %v, want the dashboard", m.view)
	}
}

// A digit typed into a text field must edit the field, not switch tabs.
func TestTextInputCapturesDigits(t *testing.T) {
	m := testModel(t)
	key(m, "4") // scoring

	key(m, "enter")  // start editing the similarity threshold
	key(m, "ctrl+u") // clear the pre-filled value
	typeText(m, "0.9")

	if m.view != viewScoring {
		t.Fatalf("typing digits switched to %v", m.view)
	}

	key(m, "enter")
	if got := m.cfg.Scoring.Clustering.SimilarityThreshold; got == 0.9 {
		t.Error("the edit reached the live config before being saved")
	}

	page := m.pages[viewScoring].(*scoringPage)
	if got := page.scoring.Clustering.SimilarityThreshold; got != 0.9 {
		t.Errorf("threshold = %v, want 0.9", got)
	}
	if !page.dirty {
		t.Error("the page is not marked dirty after an edit")
	}
}

// --- sources editing --------------------------------------------------------

func TestSourcesToggleAndSave(t *testing.T) {
	m := testModel(t)
	key(m, "3")

	page := m.pages[viewSources].(*sourcesPage)
	before := page.sources[0].Enabled

	key(m, " ")
	if page.sources[0].Enabled == before {
		t.Fatal("space did not toggle the source")
	}
	if !page.dirty {
		t.Error("toggling did not mark the page dirty")
	}
	// An in-memory edit must not reach disk until saved.
	if m.cfg.Sources[0].Enabled != before {
		t.Error("the toggle changed the live config before saving")
	}

	key(m, "s")
	if page.dirty {
		t.Error("still dirty after save")
	}
	if m.cfg.Sources[0].Enabled == before {
		t.Error("save did not update the live config")
	}

	reloaded, err := config.Load(config.Options{ConfigDir: m.cfg.ConfigDir, DataDir: m.cfg.DataDir})
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Sources[0].Enabled == before {
		t.Error("save did not reach disk")
	}
}

// The config files carry operator notes; an edit through the interface must not
// silently delete them.
func TestSourcesSavePreservesComments(t *testing.T) {
	m := testModel(t)
	key(m, "3")
	key(m, " ")
	key(m, "s")

	raw, err := os.ReadFile(m.cfg.SourcesPath())
	if err != nil {
		t.Fatal(err)
	}

	var file struct {
		Comment string `json:"$comment"`
		Sources []struct {
			ID      string `json:"id"`
			Comment string `json:"$comment"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("saved file does not parse: %v", err)
	}

	if file.Comment == "" {
		t.Error("the top-level $comment was dropped on save")
	}
	for _, s := range file.Sources {
		if s.ID == "anthropic" && s.Comment == "" {
			t.Error("the per-source $comment was dropped on save")
		}
	}
}

func TestSourcesReloadDiscardsEdits(t *testing.T) {
	m := testModel(t)
	key(m, "3")

	page := m.pages[viewSources].(*sourcesPage)
	before := page.sources[0].Enabled

	key(m, " ")
	key(m, "r")

	if page.sources[0].Enabled != before {
		t.Error("reload did not discard the unsaved toggle")
	}
	if page.dirty {
		t.Error("still dirty after reload")
	}
}

func TestSourcesDelete(t *testing.T) {
	m := testModel(t)
	key(m, "3")

	page := m.pages[viewSources].(*sourcesPage)
	before := len(page.sources)

	key(m, "D")
	if len(page.sources) != before-1 {
		t.Errorf("got %d sources, want %d", len(page.sources), before-1)
	}
}

func TestSourceFormEditsField(t *testing.T) {
	m := testModel(t)
	key(m, "3")
	key(m, "enter") // open the form

	page := m.pages[viewSources].(*sourcesPage)
	if page.editing == nil {
		t.Fatal("enter did not open the form")
	}

	key(m, "down") // name
	key(m, "enter")
	key(m, "ctrl+u") // clear the pre-filled value
	typeText(m, "Renamed")
	key(m, "enter")

	if got := page.editing.src.Name; got != "Renamed" {
		t.Errorf("name = %q, want %q", got, "Renamed")
	}

	key(m, "s") // apply back to the list
	if page.editing != nil {
		t.Error("the form is still open after applying")
	}
	if got := page.sources[0].Name; got != "Renamed" {
		t.Errorf("list shows %q, want %q", got, "Renamed")
	}
}

// The form only shows options that the selected adapter actually reads.
func TestSourceFormHidesIrrelevantOptions(t *testing.T) {
	rss := newSourceForm(0, config.Source{Type: config.SourceRSS})
	hn := newSourceForm(0, config.Source{Type: config.SourceHN})

	hasField := func(f *sourceForm, label string) bool {
		for _, field := range f.visibleFields() {
			if field.label == label {
				return true
			}
		}
		return false
	}

	if hasField(rss, "min points") {
		t.Error("an rss source is offering the Hacker News min points option")
	}
	if !hasField(hn, "min points") {
		t.Error("a hn source is missing the min points option")
	}
	if !hasField(rss, "url") {
		t.Error("every source needs a url field")
	}
}

func TestSourceFormRejectsInvalid(t *testing.T) {
	m := testModel(t)
	key(m, "3")
	key(m, "n") // new source, which starts with an empty id

	key(m, "s") // try to apply
	page := m.pages[viewSources].(*sourcesPage)

	if page.editing == nil {
		t.Fatal("an invalid source was accepted")
	}
	if !m.statusErr {
		t.Error("no error was reported for the invalid source")
	}
}

// --- scoring editing --------------------------------------------------------

func TestScoringNudge(t *testing.T) {
	m := testModel(t)
	key(m, "4")

	page := m.pages[viewScoring].(*scoringPage)
	before := page.scoring.Clustering.SimilarityThreshold

	key(m, "right")
	if got := page.scoring.Clustering.SimilarityThreshold; got != before+0.02 {
		t.Errorf("threshold = %v, want %v", got, before+0.02)
	}

	key(m, "left")
	if got := page.scoring.Clustering.SimilarityThreshold; got != before {
		t.Errorf("threshold = %v, want it back at %v", got, before)
	}
}

// Repeated nudges must land on exact values, not accumulate float noise.
func TestScoringNudgeStaysExact(t *testing.T) {
	m := testModel(t)
	key(m, "4")

	page := m.pages[viewScoring].(*scoringPage)
	for range 5 {
		key(m, "left")
	}

	if got := page.scoring.Clustering.SimilarityThreshold; got != 0.72 {
		t.Errorf("threshold = %v, want exactly 0.72", got)
	}
}

func TestScoringNudgeRespectsBounds(t *testing.T) {
	m := testModel(t)
	key(m, "4")

	page := m.pages[viewScoring].(*scoringPage)
	for range 100 {
		key(m, "right")
	}

	if got := page.scoring.Clustering.SimilarityThreshold; got > 1 {
		t.Errorf("threshold = %v, want it clamped to 1", got)
	}
}

func TestScoringSaveRejectsInvalid(t *testing.T) {
	m := testModel(t)
	key(m, "4")

	page := m.pages[viewScoring].(*scoringPage)
	// Inverting the story tiers is exactly what config validation rejects.
	page.scoring.Tiers.MajorMinScore = 10
	page.scoring.Tiers.NotableMinScore = 90

	key(m, "s")
	if !m.statusErr {
		t.Error("an invalid scoring config was saved without complaint")
	}
	if m.cfg.Scoring.Tiers.MajorMinScore == 10 {
		t.Error("the invalid config reached the live config")
	}
}

func TestScoringSaveRoundTrip(t *testing.T) {
	m := testModel(t)
	key(m, "4")
	key(m, "right")
	key(m, "s")

	if m.statusErr {
		t.Fatalf("save failed: %s", m.status)
	}

	reloaded, err := config.Load(config.Options{ConfigDir: m.cfg.ConfigDir, DataDir: m.cfg.DataDir})
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Scoring.Clustering.SimilarityThreshold; got != 0.84 {
		t.Errorf("threshold on disk = %v, want 0.84", got)
	}
	if reloaded.Scoring.Comment == "" {
		t.Error("the $comment was dropped from scoring.json")
	}
}

// Headings are labels, not fields; the cursor must skip over them.
func TestScoringCursorSkipsHeadings(t *testing.T) {
	m := testModel(t)
	key(m, "4")

	page := m.pages[viewScoring].(*scoringPage)
	for range 40 {
		key(m, "down")
		if page.rows[page.cursor].isHeading() {
			t.Fatalf("cursor landed on the heading %q", page.rows[page.cursor].section)
		}
	}
}

// --- keywords editing -------------------------------------------------------

func TestKeywordsAddAndDelete(t *testing.T) {
	m := testModel(t)
	key(m, "5")

	page := m.pages[viewKeywords].(*keywordsPage)
	before := len(page.entries())

	key(m, "n")
	typeText(m, "fine-tune")
	key(m, "enter")

	if got := len(page.entries()); got != before+1 {
		t.Fatalf("got %d entries, want %d", got, before+1)
	}
	if got := page.entries()[before]; got != "fine-tune" {
		t.Errorf("added %q, want %q", got, "fine-tune")
	}

	key(m, "D")
	if got := len(page.entries()); got != before {
		t.Errorf("got %d entries after delete, want %d", got, before)
	}
}

func TestKeywordsRejectsEmptyEntry(t *testing.T) {
	m := testModel(t)
	key(m, "5")

	page := m.pages[viewKeywords].(*keywordsPage)
	before := len(page.entries())

	key(m, "n")
	key(m, "enter") // accept nothing

	if got := len(page.entries()); got != before {
		t.Errorf("an empty entry was added")
	}
	if !m.statusErr {
		t.Error("no error was reported for the empty entry")
	}
}

func TestKeywordsListSelection(t *testing.T) {
	m := testModel(t)
	key(m, "5")

	page := m.pages[viewKeywords].(*keywordsPage)
	if page.current().label != "builder signals" {
		t.Fatalf("started on %q", page.current().label)
	}

	key(m, "down")
	if page.current().label != "hype signals" {
		t.Errorf("moved to %q, want hype signals", page.current().label)
	}

	// Every topic tag is reachable, so no phrase in the file is orphaned.
	labels := map[string]bool{}
	for _, l := range page.lists {
		labels[l.label] = true
	}
	for _, want := range []string{"tag: models", "tag: agents"} {
		if !labels[want] {
			t.Errorf("no list for %q", want)
		}
	}
}

func TestKeywordsSaveRoundTrip(t *testing.T) {
	m := testModel(t)
	key(m, "5")

	key(m, "n")
	typeText(m, "quantized")
	key(m, "enter")
	key(m, "s")

	if m.statusErr {
		t.Fatalf("save failed: %s", m.status)
	}

	reloaded, err := config.Load(config.Options{ConfigDir: m.cfg.ConfigDir, DataDir: m.cfg.DataDir})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range reloaded.Keywords.BuilderSignals {
		if s == "quantized" {
			found = true
		}
	}
	if !found {
		t.Error("the new signal is not on disk")
	}
	if reloaded.Keywords.Comment == "" {
		t.Error("the $comment was dropped from keywords.json")
	}
	if len(reloaded.Keywords.TopicTags) != 2 {
		t.Errorf("topic tags = %d, want both preserved", len(reloaded.Keywords.TopicTags))
	}
}

// --- log sink ---------------------------------------------------------------

func TestLogSinkCapturesOutput(t *testing.T) {
	sink := NewLogSink(10, slog.LevelInfo)
	log := slog.New(sink)

	log.Info("ingest complete", "new_items", 42)

	lines := sink.Lines()
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	if !strings.Contains(lines[0], "ingest complete") {
		t.Errorf("line = %q, missing the message", lines[0])
	}
	if !strings.Contains(lines[0], "42") {
		t.Errorf("line = %q, missing the attribute", lines[0])
	}
}

// A handler derived by WithAttrs must write into the same display, not a
// private buffer of its own.
func TestLogSinkSharesBufferAcrossDerivedHandlers(t *testing.T) {
	sink := NewLogSink(10, slog.LevelInfo)

	slog.New(sink).With("stage", "ingest").Info("started")
	slog.New(sink).WithGroup("embed").Info("done", "dims", 1024)

	if got := len(sink.Lines()); got != 2 {
		t.Errorf("got %d lines, want 2 — a derived handler lost its output", got)
	}
}

func TestLogSinkRespectsLevel(t *testing.T) {
	sink := NewLogSink(10, slog.LevelWarn)
	log := slog.New(sink)

	log.Info("not shown")
	log.Warn("shown")

	lines := sink.Lines()
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want only the warning", len(lines))
	}
	if !strings.Contains(lines[0], "shown") {
		t.Errorf("line = %q", lines[0])
	}
}

func TestLogSinkBoundsMemory(t *testing.T) {
	sink := NewLogSink(5, slog.LevelInfo)
	log := slog.New(sink)

	for i := range 50 {
		log.Info("line", "n", i)
	}

	lines := sink.Lines()
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want the buffer capped at 5", len(lines))
	}
	// The most recent output is what survives.
	if !strings.Contains(lines[len(lines)-1], "49") {
		t.Errorf("last line = %q, want the newest", lines[len(lines)-1])
	}
}

func TestLogSinkClear(t *testing.T) {
	sink := NewLogSink(10, slog.LevelInfo)
	slog.New(sink).Info("something")

	sink.Clear()
	if got := len(sink.Lines()); got != 0 {
		t.Errorf("got %d lines after Clear, want 0", got)
	}
}

// stripANSI removes escape sequences so width assertions measure real cells.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false

	for _, r := range s {
		switch {
		case r == '\x1b':
			inEscape = true
		case inEscape && (r == 'm' || r == 'K' || r == 'H'):
			inEscape = false
		case !inEscape:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// --- stage coverage ---------------------------------------------------------

// Every pipeline stage must be reachable from the interface. This is the test
// that fails when a stage is added to the pipeline but not surfaced here.
func TestEveryPipelineStageIsExposed(t *testing.T) {
	want := []string{"ingest", "cluster", "rank", "write", "digest", "publish", "run"}

	got := make(map[string]bool, len(stages))
	for _, s := range stages {
		got[s.key] = true
	}
	for _, key := range want {
		if !got[key] {
			t.Errorf("stage %q is not reachable from the Run tab", key)
		}
	}
	if len(stages) != len(want) {
		t.Errorf("got %d stages, want %d", len(stages), len(want))
	}
}

func TestStagesAreWellFormed(t *testing.T) {
	for _, s := range stages {
		if s.key == "" || s.name == "" || s.about == "" {
			t.Errorf("stage %+v is missing a label", s)
		}
		if s.run == nil {
			t.Errorf("stage %q has no implementation", s.key)
		}
	}
}

func TestRunPageStageNavigation(t *testing.T) {
	m := testModel(t)
	key(m, "2")

	page := m.pages[viewRun].(*runPage)
	if page.cursor != 0 {
		t.Fatalf("cursor started at %d", page.cursor)
	}

	for range len(stages) + 5 {
		key(m, "down")
	}
	if page.cursor != len(stages)-1 {
		t.Errorf("cursor = %d, want it clamped to %d", page.cursor, len(stages)-1)
	}

	for range len(stages) + 5 {
		key(m, "up")
	}
	if page.cursor != 0 {
		t.Errorf("cursor = %d, want it clamped to 0", page.cursor)
	}
}

func TestRunPageTogglesDryAndPush(t *testing.T) {
	m := testModel(t)
	key(m, "2")
	page := m.pages[viewRun].(*runPage)

	if page.dry || page.push {
		t.Fatal("dry and push should both start off")
	}

	key(m, "d")
	if !page.dry {
		t.Error("d did not turn dry run on")
	}
	key(m, "p")
	if !page.push {
		t.Error("p did not arm push")
	}
	key(m, "d")
	key(m, "p")
	if page.dry || page.push {
		t.Error("the toggles did not turn back off")
	}
}

func TestRunPageLoopToggleAndInterval(t *testing.T) {
	m := testModel(t)
	key(m, "2")
	page := m.pages[viewRun].(*runPage)

	if page.loop {
		t.Fatal("loop should start off")
	}
	if page.interval != 24*time.Hour {
		t.Errorf("interval = %v, want 24h", page.interval)
	}

	key(m, "l")
	if !page.loop {
		t.Error("l did not turn loop mode on")
	}
	if page.nextRun.IsZero() {
		t.Error("nextRun was not set after turning loop on")
	}

	// Test interval cycling with L
	key(m, "L")
	if page.interval != 12*time.Hour {
		t.Errorf("interval = %v, want 12h", page.interval)
	}

	key(m, "L")
	if page.interval != 6*time.Hour {
		t.Errorf("interval = %v, want 6h", page.interval)
	}

	// Test turning loop off
	key(m, "l")
	if page.loop {
		t.Error("l did not turn loop mode off")
	}
	if !page.nextRun.IsZero() {
		t.Error("nextRun was not cleared after turning loop off")
	}
}

// Pushing reaches outside this machine, so it must never start on one keypress.
func TestPublishAsksBeforePushing(t *testing.T) {
	m := testModel(t)
	key(m, "2")
	page := m.pages[viewRun].(*runPage)

	page.cursor = stageIndex(t, "publish")
	key(m, "p") // arm push
	key(m, "enter")

	if page.confirming == nil {
		t.Fatal("publish with push armed started without asking")
	}
	if page.running {
		t.Fatal("the stage started before the confirmation was answered")
	}

	key(m, "n")
	if page.confirming != nil {
		t.Error("n did not dismiss the confirmation")
	}
	if page.running {
		t.Error("n started the stage anyway")
	}
}

// Without push armed, publish only commits locally and needs no confirmation.
func TestPublishWithoutPushDoesNotAsk(t *testing.T) {
	m := testModel(t)
	key(m, "2")
	page := m.pages[viewRun].(*runPage)

	page.cursor = stageIndex(t, "publish")
	key(m, "enter")

	if page.confirming != nil {
		t.Error("publish asked for confirmation when push was not armed")
	}
}

// A dry run cannot reach the remote, so it needs no confirmation either.
func TestPublishDryDoesNotAsk(t *testing.T) {
	m := testModel(t)
	key(m, "2")
	page := m.pages[viewRun].(*runPage)

	page.cursor = stageIndex(t, "publish")
	key(m, "p")
	key(m, "d")
	key(m, "enter")

	if page.confirming != nil {
		t.Error("a dry publish asked to confirm a push it cannot perform")
	}
}

// Stages needing a provider must say so rather than failing deep in the run.
func TestLLMStagesRefuseWithoutAProvider(t *testing.T) {
	for _, name := range []string{"write", "digest"} {
		t.Run(name, func(t *testing.T) {
			m := testModel(t)
			t.Setenv("NVIDIA_NIM_API_KEY", "")
			t.Setenv("OPENROUTER_API_KEY", "")

			key(m, "2")
			page := m.pages[viewRun].(*runPage)
			page.cursor = stageIndex(t, name)

			if m.pipe.HasLLM() {
				t.Skip("a provider is configured in this environment")
			}
			key(m, "enter")

			if page.running {
				t.Error("the stage started without a provider")
			}
			if !m.statusErr {
				t.Error("no error explained the missing provider")
			}
			if !strings.Contains(m.status, "API_KEY") {
				t.Errorf("status = %q, want it to name the missing key", m.status)
			}
		})
	}
}

func stageIndex(t *testing.T, key string) int {
	t.Helper()
	for i, s := range stages {
		if s.key == key {
			return i
		}
	}
	t.Fatalf("no stage named %q", key)
	return 0
}

// --- browse coverage --------------------------------------------------------

func TestBrowseCyclesEveryMode(t *testing.T) {
	m := testModel(t)
	key(m, "8")
	page := m.pages[viewBrowse].(*browsePage)

	seen := map[int]bool{page.mode: true}
	for range modeCount {
		key(m, "right")
		seen[page.mode] = true
	}
	if len(seen) != modeCount {
		t.Errorf("reached %d of %d browse modes", len(seen), modeCount)
	}

	// Cycling right all the way round returns to where it started.
	if page.mode != modeItems {
		t.Errorf("mode = %d after a full cycle, want %d", page.mode, modeItems)
	}
}

func TestBrowseModeLabelsAreComplete(t *testing.T) {
	for i, label := range modeLabels {
		if label == "" {
			t.Errorf("browse mode %d has no label", i)
		}
	}
}

// --- digest coverage --------------------------------------------------------

func TestDigestPageCyclesFormats(t *testing.T) {
	m := testModel(t)
	key(m, "9")
	if m.view != viewDigest {
		t.Fatalf("view = %v, want digest", m.view)
	}

	page := m.pages[viewDigest].(*digestPage)
	if page.format != formatNewsletter {
		t.Errorf("format = %v, want newsletter", page.format)
	}

	key(m, "2")
	if page.format != formatLinkedIn {
		t.Errorf("2 went to %v, want linkedin", page.format)
	}

	key(m, "3")
	if page.format != formatThread {
		t.Errorf("3 went to %v, want thread", page.format)
	}

	key(m, "4")
	if page.format != formatHTML {
		t.Errorf("4 went to %v, want html", page.format)
	}

	key(m, "right")
	if page.format != formatNewsletter {
		t.Errorf("right went to %v, want newsletter (wrapped)", page.format)
	}

	key(m, "left")
	if page.format != formatHTML {
		t.Errorf("left went to %v, want html (wrapped)", page.format)
	}
}

func TestDigestPageWithFiles(t *testing.T) {
	m := testModel(t)
	digestDir := filepath.Join(m.cfg.DataDir, "digest")
	if err := os.MkdirAll(digestDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write mock digest files for two dates
	_ = os.WriteFile(filepath.Join(digestDir, "2026-08-25-newsletter.txt"), []byte("AIRFOIL — Aug 25\nNewsletter content"), 0o644)
	_ = os.WriteFile(filepath.Join(digestDir, "2026-08-26-newsletter.txt"), []byte("AIRFOIL — Aug 26\n1. [70] Top story\nSummary here"), 0o644)
	_ = os.WriteFile(filepath.Join(digestDir, "2026-08-26-linkedin.md"), []byte("# LinkedIn draft\nPost text"), 0o644)
	_ = os.WriteFile(filepath.Join(digestDir, "2026-08-26-x.json"), []byte(`{"posts":[{"text":"Post 1"},{"text":"Post 2"}]}`), 0o644)
	_ = os.WriteFile(filepath.Join(digestDir, "2026-08-26-newsletter.html"), []byte("<html><body>Airfoil</body></html>"), 0o644)

	key(m, "9")
	cmd := loadDigestData(m.cfg)
	m.Update(cmd())

	page := m.pages[viewDigest].(*digestPage)
	if len(page.days) != 2 {
		t.Fatalf("got %d days, want 2", len(page.days))
	}
	if page.days[0] != "2026-08-26" {
		t.Errorf("latest day = %q, want 2026-08-26", page.days[0])
	}

	// Test scrolling
	key(m, "down")
	key(m, "up")

	// Test date switching
	key(m, "[")
	if page.dayIdx != 1 {
		t.Errorf("dayIdx = %d, want 1 after [", page.dayIdx)
	}
	key(m, "]")
	if page.dayIdx != 0 {
		t.Errorf("dayIdx = %d, want 0 after ]", page.dayIdx)
	}

	// Test rendering
	out := m.View()
	if !strings.Contains(out, "Newsletter (.txt)") {
		t.Error("missing Newsletter tab in digest view")
	}

	// Test copy trigger
	key(m, "y")
	if !strings.Contains(m.status, "copied") && !m.statusErr {
		// In headless test environments clipboard might return error or success
		t.Logf("copy status: %s", m.status)
	}
}
