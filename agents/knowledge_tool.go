package agents

import (
	"context"
	"encoding/json"
	"errors"

	"Synapse/rag"
)

type knowledgeSearchInput struct {
	KnowledgeBaseID string `json:"knowledge_base_id"`
	Query           string `json:"query"`
	Limit           int    `json:"limit"`
}

type KnowledgeSearcher interface {
	Search(ctx context.Context, userID, knowledgeBaseID, query string, limit int) ([]rag.SearchResult, error)
}

func executeKnowledgeSearch(
	ctx context.Context,
	search KnowledgeSearcher,
	userID string,
	allowed map[string]struct{},
	arguments string,
) ([]rag.SearchResult, string, error) {
	var input knowledgeSearchInput
	if err := json.Unmarshal([]byte(arguments), &input); err != nil {
		return nil, "", errors.New("knowledge_search arguments must be valid JSON")
	}
	if _, ok := allowed[input.KnowledgeBaseID]; !ok {
		return nil, input.KnowledgeBaseID, errors.New("knowledge base is not attached to this run")
	}
	results, err := search.Search(ctx, userID, input.KnowledgeBaseID, input.Query, input.Limit)
	return results, input.KnowledgeBaseID, err
}
