package rag

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// Preparer performs the ingestion work that must happen before retrieval.
// BatchSize limits how many chunks are sent to an embedding provider at once.
type Preparer struct {
	Chunker   *Chunker
	Embedder  Embedder
	BatchSize int
}

// Prepare chunks extracted documents and embeds the resulting passages. It
// intentionally does not save or retrieve anything.
func (p Preparer) Prepare(ctx context.Context, documents []Document) ([]EmbeddedChunk, error) {
	if p.Chunker == nil {
		return nil, errors.New("chunker is required")
	}
	if p.Embedder == nil {
		return nil, errors.New("embedder is required")
	}
	if p.BatchSize <= 0 {
		return nil, errors.New("embedding batch size must be positive")
	}

	model := strings.TrimSpace(p.Embedder.Model())
	if model == "" {
		return nil, errors.New("embedding model is required")
	}

	chunks, err := p.Chunker.Split(ctx, documents)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return []EmbeddedChunk{}, nil
	}

	prepared := make([]EmbeddedChunk, 0, len(chunks))
	embeddingDimension := 0
	for start := 0; start < len(chunks); start += p.BatchSize {
		end := min(start+p.BatchSize, len(chunks))
		texts := make([]string, end-start)
		for index := start; index < end; index++ {
			texts[index-start] = chunks[index].Content
		}

		vectors, err := p.Embedder.Embed(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("embed chunks %d-%d: %w", start+1, end, err)
		}
		if len(vectors) != len(texts) {
			return nil, fmt.Errorf("embedding count %d does not match batch size %d", len(vectors), len(texts))
		}

		for offset, vector := range vectors {
			if len(vector) == 0 {
				return nil, fmt.Errorf("embedding %d is empty", start+offset+1)
			}
			if embeddingDimension == 0 {
				embeddingDimension = len(vector)
			}
			if len(vector) != embeddingDimension {
				return nil, fmt.Errorf(
					"embedding %d has dimension %d; expected %d",
					start+offset+1,
					len(vector),
					embeddingDimension,
				)
			}

			prepared = append(prepared, EmbeddedChunk{
				Chunk:          chunks[start+offset],
				EmbeddingModel: model,
				Vector:         append([]float32(nil), vector...),
			})
		}
	}

	return prepared, nil
}
