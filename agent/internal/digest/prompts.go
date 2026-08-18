package digest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/papitsho/airfoil/internal/model"
)

// System prompts, verbatim from docs/PROMPTS.md §2–4.
const (
	introSystem = `You write the intro for a daily AI news digest read by engineers.

Write 2-3 sentences framing what today's stories add up to. Find the
through-line if there is one. If there isn't one, say the day was quiet
and move on — do not manufacture a narrative.

Never repeat the story titles verbatim; the reader sees them right below.
No hype. No "the AI world is buzzing". Dry and useful.

Return ONLY a JSON object. No markdown fences.

Schema: {"intro": "2-3 sentences, max 60 words", "theme": "max 6 words, or empty", "quiet_day": false}`

	threadSystem = `Write a Twitter/X thread summarizing today's AI news for engineers.

Format:
- Post 1: hook, max 200 chars, states the single most important thing
  that happened today. No "thread 🧵" cliche. No emoji spam — at most one.
- Posts 2-6: one story each, max 180 chars, most important first.
  Each ends with the story's source URL.
- Final post: link to the site.

Voice: dry, factual, engineer-to-engineer. No hype words. No rhetorical
questions. No "here's why this matters".

Return ONLY a JSON object. No markdown fences.

Schema: {"posts": [{"text": "max 280 chars including URL", "url": "string or empty"}]}`

	linkedInSystem = `Draft a LinkedIn post about today's single most important AI story,
written for a technical audience.

Structure:
- Line 1: the fact, plainly stated. No question hook.
- 2-3 short paragraphs: what changed, why it matters to builders.
- End with an open question that invites a real technical opinion.
- Leave a clearly marked [YOUR TAKE] placeholder where the author adds
  personal commentary.

Max 200 words. No hashtag spam — 3 maximum, at the end. No emoji.
Do not write in the first person; the author will add that.

Return ONLY a JSON object. No markdown fences.

Schema: {"body": "max 200 words, contains the literal token [YOUR TAKE]", "hashtags": ["max 3"], "story_url": "string"}`
)

func (g *Generator) intro(ctx context.Context, stories []model.Story, day time.Time) (Intro, error) {
	user := fmt.Sprintf("DATE: %s\n\nTODAY'S TOP STORIES:\n%s",
		day.UTC().Format(time.DateOnly), storyLines(stories))

	var out Intro
	if err := g.ask(ctx, introSystem, user, &out); err != nil {
		return Intro{}, err
	}
	if strings.TrimSpace(out.Intro) == "" {
		return Intro{}, fmt.Errorf("empty intro")
	}
	return out, nil
}

func (g *Generator) thread(ctx context.Context, stories []model.Story) (Thread, error) {
	var b strings.Builder
	b.WriteString("TODAY'S STORIES:\n")
	for i, s := range stories {
		fmt.Fprintf(&b, "%d. [%d] %s\n   %s\n   URL: %s\n",
			i+1, s.Score, s.Title, s.Summary, primaryURL(s))
	}
	fmt.Fprintf(&b, "\nSITE: %s\n\nProduce the JSON object.", g.cfg.SiteURL)

	var out Thread
	if err := g.ask(ctx, threadSystem, b.String(), &out); err != nil {
		return Thread{}, err
	}
	if len(out.Posts) == 0 {
		return Thread{}, fmt.Errorf("no posts returned")
	}
	return out, nil
}

func (g *Generator) linkedIn(ctx context.Context, top model.Story) (LinkedIn, error) {
	user := fmt.Sprintf("STORY: %s\n\nSUMMARY: %s\n\nURL: %s\n\nProduce the JSON object.",
		top.Title, top.Summary, primaryURL(top))

	var out LinkedIn
	if err := g.ask(ctx, linkedInSystem, user, &out); err != nil {
		return LinkedIn{}, err
	}

	// The placeholder is the whole point of the draft: it is where the human
	// voice goes. A draft without it would read as finished and get posted.
	if !strings.Contains(out.Body, "[YOUR TAKE]") {
		out.Body = strings.TrimSpace(out.Body) + "\n\n[YOUR TAKE]"
	}
	if len(out.Hashtags) > 3 {
		out.Hashtags = out.Hashtags[:3]
	}
	if out.StoryURL == "" {
		out.StoryURL = primaryURL(top)
	}
	return out, nil
}

// ask sends one prompt and decodes the JSON reply into out, retrying once.
//
// The retry matters more here than for summaries: the thread prompt asks for
// seven posts in one object, which is long enough that a reasoning model
// occasionally runs out of budget mid-answer and returns prose with no JSON.
func (g *Generator) ask(ctx context.Context, sys, user string, out any) error {
	var lastErr error

	for attempt := 1; attempt <= 2; attempt++ {
		raw, err := g.llm.Complete(ctx, sys, user)
		if err != nil {
			// The chain is down; a retry cannot fix that.
			return err
		}
		if err := decodeJSON(raw, out); err == nil {
			return nil
		} else {
			lastErr = err
		}
		g.log.Warn("digest response rejected", "attempt", attempt, "error", lastErr)
	}
	return lastErr
}

// decodeJSON parses a model reply, tolerating fences and surrounding prose.
func decodeJSON(raw string, out any) error {
	s := strings.TrimSpace(raw)
	if err := json.Unmarshal([]byte(s), out); err == nil {
		return nil
	}
	// Same defensive extraction as the summarizer: models wrap JSON in fences
	// and prose despite being told not to.
	i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if i < 0 || j <= i {
		// Carry a snippet of what actually came back. "No JSON object" alone
		// is unactionable — it cannot distinguish a refusal from a truncation
		// from a model that answered in prose.
		return fmt.Errorf("no JSON object in a %d-char response: %s", len(s), preview(s))
	}
	if err := json.Unmarshal([]byte(s[i:j+1]), out); err != nil {
		return fmt.Errorf("decode: %w: %s", err, preview(s[i:]))
	}
	return nil
}

func preview(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
