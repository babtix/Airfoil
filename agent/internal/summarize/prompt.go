// Package summarize turns a scored cluster into a validated story summary.
//
// Prompt construction and response validation are pure functions: they take a
// cluster and a string and return a result, with no network in between. That is
// what makes R1 and R2 testable without an API key (R10).
package summarize

import (
	"fmt"
	"strings"
	"time"

	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/score"
)

// SystemPrompt is docs/PROMPTS.md §1, verbatim. The rules are restated to the
// model, but never trusted — every one of them is also enforced in Go by
// Validate, because a prompt is a request and a validator is a guarantee.
const SystemPrompt = `You write for an AI news site read by software engineers.

Your job: turn a group of articles about the SAME event into one short,
factual summary.

RULES — these are absolute:
1. Write ORIGINAL prose. Never copy or lightly reword sentences from the
   input. If you find yourself reusing more than a few consecutive words
   from a source, rewrite from scratch.
2. Maximum 80 words in the summary.
3. At most ONE quoted phrase, under 15 words, and only when the exact
   wording carries meaning that paraphrase would lose. Prefer zero quotes.
4. Only state facts present in the input. Never infer, speculate, or add
   background knowledge.
5. If the input is contradictory or too thin to summarize, say so in the
   summary field and set confidence to "low".
6. No hype. No "game-changing", "revolutionary", "massive". Neutral register.
7. Lead with what changed and what it means for someone building software.

Return ONLY a JSON object. No markdown fences. No text before or after.

Schema:
{"title": "max 80 chars, factual, no clickbait, no trailing period",
 "summary": "max 80 words, original prose",
 "takeaways": ["max 12 words each, 2-3 items"],
 "tags": ["2-4 items, lowercase, from the allowed list"],
 "builder_relevant": true,
 "confidence": "high | medium | low"}

Allowed tags: models, agents, rag, infra, open-source, research, policy,
business, safety, coding, tools, hardware`

// UserPrompt renders the per-cluster message from docs/PROMPTS.md §1.
func UserPrompt(c model.Cluster, r score.Result) string {
	var b strings.Builder

	fmt.Fprintf(&b, "EVENT SOURCES (%d articles):\n\n", len(c.Items))
	for _, item := range c.Items {
		b.WriteString("---\n")
		fmt.Fprintf(&b, "SOURCE: %s (tier %d)\n", item.SourceName, item.SourceTier)
		fmt.Fprintf(&b, "TITLE: %s\n", item.Title)
		fmt.Fprintf(&b, "PUBLISHED: %s\n", item.PublishedAt.UTC().Format(time.RFC3339))
		fmt.Fprintf(&b, "EXCERPT: %s\n", item.Excerpt)
		if item.RepoURL != "" {
			fmt.Fprintf(&b, "REPO: %s\n", item.RepoURL)
		}
		if item.PaperURL != "" {
			fmt.Fprintf(&b, "PAPER: %s\n", item.PaperURL)
		}
	}

	b.WriteString("\nSIGNALS:\n")
	fmt.Fprintf(&b, "- distinct sources: %d\n", r.DistinctSources)
	fmt.Fprintf(&b, "- highest source tier: %d\n", r.BestTier)
	fmt.Fprintf(&b, "- HN points: %d\n", r.HNPoints)
	fmt.Fprintf(&b, "- Reddit score: %d\n", r.RedditScore)
	b.WriteString("\nProduce the JSON object.")

	return b.String()
}
