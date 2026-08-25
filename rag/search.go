package rag

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type TokenResolver func(ctx context.Context, userID, tokenName string) (string, error)

type SearchService struct {
	repository Repository
	tokens     TokenResolver
}

func NewSearchService(repository Repository, tokens TokenResolver) *SearchService {
	return &SearchService{repository: repository, tokens: tokens}
}

func (s *SearchService) Search(ctx context.Context, userID, knowledgeBaseID, query string, limit int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("search query is required")
	}
	if limit == 0 {
		limit = 6
	}
	if limit < 1 || limit > 20 {
		return nil, errors.New("search limit must be between 1 and 20")
	}
	base, err := s.repository.GetKnowledgeBase(ctx, userID, knowledgeBaseID)
	if err != nil {
		return nil, err
	}
	token, err := s.tokens(ctx, userID, base.HFTokenName)
	if err != nil {
		return nil, fmt.Errorf("load embedding credential: %w", err)
	}
	embedder, err := NewHuggingFaceEmbedder(base.EmbeddingModelID, token)
	if err != nil {
		return nil, err
	}
	vectors, err := embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 || len(vectors[0]) == 0 {
		return nil, errors.New("embedding provider returned no query vector")
	}
	if base.EmbeddingDimension != nil && *base.EmbeddingDimension != len(vectors[0]) {
		return nil, errors.New("query embedding dimension does not match knowledge base")
	}
	return s.repository.Search(ctx, userID, knowledgeBaseID, vectors[0], limit)
}
