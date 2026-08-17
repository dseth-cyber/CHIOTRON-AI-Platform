package knowledge

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/chiotron/ai-control-plane/internal/provider"
	"github.com/chiotron/ai-control-plane/internal/storage"
)

// embedBatch bounds how many chunks are vectorised per provider call. Larger
// batches are fewer round trips but a longer stall if the compute plane is slow.
const embedBatch = 16

// Worker ingests pending documents: read from storage, parse, chunk, embed and
// store.
//
// It runs in-process rather than as a separate deployable. Ingestion is not on a
// user's request path, and splitting it out is a scaling decision that belongs
// with the Kafka work rather than something to guess at now.
type Worker struct {
	store    *Store
	storage  storage.Provider
	embedder provider.EmbeddingProvider
	plan     ChunkPlan
	log      *slog.Logger
	interval time.Duration
	batch    int
}

func NewWorker(store *Store, objects storage.Provider, embedder provider.EmbeddingProvider,
	plan ChunkPlan, interval time.Duration, batch int, log *slog.Logger) *Worker {
	if batch <= 0 {
		batch = 4
	}
	return &Worker{
		store: store, storage: objects, embedder: embedder, plan: plan,
		log: log, interval: interval, batch: batch,
	}
}

// Run polls until the context ends.
//
// Polling rather than listening: the queue lives in PostgreSQL, a poll costs one
// indexed query against a partial index, and it survives a missed notification
// where LISTEN/NOTIFY would not.
func (w *Worker) Run(ctx context.Context) {
	w.log.Info("ingestion worker started",
		"interval", w.interval.String(), "batch", w.batch,
		"embeddingModel", w.embedder.Model(), "dimensions", w.embedder.Dimensions())

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		// Drain whatever is waiting before sleeping again, so a burst of uploads
		// is not spread across one poll each.
		for {
			processed, err := w.pass(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				w.log.Error("ingestion pass failed", "error", err)
				break
			}
			if processed == 0 {
				break
			}
		}

		select {
		case <-ctx.Done():
			w.log.Info("ingestion worker stopped")
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) pass(ctx context.Context) (int, error) {
	documents, err := w.store.ClaimPending(ctx, w.batch)
	if err != nil {
		return 0, err
	}

	for _, document := range documents {
		if err := w.ingest(ctx, document); err != nil {
			if ctx.Err() != nil {
				return len(documents), ctx.Err()
			}
			// One bad document must not stop the queue. The reason is stored on
			// the row so an operator can see it without reading logs.
			w.log.Error("ingest document", "documentId", document.ID, "title", document.Title, "error", err)
			if markErr := w.store.MarkFailed(context.WithoutCancel(ctx), document.ID, err.Error()); markErr != nil {
				w.log.Error("mark document failed", "documentId", document.ID, "error", markErr)
			}
		}
	}
	return len(documents), nil
}

func (w *Worker) ingest(ctx context.Context, document Document) error {
	content, err := w.storage.Get(ctx, document.StorageKey())
	if err != nil {
		return fmt.Errorf("read stored bytes: %w", err)
	}

	text, err := Parse(document.MimeType, content)
	if err != nil {
		return err
	}

	chunks := w.plan.Split(text)
	if len(chunks) == 0 {
		return errors.New("document produced no chunks")
	}

	embeddings := make([][]float32, 0, len(chunks))
	for start := 0; start < len(chunks); start += embedBatch {
		end := min(start+embedBatch, len(chunks))
		batch, err := w.embedder.Embed(ctx, chunks[start:end])
		if err != nil {
			return fmt.Errorf("embed chunks %d-%d: %w", start, end-1, err)
		}
		embeddings = append(embeddings, batch...)
	}

	if err := w.store.SaveChunks(ctx, document, chunks, embeddings); err != nil {
		return err
	}

	w.log.Info("document ingested",
		"documentId", document.ID, "chunks", len(chunks),
		"classification", document.Classification, "company", document.CompanyID)
	return nil
}
