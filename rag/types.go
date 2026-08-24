// Package rag prepares documents for retrieval-augmented generation.
//
// This package currently owns ingestion preparation only: it chunks extracted
// text and produces embeddings. Storage, retrieval, and prompt grounding must
// be added as explicit application features rather than hidden inside Prepare.
package rag

import "context"

// Document is extracted text ready for chunking. File parsing belongs before
// this boundary so PDF, Markdown, and other formats can share the same pipeline.
type Document struct {
	ID       string
	Source   string
	Content  string
	Metadata map[string]string
}

// Chunk is a source-addressable passage from one document.
type Chunk struct {
	ID         string
	DocumentID string
	Source     string
	Index      int
	Content    string
	Metadata   map[string]string
}

// EmbeddedChunk records which model produced its vector. Vectors from
// different embedding models must never be mixed in the same retrieval index.
type EmbeddedChunk struct {
	Chunk
	EmbeddingModel string
	Vector         []float32
}

// Embedder converts text batches into vectors and identifies the model used.
// Implementations may call a remote provider or a future local model.
type Embedder interface {
	Model() string
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}
