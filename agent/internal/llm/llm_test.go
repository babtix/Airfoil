package llm

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/papitsho/airfoil/internal/config"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// stubProvider is a Provider whose behaviour the test controls.
type stubProvider struct {
	name  string
	reply string
	err   error
	calls int32
	// block holds the call until the context expires, to exercise timeouts.
	block bool
}

func (s *stubProvider) Name() string { return s.name }

func (s *stubProvider) Complete(ctx context.Context, _, _ string) (string, error) {
	atomic.AddInt32(&s.calls, 1)
	if s.block {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return s.reply, s.err
}

func chainOf(ps ...Provider) *Chain {
	return &Chain{providers: ps, log: quietLogger()}
}

func TestChainReturnsFirstSuccess(t *testing.T) {
	first := &stubProvider{name: "first", reply: "answer"}
	second := &stubProvider{name: "second", reply: "unused"}

	got, err := chainOf(first, second).Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got != "answer" {
		t.Errorf("Complete() = %q, want %q", got, "answer")
	}
	if second.calls != 0 {
		t.Errorf("second provider was called %d times; the chain must stop at the first success", second.calls)
	}
}

func TestChainFallsThroughOnError(t *testing.T) {
	// R8: a dead provider must not kill the run.
	dead := &stubProvider{name: "dead", err: errors.New("503 service unavailable")}
	alive := &stubProvider{name: "alive", reply: "answer"}

	got, err := chainOf(dead, alive).Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got != "answer" {
		t.Errorf("Complete() = %q, want the second provider's answer", got)
	}
	if dead.calls != 1 || alive.calls != 1 {
		t.Errorf("calls: dead=%d alive=%d, want 1 each", dead.calls, alive.calls)
	}
}

func TestChainTreatsEmptyContentAsFailure(t *testing.T) {
	// A reasoning model can spend its whole budget thinking and return empty
	// content with no error. Surfacing "" would hand the caller a non-answer
	// while a healthy fallback sat unused.
	empty := &stubProvider{name: "empty", reply: "   "}
	alive := &stubProvider{name: "alive", reply: "answer"}

	got, err := chainOf(empty, alive).Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got != "answer" {
		t.Errorf("Complete() = %q, want the fallback to have been used", got)
	}
}

func TestChainExhaustedNamesEveryFailure(t *testing.T) {
	a := &stubProvider{name: "alpha", err: errors.New("401 unauthorized")}
	b := &stubProvider{name: "bravo", err: errors.New("timeout")}

	_, err := chainOf(a, b).Complete(context.Background(), "sys", "user")
	if !errors.Is(err, ErrNoProvider) {
		t.Fatalf("error = %v, want ErrNoProvider", err)
	}
	// The whole point of the aggregated message is that one log line explains
	// why every provider failed.
	for _, want := range []string{"alpha", "401 unauthorized", "bravo", "timeout"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

func TestChainWithNoProviders(t *testing.T) {
	_, err := chainOf().Complete(context.Background(), "sys", "user")
	if !errors.Is(err, ErrNoProvider) {
		t.Fatalf("error = %v, want ErrNoProvider", err)
	}
}

func TestChainStopsWhenParentContextCancelled(t *testing.T) {
	// A cancelled run must not keep trying providers: each would just produce
	// another cancellation and delay shutdown.
	first := &stubProvider{name: "first", err: errors.New("boom")}
	second := &stubProvider{name: "second", reply: "answer"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := chainOf(first, second).Complete(ctx, "sys", "user"); err == nil {
		t.Fatal("Complete() = nil error, want failure on a cancelled context")
	}
	if second.calls != 0 {
		t.Errorf("second provider called %d times after cancellation", second.calls)
	}
}

func TestChainAppliesPerAttemptTimeout(t *testing.T) {
	// The deadline is per attempt, so a provider that hangs must not consume
	// the whole run — the next one still gets its own full budget.
	slow := &stubProvider{name: "slow", block: true}
	fast := &stubProvider{name: "fast", reply: "answer"}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// The parent deadline fires first here; what matters is that the hung
	// provider is abandoned rather than blocking forever.
	done := make(chan struct{})
	go func() {
		_, _ = chainOf(slow, fast).Complete(ctx, "sys", "user")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Complete() did not return; the per-attempt timeout is not being applied")
	}
}

// --- provider construction ---------------------------------------------------

func TestNewSkipsUnconfiguredProviders(t *testing.T) {
	cfg := &config.Config{LLM: config.LLMConfig{
		NvidiaNIM:  config.ProviderCreds{APIKey: "k", Model: "m"}, // ready
		OpenRouter: config.ProviderCreds{APIKey: "k"},             // no model
	}}

	got := New(cfg, quietLogger()).Providers()
	if len(got) != 1 {
		t.Fatalf("providers = %d, want 1", len(got))
	}
	if !strings.HasPrefix(got[0].Name(), "nvidia_nim") {
		t.Errorf("provider = %q, want the NIM one", got[0].Name())
	}
}

func TestNewOrdersChainByFallbackPriority(t *testing.T) {
	cfg := &config.Config{LLM: config.LLMConfig{
		NvidiaNIM:  config.ProviderCreds{APIKey: "k", Model: "n"},
		OpenRouter: config.ProviderCreds{APIKey: "k", Model: "o"},
	}}

	got := New(cfg, quietLogger()).Providers()
	want := []string{"openrouter", "nvidia_nim"}

	if len(got) != len(want) {
		t.Fatalf("providers = %d, want %d", len(got), len(want))
	}
	for i, prefix := range want {
		if !strings.HasPrefix(got[i].Name(), prefix) {
			t.Errorf("provider %d = %q, want %s first", i, got[i].Name(), prefix)
		}
	}
}

func TestNewIncludesNoLocalProvider(t *testing.T) {
	// Local models were removed deliberately: the pipeline runs in CI, where
	// no daemon is listening, so a local fallback would look healthy on a
	// laptop and fail in the only place that matters.
	cfg := &config.Config{LLM: config.LLMConfig{
		NvidiaNIM: config.ProviderCreds{APIKey: "k", Model: "m"},
	}}

	for _, p := range New(cfg, quietLogger()).Providers() {
		if strings.Contains(strings.ToLower(p.Name()), "ollama") ||
			strings.Contains(p.Name(), "localhost") {
			t.Errorf("chain contains a local provider: %q", p.Name())
		}
	}
}

// --- OpenAI-compatible transport ---------------------------------------------

func TestOpenAICompatibleParsesContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"the answer"}}]}`))
	}))
	defer srv.Close()

	p := newOpenAICompatible("test", srv.URL, "test-key", "model-x", nil)
	got, err := p.Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got != "the answer" {
		t.Errorf("Complete() = %q", got)
	}
}

func TestOpenAICompatibleIgnoresReasoningContent(t *testing.T) {
	// Reasoning is the model's scratch work. Returning it would put the
	// model's thinking into a published summary.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{
			"content":"the answer","reasoning_content":"first I should consider..."}}]}`))
	}))
	defer srv.Close()

	got, err := newOpenAICompatible("test", srv.URL, "k", "m", nil).
		Complete(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if got != "the answer" {
		t.Errorf("Complete() = %q, want only the content field", got)
	}
}

func TestOpenAICompatibleReportsTruncation(t *testing.T) {
	// finish_reason "length" with reasoning prose in content is the failure
	// that cost three wrong diagnoses: downstream it looks like "not JSON",
	// but the fix is a bigger budget. It must name itself.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{
			"content":"We need to produce JSON with a posts array. Let me think..."}}]}`))
	}))
	defer srv.Close()

	_, err := newOpenAICompatible("test", srv.URL, "k", "m", nil).
		Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("Complete() = nil error, want a truncation report")
	}
	if !strings.Contains(err.Error(), "budget") {
		t.Errorf("error = %v, want it to name the token budget", err)
	}
}

func TestOpenAICompatibleSurfacesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"User not found.","code":401}}`))
	}))
	defer srv.Close()

	_, err := newOpenAICompatible("test", srv.URL, "bad", "m", nil).
		Complete(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("Complete() = nil error, want the 401 surfaced")
	}
	if !strings.Contains(err.Error(), "User not found") {
		t.Errorf("error = %v, want it to carry the provider's message", err)
	}
}

func TestOpenAICompatibleSendsAttributionHeaders(t *testing.T) {
	var referer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		referer = r.Header.Get("HTTP-Referer")
		w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := newOpenAICompatible("test", srv.URL, "k", "m",
		map[string]string{"HTTP-Referer": "https://example.test"})
	if _, err := p.Complete(context.Background(), "sys", "user"); err != nil {
		t.Fatal(err)
	}
	if referer != "https://example.test" {
		t.Errorf("HTTP-Referer = %q, want it forwarded", referer)
	}
}

func TestOpenAICompatibleHandlesNoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	if _, err := newOpenAICompatible("test", srv.URL, "k", "m", nil).
		Complete(context.Background(), "sys", "user"); err == nil {
		t.Error("Complete() = nil error, want a failure on an empty choices array")
	}
}
