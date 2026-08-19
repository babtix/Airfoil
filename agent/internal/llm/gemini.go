package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/papitsho/airfoil/internal/config"
)

// gemini speaks Google's generateContent API, which differs from the OpenAI
// shape enough to need its own implementation: the system prompt is a separate
// top-level field and the answer arrives as parts of a candidate.
type gemini struct {
	apiKey string
	model  string
	client *http.Client
}

func newGemini(creds config.ProviderCreds) *gemini {
	// No client timeout: the per-attempt deadline comes from the context.
	return &gemini{apiKey: creds.APIKey, model: creds.Model, client: &http.Client{}}
}

func (g *gemini) Name() string { return "gemini:" + g.model }

type geminiRequest struct {
	SystemInstruction *geminiContent   `json:"system_instruction,omitempty"`
	Contents          []geminiContent  `json:"contents"`
	GenerationConfig  geminiGenIConfig `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenIConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
	ResponseMIME    string  `json:"responseMimeType,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (g *gemini) Complete(ctx context.Context, sys, user string) (string, error) {
	body, err := json.Marshal(geminiRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: sys}}},
		Contents:          []geminiContent{{Parts: []geminiPart{{Text: user}}}},
		GenerationConfig: geminiGenIConfig{
			Temperature:     temperature,
			MaxOutputTokens: maxTokens,
			// Asking for JSON directly removes most markdown-fence cleanup.
			ResponseMIME: "application/json",
		},
	})
	if err != nil {
		return "", fmt.Errorf("gemini: marshal: %w", err)
	}

	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", g.model)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("gemini: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Header auth, not a query parameter: a key in the URL leaks into logs.
	req.Header.Set("x-goog-api-key", g.apiKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", fmt.Errorf("gemini: read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini: %s: %s", resp.Status, snippet(raw))
	}

	var out geminiResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("gemini: decode: %w: %s", err, snippet(raw))
	}
	if out.Error != nil {
		return "", fmt.Errorf("gemini: %s", out.Error.Message)
	}
	if len(out.Candidates) == 0 {
		return "", fmt.Errorf("gemini: no candidates (safety filter or empty prompt)")
	}

	var b strings.Builder
	for _, p := range out.Candidates[0].Content.Parts {
		b.WriteString(p.Text)
	}
	return strings.TrimSpace(b.String()), nil
}
