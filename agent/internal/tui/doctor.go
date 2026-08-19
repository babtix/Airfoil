package tui

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/sync/errgroup"

	"github.com/papitsho/airfoil/internal/config"
)

// doctorPage makes live calls: every feed URL and every configured provider.
//
// The dashboard reports whether a key is present; this reports whether it
// actually works, which is a different question and the one that matters.
type doctorPage struct {
	checks  []checkResult
	running bool
	cursor  int
	ranAt   time.Time
}

type checkResult struct {
	group  string
	name   string
	ok     bool
	skip   bool
	detail string
	ms     int64
}

type doctorDoneMsg struct {
	checks []checkResult
}

func newDoctorPage() *doctorPage { return &doctorPage{} }

func (p *doctorPage) Init() tea.Cmd { return nil }

func (p *doctorPage) Footer() string {
	if p.running {
		return styleWarn.Render("running checks…")
	}
	return styleKey.Render("enter") + styleFooter.Render(" run checks  ") +
		styleKey.Render("↑↓") + styleFooter.Render(" scroll")
}

func (p *doctorPage) Update(msg tea.Msg, m *Model) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case doctorDoneMsg:
		p.checks, p.running, p.ranAt = msg.checks, false, time.Now()

		failed := 0
		for _, c := range msg.checks {
			if !c.ok && !c.skip {
				failed++
			}
		}
		if failed == 0 {
			m.setStatus("all %d checks passed", len(msg.checks))
		} else {
			m.setStatus("%d of %d checks failed", failed, len(msg.checks))
		}
		return nil, false

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if p.running {
				return nil, true
			}
			p.running = true
			p.cursor = 0
			return runDoctor(m.cfg), true

		case "up", "k":
			if p.cursor > 0 {
				p.cursor--
			}
			return nil, true

		case "down", "j":
			if p.cursor < len(p.checks)-1 {
				p.cursor++
			}
			return nil, true
		}
	}
	return nil, false
}

// runDoctor probes every source and provider concurrently.
func runDoctor(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		client := &http.Client{Timeout: 10 * time.Second}
		sources := cfg.EnabledSources()

		results := make([]checkResult, len(sources))
		g, gctx := errgroup.WithContext(ctx)
		g.SetLimit(6)

		for i, src := range sources {
			g.Go(func() error {
				results[i] = probeSource(gctx, client, src)
				return nil
			})
		}

		providers := providerChecks(cfg)
		providerResults := make([]checkResult, len(providers))
		for i, pc := range providers {
			g.Go(func() error {
				providerResults[i] = pc.run(gctx, client)
				return nil
			})
		}

		_ = g.Wait()
		return doctorDoneMsg{checks: append(results, providerResults...)}
	}
}

// probeSource issues a real request and reports what came back.
func probeSource(ctx context.Context, client *http.Client, src config.Source) checkResult {
	started := time.Now()
	out := checkResult{group: "sources", name: src.ID}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		out.detail = err.Error()
		return out
	}
	req.Header.Set("User-Agent", "airfoil/0.1 (+https://github.com/papitsho/airfoil)")

	resp, err := client.Do(req)
	out.ms = time.Since(started).Milliseconds()
	if err != nil {
		out.detail = truncate(err.Error(), 60)
		return out
	}
	defer resp.Body.Close()

	out.ok = resp.StatusCode == http.StatusOK
	out.detail = resp.Status
	if !out.ok {
		out.detail = resp.Status + "  " + truncate(src.URL, 50)
	}
	return out
}

// providerCheck is one live provider probe.
type providerCheck struct {
	name string
	run  func(ctx context.Context, client *http.Client) checkResult
}

func providerChecks(cfg *config.Config) []providerCheck {
	return []providerCheck{
		{"nvidia_nim", func(ctx context.Context, c *http.Client) checkResult {
			if !cfg.LLM.NvidiaNIM.Configured() {
				return checkResult{group: "providers", name: "nvidia_nim", skip: true, detail: "NVIDIA_NIM_API_KEY not set"}
			}
			return probeAuth(ctx, c, "providers", "nvidia_nim",
				strings.TrimSuffix(cfg.Embed.NIMBaseURL, "/")+"/models",
				map[string]string{"Authorization": "Bearer " + cfg.LLM.NvidiaNIM.APIKey})
		}},

		{"openrouter", func(ctx context.Context, c *http.Client) checkResult {
			if !cfg.LLM.OpenRouter.Configured() {
				return checkResult{group: "providers", name: "openrouter", skip: true, detail: "OPENROUTER_API_KEY not set"}
			}
			// /key reports whether the credential itself is live, which a
			// models listing would not.
			return probeAuth(ctx, c, "providers", "openrouter",
				"https://openrouter.ai/api/v1/key",
				map[string]string{"Authorization": "Bearer " + cfg.LLM.OpenRouter.APIKey})
		}},

		{"embedder", func(ctx context.Context, c *http.Client) checkResult {
			out := checkResult{group: "embedder", name: cfg.Embed.Provider}
			switch cfg.Embed.Provider {
			case config.EmbedderNvidiaNIM:
				if cfg.Embed.NIMKey == "" {
					out.detail = "NVIDIA_NIM_API_KEY not set"
					return out
				}
			}
			out.ok = true
			out.detail = cfg.Embed.Model()
			return out
		}},
	}
}

// probeAuth issues an authenticated GET and treats any non-2xx as a failure,
// so an expired key shows up as failing rather than merely "set".
func probeAuth(ctx context.Context, client *http.Client, group, name, url string, headers map[string]string) checkResult {
	started := time.Now()
	out := checkResult{group: group, name: name}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		out.detail = err.Error()
		return out
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	out.ms = time.Since(started).Milliseconds()
	if err != nil {
		out.detail = truncate(err.Error(), 60)
		return out
	}
	defer resp.Body.Close()

	out.ok = resp.StatusCode >= 200 && resp.StatusCode < 300
	out.detail = resp.Status
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		out.detail = resp.Status + " — the key is present but not accepted"
	}
	return out
}

func (p *doctorPage) View(m *Model, width, height int) string {
	var b strings.Builder

	b.WriteString(styleHeader.Render("doctor"))
	if !p.ranAt.IsZero() {
		b.WriteString(styleDim.Render("   last run " + relativeTime(p.ranAt)))
	}
	b.WriteString("\n\n")

	if p.running {
		b.WriteString(styleWarn.Render("  probing every feed and provider…"))
		return b.String()
	}
	if len(p.checks) == 0 {
		b.WriteString(styleDim.Render("  press enter to probe every feed URL and provider credential"))
		return b.String()
	}

	visible := max(height-3, 4)
	start := 0
	if p.cursor >= visible {
		start = p.cursor - visible + 1
	}
	end := min(start+visible, len(p.checks))

	group := ""
	for i := start; i < end; i++ {
		c := p.checks[i]

		if c.group != group {
			group = c.group
			b.WriteString(styleHeader.Render("  "+group) + "\n")
		}

		mark := styleError.Render("fail")
		switch {
		case c.skip:
			mark = styleDim.Render("skip")
		case c.ok:
			mark = styleOK.Render("ok")
		}

		timing := ""
		if c.ms > 0 {
			timing = styleDim.Render(fmt.Sprintf("%5dms", c.ms))
		}

		row := pad(c.name, 24) + pad(mark, 14) + pad(timing, 10) + styleDim.Render(c.detail)
		if i == p.cursor {
			b.WriteString(styleCursor.Render("▸ ") + row + "\n")
		} else {
			b.WriteString("  " + row + "\n")
		}
	}
	return b.String()
}
