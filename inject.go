package main

import (
	"context"
	"fmt"
	"log"
)

// InjectConfig holds parameters for a single injection run.
type InjectConfig struct {
	// QdrantCfg describes the target collection and transport.
	QdrantCfg Config

	// Docs are the records to inject.  Build them with ElogEntryToRAGDoc.
	Docs []RAGDoc

	// BatchSize controls how many points are sent per Upsert call.
	// 0 defaults to 200.
	BatchSize int
}

// Inject opens a Qdrant client, ensures the collection exists, and upserts
// all docs in batches.  It returns the total number of points written.
//
// Errors from individual batches are logged and counted; the function
// continues with the remaining batches so a single bad record doesn't abort
// the whole run.  A non-nil error is returned only when the client cannot be
// opened or the collection cannot be initialised.
func Inject(ctx context.Context, cfg InjectConfig) (int, error) {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 200
	}

	client, err := New(cfg.QdrantCfg)
	if err != nil {
		return 0, fmt.Errorf("qdrant inject: open client: %w", err)
	}
	defer client.Close()

	if err := client.EnsureCollection(ctx, cfg.QdrantCfg.Dimension); err != nil {
		return 0, fmt.Errorf("qdrant inject: ensure collection: %w", err)
	}

	points := make([]Point, len(cfg.Docs))
	for i, d := range cfg.Docs {
		points[i] = d.ToPoint()
	}

	total, batchErrs := 0, 0
	for start := 0; start < len(points); start += cfg.BatchSize {
		end := start + cfg.BatchSize
		if end > len(points) {
			end = len(points)
		}

		batch := points[start:end]
		if err := client.UpsertPoints(ctx, batch); err != nil {
			log.Printf("qdrant inject: batch [%d:%d] failed: %v", start, end, err)
			batchErrs++
			continue
		}
		total += len(batch)
	}

	if batchErrs > 0 {
		return total, fmt.Errorf("qdrant inject: %d batch(es) failed (injected %d/%d points)",
			batchErrs, total, len(points))
	}
	return total, nil
}
