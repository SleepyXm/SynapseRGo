package rag

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/textsplitter"
)

const (
	documentIDMetadataKey = "_synapse_document_id"
	sourceMetadataKey     = "_synapse_source"
)

// Chunker wraps LangChainGo's recursive character splitter. Both size values
// are Unicode rune counts, not model-token counts; the names make that choice
// visible to callers instead of silently presenting it as a universal policy.
type Chunker struct {
	splitter textsplitter.TextSplitter
}

func NewChunker(chunkSizeRunes, overlapRunes int) (*Chunker, error) {
	if chunkSizeRunes <= 0 {
		return nil, errors.New("chunk size must be positive")
	}
	if overlapRunes < 0 || overlapRunes >= chunkSizeRunes {
		return nil, errors.New("chunk overlap must be non-negative and smaller than chunk size")
	}

	return &Chunker{
		splitter: textsplitter.NewRecursiveCharacter(
			textsplitter.WithChunkSize(chunkSizeRunes),
			textsplitter.WithChunkOverlap(overlapRunes),
			textsplitter.WithKeepSeparator(true),
		),
	}, nil
}

// Split validates source identity, preserves caller metadata, and numbers each
// passage within its document. Duplicate document IDs are rejected because they
// would otherwise generate colliding chunk IDs.
func (c *Chunker) Split(ctx context.Context, documents []Document) ([]Chunk, error) {
	if c == nil || c.splitter == nil {
		return nil, errors.New("chunker is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	seenDocumentIDs := make(map[string]struct{}, len(documents))
	langChainDocuments := make([]schema.Document, 0, len(documents))
	for _, document := range documents {
		segmentID := strings.TrimSpace(document.ID)
		if segmentID == "" {
			return nil, errors.New("document ID is required")
		}
		if _, exists := seenDocumentIDs[segmentID]; exists {
			return nil, fmt.Errorf("duplicate document ID %q", segmentID)
		}
		seenDocumentIDs[segmentID] = struct{}{}

		documentID := strings.TrimSpace(document.DocumentID)
		if documentID == "" {
			documentID = segmentID
		}

		if strings.TrimSpace(document.Content) == "" {
			continue
		}

		source := strings.TrimSpace(document.Source)
		if source == "" {
			source = segmentID
		}
		metadata := make(map[string]any, len(document.Metadata)+2)
		for key, value := range document.Metadata {
			metadata[key] = value
		}
		// Internal identity is written last so caller metadata cannot replace it.
		metadata[documentIDMetadataKey] = documentID
		metadata[sourceMetadataKey] = source

		langChainDocuments = append(langChainDocuments, schema.Document{
			PageContent: document.Content,
			Metadata:    metadata,
		})
	}

	splitDocuments, err := textsplitter.SplitDocuments(c.splitter, langChainDocuments)
	if err != nil {
		return nil, fmt.Errorf("split documents: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	nextIndex := make(map[string]int, len(documents))
	chunks := make([]Chunk, 0, len(splitDocuments))
	for _, document := range splitDocuments {
		documentID, _ := document.Metadata[documentIDMetadataKey].(string)
		source, _ := document.Metadata[sourceMetadataKey].(string)
		nextIndex[documentID]++
		index := nextIndex[documentID]

		metadata := make(map[string]string, len(document.Metadata)-2)
		for key, value := range document.Metadata {
			if text, ok := value.(string); ok && key != documentIDMetadataKey && key != sourceMetadataKey {
				metadata[key] = text
			}
		}

		chunks = append(chunks, Chunk{
			ID:         fmt.Sprintf("%s:%d", documentID, index),
			DocumentID: documentID,
			Source:     source,
			Index:      index,
			Content:    strings.TrimSpace(document.PageContent),
			Metadata:   metadata,
		})
	}

	return chunks, nil
}
