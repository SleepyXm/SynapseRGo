package agents

import (
	"context"
	"time"

	"Synapse/rag"
)

type ModelSettings struct {
	Temperature      float64 `json:"temperature"`
	TopP             float64 `json:"top_p"`
	MaxTokens        int     `json:"max_tokens"`
	PresencePenalty  float64 `json:"presence_penalty"`
	FrequencyPenalty float64 `json:"frequency_penalty"`
}

type Limits struct {
	MaxSteps       int `json:"max_steps"`
	TimeoutSeconds int `json:"timeout_seconds"`
}

type Agent struct {
	ID               string        `json:"id"`
	UserID           string        `json:"-"`
	Name             string        `json:"name"`
	Description      string        `json:"description"`
	Instructions     string        `json:"instructions"`
	ModelID          string        `json:"model_id"`
	HFTokenName      string        `json:"hf_token_name"`
	ToolIDs          []string      `json:"tool_ids"`
	KnowledgeBaseIDs []string      `json:"knowledge_base_ids"`
	Settings         ModelSettings `json:"settings"`
	Limits           Limits        `json:"limits"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

type Message struct {
	Role    string
	Content string
}

type RunConfig struct {
	AgentID        *string
	UserID         string
	ConversationID string
	Input          string
	ModelID        string
	HFTokenName    string
	Instructions   string
	ToolIDs        []string
	KnowledgeBases []rag.KnowledgeBase
	Settings       ModelSettings
	Limits         Limits
	History        []Message
}

type Run struct {
	ID             string     `json:"id"`
	UserID         string     `json:"-"`
	AgentID        *string    `json:"agent_id,omitempty"`
	ConversationID string     `json:"conversation_id"`
	ModelID        string     `json:"model_id"`
	Status         string     `json:"status"`
	Input          string     `json:"input"`
	Output         *string    `json:"output,omitempty"`
	FailureCode    *string    `json:"failure_code,omitempty"`
	FailureMessage *string    `json:"failure_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Steps          []RunStep  `json:"steps"`
}

type RunStep struct {
	Ordinal   int            `json:"ordinal"`
	EventType string         `json:"event_type"`
	ToolName  *string        `json:"tool_name,omitempty"`
	Payload   map[string]any `json:"payload"`
	CreatedAt time.Time      `json:"created_at"`
}

type Event struct {
	Type string         `json:"-"`
	Data map[string]any `json:"data"`
}

type Emitter func(ctx context.Context, event Event) error

type RunResult struct {
	RunID    string
	Output   string
	Evidence []rag.SearchResult
}
