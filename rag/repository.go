package rag

import (
	"context"
	"errors"
)

var ErrNoQueuedDocuments = errors.New("no queued knowledge documents")

type Repository interface {
	CreateKnowledgeBase(ctx context.Context, base KnowledgeBase) (KnowledgeBase, error)
	ListKnowledgeBases(ctx context.Context, userID string) ([]KnowledgeBase, error)
	GetKnowledgeBase(ctx context.Context, userID, id string) (KnowledgeBase, error)
	GetKnowledgeBaseInternal(ctx context.Context, id string) (KnowledgeBase, error)
	UpdateKnowledgeBase(ctx context.Context, userID, id, name, description string) (KnowledgeBase, error)
	DeleteKnowledgeBase(ctx context.Context, userID, id string) error

	CreateDocument(ctx context.Context, document DocumentRecord) (DocumentRecord, error)
	ListDocuments(ctx context.Context, userID, knowledgeBaseID string) ([]DocumentRecord, error)
	GetDocument(ctx context.Context, userID, knowledgeBaseID, documentID string) (DocumentRecord, error)
	DeleteDocument(ctx context.Context, userID, knowledgeBaseID, documentID string) (DocumentRecord, error)
	RetryDocument(ctx context.Context, userID, knowledgeBaseID, documentID string) error
	RecoverProcessingDocuments(ctx context.Context) error
	ClaimNextDocument(ctx context.Context) (DocumentRecord, error)
	SavePreparedDocument(ctx context.Context, base KnowledgeBase, document DocumentRecord, chunks []EmbeddedChunk) error
	FailDocument(ctx context.Context, documentID, reason string) error

	Search(ctx context.Context, userID, knowledgeBaseID string, vector []float32, limit int) ([]SearchResult, error)
}
