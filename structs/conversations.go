package structs

import (
	"fmt"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	ModelID      string    `json:"modelId"`
	HFToken      string    `json:"hfToken" binding:"required"`
	Conversation []Message `json:"conversation" binding:"required"`
}

type CreateConversationRequest struct {
	Title    string `json:"title" binding:"required"`
	LLMModel string `json:"llm_model" binding:"required"`
}

type AddLLMRequest struct {
	LLMID   string `json:"llm_id" binding:"required"`
	LLMName string `json:"llm_name" binding:"required"`
}

type StoredMessage struct {
	ID        string         `json:"id"`
	Message   map[string]any `json:"message"`
	Role      string         `json:"role"`
	CreatedAt FlexTime       `json:"created_at"`
	Metadata  map[string]any `json:"metadata"`
}

type FlexTime struct {
	time.Time
}

func (ft *FlexTime) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	formats := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			ft.Time = t
			return nil
		}
	}
	return fmt.Errorf("cannot parse time: %s", s)
}
