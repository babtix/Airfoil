package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// openAICompatible speaks the /chat/completions shape used by both NVIDIA NIM
// and OpenRouter. Only the base URL, key, model, and a few attribution headers
// differ between them.
type openAICompatible struct {
	name    string
	baseURL string
	apiKey  string
	model   string
	headers map[string]string
	client  *http.Client
}

func newOpenAICompatible(name, baseURL, apiKey, model string, headers map[string]string) *openAICompatible {
	return &openAICompatible{
		name:    name,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		headers: headers,
		// No client timeout: the per-attempt deadline comes from the context,
		// so the chain controls it in one place.
		client: &http.Client{},
	}
}

func (o *openAICompatible) Name() string { return o.name + ":" + o.model }

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Content string `json:"content"`
			// ReasoningContent is where reasoning models put their thinking.
			// It is deliberately never returned to the caller — only the
			// content field carries the answer.
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (o *openAICompatible) Complete(ctx context.Context, sys, user string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: o.model,
		Messages: []chatMessage{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
		MaxTokens:   maxTokens,
		Temperature: temperature,
	})
	if err != nil {
		return "", fmt.Errorf("%s: marshal: %w", o.name, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		o.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("%s: %w", o.name, err)
	}
	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range o.headers {
		req.Header.Set(k, v)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: %w", o.name, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("%s: read: %w", o.name, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s: %s", o.name, resp.Status, snippet(raw))
	}

	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("%s: decode: %w: %s", o.name, err, snippet(raw))
	}
	if out.Error != nil {
		return "", fmt.Errorf("%s: %s", o.name, out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("%s: no choices returned", o.name)
	}

	choice := out.Choices[0]
	content := strings.TrimSpace(choice.Message.Content)

	// A reasoning model whose budget ran out mid-thought hits finish_reason
	// "length". The content field is then either empty or — worse — full of
	// reasoning prose with no answer in it, which every downstream parser
	// reports as "not JSON" and no one can act on. Name it here instead.
	if choice.FinishReason == "length" {
		return "", fmt.Errorf(
			"%s: hit the %d-token budget before finishing (reasoning consumed it; %d chars of partial output)",
			o.name, maxTokens, len(content))
	}
	return content, nil
}

// snippet trims a provider error body to something loggable.
func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return strings.ReplaceAll(s, "\n", " ")
}

// backoff is the jittered pause before a retry. Exposed for the summarizer.
func backoff(attempt int) time.Duration {
	return time.Duration(attempt) * 750 * time.Millisecond
}
