package normalize

import "testing"

func TestExtractRepoURL(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"plain repo url", []string{"https://github.com/ollama/ollama"}, "https://github.com/ollama/ollama"},
		{"repo url with path", []string{"https://github.com/vllm-project/vllm/releases/tag/v0.6"}, "https://github.com/vllm-project/vllm"},
		{"in prose", []string{"weights are at github.com/meta-llama/llama-models today"}, "https://github.com/meta-llama/llama-models"},
		{"scans later texts", []string{"nothing here", "see https://github.com/a/b"}, "https://github.com/a/b"},
		{"git suffix is stripped", []string{"https://github.com/a/b.git"}, "https://github.com/a/b"},
		{"trailing sentence period", []string{"see github.com/a/b."}, "https://github.com/a/b"},
		{"uppercase host", []string{"https://GitHub.com/Owner/Repo"}, "https://github.com/Owner/Repo"},
		{"dots and dashes in names", []string{"https://github.com/my-org/my.repo-name"}, "https://github.com/my-org/my.repo-name"},

		{"feature page is not a repo", []string{"https://github.com/features/copilot"}, ""},
		{"topic page is not a repo", []string{"https://github.com/topics/llm"}, ""},
		{"pricing page is not a repo", []string{"https://github.com/pricing"}, ""},
		{"user tab is not a repo", []string{"https://github.com/torvalds/followers"}, ""},
		{"reserved path is skipped but a real repo still wins", []string{"https://github.com/features/x and https://github.com/a/b"}, "https://github.com/a/b"},

		{"bare profile has no repo", []string{"https://github.com/torvalds"}, ""},
		{"no github at all", []string{"https://openai.com/news"}, ""},
		{"empty input", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractRepoURL(tt.in...); got != tt.want {
				t.Errorf("ExtractRepoURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtractPaperURL(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want string
	}{
		{"abs url", []string{"https://arxiv.org/abs/2401.12345"}, "https://arxiv.org/abs/2401.12345"},
		{"pdf url", []string{"https://arxiv.org/pdf/2401.12345"}, "https://arxiv.org/abs/2401.12345"},
		{"html url", []string{"https://arxiv.org/html/2401.12345v2"}, "https://arxiv.org/abs/2401.12345"},
		{"five digit id", []string{"https://arxiv.org/abs/2401.123456"}, "https://arxiv.org/abs/2401.12345"},
		{"identifier in prose", []string{"see arXiv:2401.12345 for details"}, "https://arxiv.org/abs/2401.12345"},
		{"huggingface papers", []string{"https://huggingface.co/papers/2401.12345"}, "https://arxiv.org/abs/2401.12345"},
		{"legacy identifier", []string{"https://arxiv.org/abs/cs.AI/0301001"}, "https://arxiv.org/abs/cs.ai/0301001"},
		{"scans later texts", []string{"nothing", "arXiv:2401.12345"}, "https://arxiv.org/abs/2401.12345"},

		{"no paper", []string{"https://openai.com/news"}, ""},
		{"empty input", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractPaperURL(tt.in...); got != tt.want {
				t.Errorf("ExtractPaperURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// Versioned and unversioned links to one paper must produce one identity, so
// that v1 and v2 coverage lands in the same cluster.
func TestExtractPaperURLIgnoresVersion(t *testing.T) {
	variants := []string{
		"https://arxiv.org/abs/2401.12345",
		"https://arxiv.org/abs/2401.12345v1",
		"https://arxiv.org/pdf/2401.12345v3",
		"arXiv:2401.12345v2",
	}

	want := "https://arxiv.org/abs/2401.12345"
	for _, v := range variants {
		if got := ExtractPaperURL(v); got != want {
			t.Errorf("ExtractPaperURL(%q) = %q, want %q", v, got, want)
		}
	}
}
