package rag

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

type recordingEmbedder struct {
	batches [][]string
}

func (*recordingEmbedder) Model() string {
	return "example/embedding-model"
}

func (embedder *recordingEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	embedder.batches = append(embedder.batches, append([]string(nil), texts...))
	vectors := make([][]float32, len(texts))
	for index, text := range texts {
		vectors[index] = []float32{float32(len(text)), float32(index + 1)}
	}
	return vectors, nil
}

func TestPrepareChunksAndEmbedsInBoundedBatches(t *testing.T) {
	chunker, err := NewChunker(80, 10)
	if err != nil {
		t.Fatal(err)
	}
	embedder := &recordingEmbedder{}
	preparer := Preparer{Chunker: chunker, Embedder: embedder, BatchSize: 2}

	prepared, err := preparer.Prepare(context.Background(), []Document{
		{
			ID:      "document-1",
			Source:  "case.md",
			Content: strings.Repeat("A source-addressable sentence. ", 12),
			Metadata: map[string]string{
				"matter":              "example",
				documentIDMetadataKey: "cannot-replace-identity",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared) < 3 {
		t.Fatalf("prepared chunk count = %d, want at least 3", len(prepared))
	}
	for _, batch := range embedder.batches {
		if len(batch) > 2 {
			t.Fatalf("embedding batch size = %d, want at most 2", len(batch))
		}
	}
	for index, item := range prepared {
		if item.DocumentID != "document-1" || item.Source != "case.md" {
			t.Fatalf("chunk %d lost source identity: %+v", index, item.Chunk)
		}
		if item.ID != "document-1:"+strconv.Itoa(index+1) {
			t.Fatalf("chunk %d ID = %q", index, item.ID)
		}
		if item.EmbeddingModel != "example/embedding-model" || len(item.Vector) != 2 {
			t.Fatalf("chunk %d has invalid embedding metadata: %+v", index, item)
		}
	}
}

func TestChunkerRejectsDuplicateDocumentIDs(t *testing.T) {
	chunker, err := NewChunker(80, 10)
	if err != nil {
		t.Fatal(err)
	}

	_, err = chunker.Split(context.Background(), []Document{
		{ID: "same", Content: "first"},
		{ID: "same", Content: "second"},
	})
	if err == nil || !strings.Contains(err.Error(), "duplicate document ID") {
		t.Fatalf("error = %v, want duplicate document ID", err)
	}
}
