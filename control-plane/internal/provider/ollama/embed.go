package ollama

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/chiotron/ai-control-plane/internal/provider"
)

// Embedder adapts Ollama's /api/embed to the EmbeddingProvider contract.
//
// It is a separate type from Client because the embedding model is a separate
// configuration decision from the completion model, and the two are frequently
// different models on the same runtime.
type Embedder struct {
	client     *Client
	model      string
	dimensions int
}

func NewEmbedder(baseURL, model string, dimensions int, timeout time.Duration) *Embedder {
	return &Embedder{client: New(baseURL, timeout), model: model, dimensions: dimensions}
}

func (e *Embedder) Name() string    { return "ollama" }
func (e *Embedder) Model() string   { return e.model }
func (e *Embedder) Dimensions() int { return e.dimensions }

// Embed vectorises a batch in one call. Ollama accepts an array input, which
// keeps one HTTP round trip per chunk batch rather than per chunk.
func (e *Embedder) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	body := map[string]any{"model": e.model, "input": inputs}
	var payload struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := e.client.do(ctx, http.MethodPost, "/api/embed", body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %s", provider.ErrUnavailable, err)
	}

	if len(payload.Embeddings) != len(inputs) {
		return nil, fmt.Errorf("%w: asked for %d embeddings, got %d",
			provider.ErrUnavailable, len(inputs), len(payload.Embeddings))
	}
	// The schema pins the vector width, so a model swap must fail here rather
	// than write rows that no index can use.
	for i, embedding := range payload.Embeddings {
		if len(embedding) != e.dimensions {
			return nil, fmt.Errorf("%w: embedding %d has %d dimensions, schema expects %d (model %q)",
				provider.ErrUnavailable, i, len(embedding), e.dimensions, e.model)
		}
	}
	return payload.Embeddings, nil
}
