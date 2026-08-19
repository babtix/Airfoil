package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// postJSON sends body as JSON and decodes the response into out.
func postJSON(ctx context.Context, client *http.Client, url string, headers map[string]string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post %s: %w", url, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("read %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("post %s: %s: %s", url, resp.Status, snippet(raw))
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode %s: %w", url, err)
	}
	return nil
}

// snippet trims an error body to something loggable.
func snippet(b []byte) string {
	const max = 200
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}

// dims records the vector width once the first response arrives.
type dims struct {
	mu sync.Mutex
	n  int
}

func (d *dims) set(n int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.n == 0 {
		d.n = n
	}
}

func (d *dims) get() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.n
}

// --- Gemini -----------------------------------------------------------------

// geminiEmbedder uses the batchEmbedContents endpoint.
type geminiEmbedder struct {
	client *http.Client
	apiKey string
	model  string
	dims   dims
}

func newGemini(client *http.Client, apiKey, model string) *geminiEmbedder {
	return &geminiEmbedder{client: client, apiKey: apiKey, model: strings.TrimPrefix(model, "models/")}
}

func (g *geminiEmbedder) Name() string    { return "gemini:" + g.model }
func (g *geminiEmbedder) Dimensions() int { return g.dims.get() }

func (g *geminiEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// The documented ceiling for batchEmbedContents is 100 requests.
	return embedBatches(ctx, texts, 100, g.embedBatch)
}

func (g *geminiEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	type content struct {
		Parts []map[string]string `json:"parts"`
	}
	type request struct {
		Model   string  `json:"model"`
		Content content `json:"content"`
	}

	reqs := make([]request, len(texts))
	for i, text := range texts {
		reqs[i] = request{
			Model:   "models/" + g.model,
			Content: content{Parts: []map[string]string{{"text": text}}},
		}
	}

	var resp struct {
		Embeddings []struct {
			Values []float32 `json:"values"`
		} `json:"embeddings"`
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:batchEmbedContents", g.model)
	err := postJSON(ctx, g.client, url,
		map[string]string{"x-goog-api-key": g.apiKey},
		map[string]any{"requests": reqs}, &resp)
	if err != nil {
		return nil, fmt.Errorf("gemini: %w", err)
	}

	out := make([][]float32, len(resp.Embeddings))
	for i, e := range resp.Embeddings {
		out[i] = e.Values
	}
	if len(out) > 0 {
		g.dims.set(len(out[0]))
	}
	return out, nil
}

// --- NVIDIA NIM -------------------------------------------------------------

// nimEmbedder uses NVIDIA's OpenAI-compatible embeddings endpoint.
type nimEmbedder struct {
	client  *http.Client
	baseURL string
	apiKey  string
	model   string
	dims    dims
}

func newNIM(client *http.Client, baseURL, apiKey, model string) *nimEmbedder {
	return &nimEmbedder{
		client:  client,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
	}
}

func (n *nimEmbedder) Name() string    { return "nvidia_nim:" + n.model }
func (n *nimEmbedder) Dimensions() int { return n.dims.get() }

func (n *nimEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// NIM's retrieval endpoints reject large batches.
	return embedBatches(ctx, texts, 32, n.embedBatch)
}

func (n *nimEmbedder) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var resp struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	// The NIM retrieval models are asymmetric and require input_type. Every
	// item here is a document being compared against other documents, so they
	// all go in as passages — using "query" for some would put them in a
	// different part of the space and break the similarity comparison.
	err := postJSON(ctx, n.client, n.baseURL+"/embeddings",
		map[string]string{"Authorization": "Bearer " + n.apiKey},
		map[string]any{
			"model":           n.model,
			"input":           texts,
			"input_type":      "passage",
			"encoding_format": "float",
			"truncate":        "END",
		}, &resp)
	if err != nil {
		return nil, fmt.Errorf("nvidia_nim: %w", err)
	}

	// The API documents an index field; honour it rather than assuming order.
	out := make([][]float32, len(resp.Data))
	for _, d := range resp.Data {
		if d.Index < 0 || d.Index >= len(out) {
			return nil, fmt.Errorf("nvidia_nim: response index %d out of range", d.Index)
		}
		out[d.Index] = d.Embedding
	}
	for i, v := range out {
		if v == nil {
			return nil, fmt.Errorf("nvidia_nim: no vector for input %d", i)
		}
	}
	if len(out) > 0 {
		n.dims.set(len(out[0]))
	}
	return out, nil
}
