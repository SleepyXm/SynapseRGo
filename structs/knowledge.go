package structs

type ChunkingRequest struct {
	SizeRunes    int `json:"size_runes"`
	OverlapRunes int `json:"overlap_runes"`
}

type CreateKnowledgeBaseRequest struct {
	Name             string          `json:"name" binding:"required"`
	Description      string          `json:"description" binding:"required"`
	EmbeddingModelID string          `json:"embedding_model_id" binding:"required"`
	HFTokenName      string          `json:"hf_token_name" binding:"required"`
	Chunking         ChunkingRequest `json:"chunking"`
}

type UpdateKnowledgeBaseRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description" binding:"required"`
}

type KnowledgeSearchRequest struct {
	Query string `json:"query" binding:"required"`
	Limit int    `json:"limit"`
}
