package normalize

import (
	"regexp"
	"strings"
)

var (
	// github.com/owner/repo, in a URL or in prose.
	githubRepoRE = regexp.MustCompile(`(?i)github\.com/([A-Za-z0-9][A-Za-z0-9_.-]*)/([A-Za-z0-9][A-Za-z0-9_.-]*)`)

	// Modern arXiv identifiers: 2401.12345, optionally versioned.
	arxivModernRE = regexp.MustCompile(`(?i)(?:arxiv\.org/(?:abs|pdf|html)/|arxiv[:\s]+|huggingface\.co/papers/)(\d{4}\.\d{4,5})(v\d+)?`)

	// Pre-2007 identifiers: cs.AI/0301001, math/0211159.
	arxivLegacyRE = regexp.MustCompile(`(?i)arxiv\.org/(?:abs|pdf)/([a-z-]+(?:\.[a-z]{2})?/\d{7})`)
)

// githubReservedPaths are first path segments on github.com that are site
// features rather than user accounts. Without this, "github.com/features/copilot"
// would be recorded as a repository.
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

// githubReservedRepos are second path segments that belong to an account page
// rather than to a repository.
var githubReservedRepos = map[string]bool{
	"followers": true, "following": true, "repositories": true, "projects": true,
	"packages": true, "stars": true, "sponsors": true, "settings": true,
}

// ExtractRepoURL returns the first GitHub repository URL found in the given
// texts, canonicalized to https://github.com/owner/repo.
//
// A repository link is a strong builder signal, and two items pointing at the
// same repo are forced into one cluster regardless of embedding similarity.
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

// ExtractPaperURL returns the first arXiv paper found in the given texts,
// canonicalized to https://arxiv.org/abs/ID.
//
// The version suffix is dropped deliberately: v1 and v2 of a paper are the same
// event, so they must collapse into one cluster.
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
}
