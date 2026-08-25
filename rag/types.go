// Package rag prepares documents for retrieval-augmented generation.
//
// This package currently owns ingestion preparation only: it chunks extracted
// text and produces embeddings. Storage, retrieval, and prompt grounding must
// be added as explicit application features rather than hidden inside Prepare.
package rag

import (
	"context"
	"time"
)

// Document is extracted text ready for chunking. File parsing belongs before
// this boundary so PDF, Markdown, and other formats can share the same pipeline.
type Document struct {
	// ID identifies one extracted segment, such as a PDF page. DocumentID
	// identifies the uploaded source shared by all of its segments.
	ID         string
	DocumentID string
	Source     string
	Content    string
	Metadata   map[string]string
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

type KnowledgeBase struct {
	ID                 string    `json:"id"`
	UserID             string    `json:"-"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	EmbeddingModelID   string    `json:"embedding_model_id"`
	HFTokenName        string    `json:"hf_token_name"`
	EmbeddingDimension *int      `json:"embedding_dimension"`
	ChunkSizeRunes     int       `json:"chunk_size_runes"`
	ChunkOverlapRunes  int       `json:"chunk_overlap_runes"`
	ReadyDocuments     int       `json:"ready_documents"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type DocumentRecord struct {
	ID              string    `json:"id"`
	KnowledgeBaseID string    `json:"knowledge_base_id"`
	Filename        string    `json:"filename"`
	MediaType       string    `json:"media_type"`
	ObjectKey       string    `json:"-"`
	SHA256          string    `json:"sha256"`
	SizeBytes       int64     `json:"size_bytes"`
	Status          string    `json:"status"`
	FailureReason   *string   `json:"failure_reason"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type SearchResult struct {
	CitationID  string  `json:"citation_id"`
	DocumentID  string  `json:"document_id"`
	Filename    string  `json:"filename"`
	Page        *int    `json:"page,omitempty"`
	ChunkIndex  int     `json:"chunk_index"`
	Content     string  `json:"content"`
	Score       float64 `json:"score"`
	KnowledgeID string  `json:"knowledge_base_id"`
}
