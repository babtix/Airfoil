package ingest

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/papitsho/airfoil/internal/config"
	"github.com/papitsho/airfoil/internal/model"
	"github.com/papitsho/airfoil/internal/normalize"
)

// githubAdapter finds recently created, fast-growing AI repositories.
//
// A token is optional but raises the search rate limit from 10 to 30 requests
// per minute, which matters when several keyword queries run in one pass.
type githubAdapter struct {
	fetch *fetcher
	token string
}

func newGitHubAdapter(f *fetcher, token string) *githubAdapter {
	return &githubAdapter{fetch: f, token: token}
}

func (a *githubAdapter) Type() string { return config.SourceGitHub }

type githubSearchResponse struct {
	Items []githubRepo `json:"items"`
}

type githubRepo struct {
	FullName    string `json:"full_name"`
	HTMLURL     string `json:"html_url"`
	Description string `json:"description"`
	Stars       int    `json:"stargazers_count"`
	CreatedAt   string `json:"created_at"`
	PushedAt    string `json:"pushed_at"`
	Owner       struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (a *githubAdapter) Fetch(ctx context.Context, src config.Source) ([]normalize.Raw, error) {
	opts := src.Options
	if len(opts.Queries) == 0 {
		return nil, fmt.Errorf("github: no queries configured for %s", src.ID)
	}

	days := opts.CreatedWithinDays
	if days <= 0 {
		days = 7
	}
	perQuery := opts.MaxPerQuery
	if perQuery <= 0 {
		perQuery = 5
	}
	createdAfter := time.Now().AddDate(0, 0, -days).Format(time.DateOnly)

	headers := map[string]string{
		"Accept":               "application/vnd.github+json",
		"X-GitHub-Api-Version": "2022-11-28",
	}
	if a.token != "" {
		headers["Authorization"] = "Bearer " + a.token
	}

	// One repository can match several keywords.
	seen := make(map[string]bool)
	var out []normalize.Raw
	var lastErr error
	succeeded := 0

	for _, topic := range opts.Queries {
		q := fmt.Sprintf("%s created:>%s", topic, createdAfter)
		if opts.MinStars > 0 {
			q += " stars:>" + strconv.Itoa(opts.MinStars)
		}

		params := url.Values{}
		params.Set("q", q)
		params.Set("sort", "stars")
		params.Set("order", "desc")
		params.Set("per_page", strconv.Itoa(perQuery))

		// The unauthenticated search limit is low and the whole source shares a
		// 15s budget, so a later query timing out is expected. Keep whatever
		// the earlier queries returned rather than discarding the source.
		var resp githubSearchResponse
		if err := a.fetch.getJSON(ctx, src.URL+"?"+params.Encode(), headers, &resp); err != nil {
			lastErr = fmt.Errorf("github %q: %w", topic, err)
			continue
		}
		succeeded++

		for _, repo := range resp.Items {
			if repo.HTMLURL == "" || seen[repo.FullName] {
				continue
			}
			seen[repo.FullName] = true

			out = append(out, normalize.Raw{
				SourceID:   src.ID,
				SourceName: src.Name,
				SourceTier: src.Tier,
				URL:        repo.HTMLURL,
				// The bare repo name is not a headline, so pair it with the
				// description the way a reader would see it on the repo page.
				Title:       githubTitle(repo),
				Body:        repo.Description,
				Author:      repo.Owner.Login,
				PublishedAt: githubTime(repo.CreatedAt),
				RepoURL:     repo.HTMLURL,
				Metrics: model.Metrics{
					GitHubStars: repo.Stars,
				},
			})
		}
	}

	if succeeded == 0 {
		return nil, lastErr
	}
	return out, nil
}

func githubTitle(repo githubRepo) string {
	if repo.Description == "" {
		return repo.FullName
	}
	return repo.FullName + ": " + repo.Description
}

func githubTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
