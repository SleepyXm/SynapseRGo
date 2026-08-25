package rag

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"Synapse/storage"
)

type Worker struct {
	repository Repository
	objects    storage.ObjectStore
	tokens     TokenResolver
	poll       time.Duration
}

func NewWorker(repository Repository, objects storage.ObjectStore, tokens TokenResolver, poll time.Duration) *Worker {
	if poll <= 0 {
		poll = 2 * time.Second
	}
	return &Worker{repository: repository, objects: objects, tokens: tokens, poll: poll}
}

func (w *Worker) Run(ctx context.Context) {
	if err := w.repository.RecoverProcessingDocuments(ctx); err != nil {
		log.Printf("knowledge worker recovery failed: %v", err)
	}
	ticker := time.NewTicker(w.poll)
	defer ticker.Stop()
	for {
		if err := w.processNext(ctx); err != nil && !errors.Is(err, ErrNoQueuedDocuments) && !errors.Is(err, context.Canceled) {
			log.Printf("knowledge worker error: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) processNext(ctx context.Context) error {
	document, err := w.repository.ClaimNextDocument(ctx)
	if err != nil {
		return err
	}
	fail := func(processErr error) error {
		if ctx.Err() == nil {
			_ = w.repository.FailDocument(context.Background(), document.ID, processErr.Error())
		}
		return processErr
	}
	base, err := w.repository.GetKnowledgeBaseInternal(ctx, document.KnowledgeBaseID)
	if err != nil {
		return fail(fmt.Errorf("load knowledge base: %w", err))
	}
	object, err := w.objects.Open(ctx, document.ObjectKey)
	if err != nil {
		return fail(err)
	}
	defer object.Close()
	extracted, err := LoadExtractedDocuments(ctx, document.ID, document.Filename, document.MediaType, object)
	if err != nil {
		return fail(err)
	}
	chunker, err := NewChunker(base.ChunkSizeRunes, base.ChunkOverlapRunes)
	if err != nil {
		return fail(err)
	}
	token, err := w.tokens(ctx, base.UserID, base.HFTokenName)
	if err != nil {
		return fail(fmt.Errorf("load embedding credential: %w", err))
	}
	embedder, err := NewHuggingFaceEmbedder(base.EmbeddingModelID, token)
	if err != nil {
		return fail(err)
	}
	prepared, err := (Preparer{Chunker: chunker, Embedder: embedder, BatchSize: 16}).Prepare(ctx, extracted)
	if err != nil {
		return fail(err)
	}
	if err := w.repository.SavePreparedDocument(ctx, base, document, prepared); err != nil {
		return fail(err)
	}
	return nil
}
