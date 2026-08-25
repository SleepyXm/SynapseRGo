package structs

type AgentLimitsRequest struct {
	MaxSteps       int `json:"max_steps"`
	TimeoutSeconds int `json:"timeout_seconds"`
}

type AgentRequest struct {
	Name             string             `json:"name" binding:"required"`
	Description      string             `json:"description" binding:"required"`
	Instructions     string             `json:"instructions" binding:"required"`
	ModelID          string             `json:"model_id" binding:"required"`
	HFTokenName      string             `json:"hf_token_name" binding:"required"`
	ToolIDs          []string           `json:"tool_ids"`
	KnowledgeBaseIDs []string           `json:"knowledge_base_ids"`
	Settings         ModelSettings      `json:"settings"`
	Limits           AgentLimitsRequest `json:"limits"`
}

type AgentRunRequest struct {
	ConversationID string `json:"conversation_id" binding:"required"`
	Input          string `json:"input" binding:"required"`
}

type KnowledgeChatRequest struct {
	ConversationID   string        `json:"conversation_id" binding:"required"`
	Input            string        `json:"input" binding:"required"`
	ModelID          string        `json:"model_id" binding:"required"`
	HFTokenName      string        `json:"hf_token_name" binding:"required"`
	KnowledgeBaseIDs []string      `json:"knowledge_base_ids"`
	Settings         ModelSettings `json:"settings"`
}
