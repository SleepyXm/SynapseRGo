package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"Synapse/agents"
	agenthandlers "Synapse/handlers/agents"
	conversations "Synapse/handlers/conversations"
	tokens "Synapse/handlers/tokens"
	"Synapse/rag"
	"Synapse/structs"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func generateTitle(hfToken, modelID, firstMessage string) string {
	messages := []structs.LLMMessage{
		{Role: "system", Content: "You are an assistant that creates short, descriptive titles for conversations."},
		{Role: "user", Content: "Generate a short concise title for the following: " + firstMessage},
	}
	maxTitleTokens := 12
	payload, err := json.Marshal(structs.OpenAIRequest{Model: modelID, Messages: messages, Stream: false, MaxTokens: &maxTitleTokens})
	if err != nil {
		return "Untitled Conversation"
	}
	request, err := http.NewRequest("POST", "https://router.huggingface.co/v1/chat/completions", bytes.NewBuffer(payload))
	if err != nil {
		return "Untitled Conversation"
	}
	request.Header.Set("Authorization", "Bearer "+hfToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return "Untitled Conversation"
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "Untitled Conversation"
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil || len(result.Choices) == 0 {
		return "Untitled Conversation"
	}
	title := strings.Trim(strings.TrimSpace(result.Choices[0].Message.Content), `"`)
	if title == "" {
		return "Untitled Conversation"
	}
	return title
}

func pointerValue(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func ChatStream(db *sql.DB, runtime *agents.Runtime, knowledge rag.Repository, registry *agents.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request structs.KnowledgeChatRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "conversation_id, input, model_id, and hf_token_name are required"}})
			return
		}
		request.Input = strings.TrimSpace(request.Input)
		request.ModelID = strings.TrimSpace(request.ModelID)
		request.HFTokenName = strings.TrimSpace(request.HFTokenName)
		if request.Input == "" || request.ModelID == "" || request.HFTokenName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_request", "message": "Input, model, and token name cannot be empty"}})
			return
		}
		if _, err := uuid.Parse(request.ConversationID); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_conversation_id", "message": "Invalid conversation identifier"}})
			return
		}
		userID := c.GetString("userID")
		hfToken, err := tokens.GetDecryptedToken(db, userID, request.HFTokenName)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "credential_not_found", "message": "The selected Hugging Face token could not be loaded"}})
			return
		}
		bases := make([]rag.KnowledgeBase, 0, len(request.KnowledgeBaseIDs))
		for _, baseID := range request.KnowledgeBaseIDs {
			if _, err := uuid.Parse(baseID); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_knowledge_id", "message": "Invalid knowledge-base identifier"}})
				return
			}
			base, err := knowledge.GetKnowledgeBase(c, userID, baseID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "knowledge_not_found", "message": "A selected knowledge base could not be loaded"}})
				return
			}
			bases = append(bases, base)
		}
		manager := conversations.NewConversationManager(request.ConversationID, userID)
		if err := manager.Load(db); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "conversation_forbidden", "message": "Conversation not found or unavailable"}})
			return
		}
		history := manager.GetHistorySnapshot(20)
		runtimeHistory := make([]agents.Message, 0, len(history))
		for _, message := range history {
			runtimeHistory = append(runtimeHistory, agents.Message{Role: message.Role, Content: message.Content})
		}
		maxTokens := 1024
		if request.Settings.MaxTokens != nil {
			maxTokens = *request.Settings.MaxTokens
		}
		toolIDs := []string{}
		if len(bases) > 0 {
			toolIDs = []string{agents.KnowledgeSearchToolID}
		}
		config := agents.RunConfig{
			UserID: userID, ConversationID: request.ConversationID, Input: request.Input,
			ModelID: request.ModelID, HFTokenName: request.HFTokenName,
			Instructions: "Answer the user directly and accurately. Use attached knowledge when it can materially improve or support the answer.",
			ToolIDs:      toolIDs, KnowledgeBases: bases, History: runtimeHistory,
			Settings: agents.ModelSettings{
				Temperature: pointerValue(request.Settings.Temperature, 0.7), TopP: pointerValue(request.Settings.TopP, 0.95),
				MaxTokens: maxTokens, PresencePenalty: pointerValue(request.Settings.PresencePenalty, 0),
				FrequencyPenalty: pointerValue(request.Settings.FrequencyPenalty, 0),
			},
			Limits: agents.Limits{MaxSteps: 6, TimeoutSeconds: 120},
		}
		if err := agents.ValidateRunConfig(config, registry); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_run", "message": err.Error()}})
			return
		}
		manager.Append([]map[string]any{{"role": "user", "content": config.Input}})
		if err := manager.Persist(db); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "conversation_save_failed", "message": "Could not save user message"}})
			return
		}
		agenthandlers.PrepareSSE(c)
		result, err := runtime.Run(c.Request.Context(), config, agenthandlers.SSEEmitter(c))
		if err != nil {
			return
		}
		manager.Append([]map[string]any{{"role": "assistant", "content": result.Output}})
		_ = manager.Persist(db)

		go func() {
			var existingTitle *string
			if err := db.QueryRow("SELECT title FROM conversations WHERE id = $1", request.ConversationID).Scan(&existingTitle); err != nil {
				return
			}
			if existingTitle != nil && strings.TrimSpace(*existingTitle) != "" {
				return
			}
			_, _ = db.Exec("UPDATE conversations SET title = $1 WHERE id = $2", generateTitle(hfToken, request.ModelID, request.Input), request.ConversationID)
		}()
	}
}
