package handlers

import (
	"Synapse/structs"
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	conversations "Synapse/handlers/conversations"
	tokens "Synapse/handlers/tokens"

	"github.com/gin-gonic/gin"
)

func generateTitle(hfToken, modelID, firstMessage string) string {
	messages := []structs.LLMMessage{
		{
			Role:    "system",
			Content: "You are an assistant that creates short, descriptive titles for conversations.",
		},
		{
			Role:    "user",
			Content: "Generate a short concise title for the following: " + firstMessage,
		},
	}

	maxTitleTokens := 12

	payload, err := json.Marshal(structs.OpenAIRequest{
		Model:     modelID,
		Messages:  messages,
		Stream:    false,
		MaxTokens: &maxTitleTokens,
	})
	if err != nil {
		return "Untitled Conversation"
	}

	httpReq, err := http.NewRequest(
		"POST",
		"https://router.huggingface.co/v1/chat/completions",
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return "Untitled Conversation"
	}

	httpReq.Header.Set("Authorization", "Bearer "+hfToken)
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "Untitled Conversation"
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "Untitled Conversation"
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "Untitled Conversation"
	}

	if len(result.Choices) > 0 {
		title := strings.TrimSpace(result.Choices[0].Message.Content)
		title = strings.Trim(title, `"`)
		if title != "" {
			return title
		}
	}

	return "Untitled Conversation"
}
func ChatStream(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		conversationID := c.Query("conversation_id")
		if conversationID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id required"})
			return
		}

		userID := c.GetString("userID")

		var req structs.ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}
		// Conversation history belongs to the backend. The public chat endpoint
		// accepts exactly one new user turn so clients cannot re-persist history or
		// inject messages under privileged roles.
		if len(req.Conversation) != 1 || req.Conversation[0].Role != "user" || strings.TrimSpace(req.Conversation[0].Content) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "conversation must contain exactly one non-empty user message"})
			return
		}

		if strings.TrimSpace(req.HFTokenName) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hfTokenName required"})
			return
		}

		hfToken, err := tokens.GetDecryptedToken(db, userID, req.HFTokenName)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to load HF token"})
			return
		}

		manager := conversations.NewConversationManager(conversationID, userID)
		if err := manager.Load(db); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}

		messages := manager.GetMemorySnapshot(20)
		for _, m := range req.Conversation {
			messages = append(messages, structs.LLMMessage{
				Role:    m.Role,
				Content: m.Content,
			})
		}

		newMessages := []map[string]any{}
		for _, m := range req.Conversation {
			newMessages = append(newMessages, map[string]any{
				"role":    m.Role,
				"content": m.Content,
			})
		}

		manager.Append(newMessages)
		if err := manager.Persist(db); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save conversation"})
			return
		}

		payload, err := json.Marshal(structs.OpenAIRequest{
			Model:            req.ModelID,
			Messages:         messages,
			Stream:           true,
			MaxTokens:        req.Settings.MaxTokens,
			Temperature:      req.Settings.Temperature,
			TopP:             req.Settings.TopP,
			PresencePenalty:  req.Settings.PresencePenalty,
			FrequencyPenalty: req.Settings.FrequencyPenalty,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build payload"})
			return
		}

		httpReq, err := http.NewRequestWithContext(
			c.Request.Context(),
			http.MethodPost,
			"https://router.huggingface.co/v1/chat/completions",
			bytes.NewBuffer(payload),
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
			return
		}

		httpReq.Header.Set("Authorization", "Bearer "+hfToken)
		httpReq.Header.Set("Content-Type", "application/json")

		httpClient := &http.Client{Timeout: 120 * time.Second}
		resp, err := httpClient.Do(httpReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reach LLM"})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			var hfErr map[string]any
			_ = json.NewDecoder(resp.Body).Decode(&hfErr)

			c.JSON(resp.StatusCode, gin.H{
				"error": "hugging face request failed",
				"hf":    hfErr,
			})
			return
		}

		c.Header("Content-Type", "text/plain")
		c.Header("Transfer-Encoding", "chunked")
		c.Status(http.StatusOK)

		var assistantContent strings.Builder
		scanner := bufio.NewScanner(resp.Body)

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" || line == "data: [DONE]" {
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				var chunk structs.StreamChunk
				if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk); err != nil {
					continue
				}

				if len(chunk.Choices) > 0 {
					delta := chunk.Choices[0].Delta.Content
					if delta != "" {
						assistantContent.WriteString(delta)
						fmt.Fprint(c.Writer, delta)
						c.Writer.Flush()
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("stream scanner error: %v", err)
		}

		manager.Append([]map[string]any{
			{"role": "assistant", "content": assistantContent.String()},
		})
		if err := manager.Persist(db); err != nil {
			log.Printf("failed to save assistant response for conversation %s: %v", conversationID, err)
		}

		go func() {
			var existingTitle *string
			err := db.QueryRow(
				"SELECT title FROM conversations WHERE id = $1",
				conversationID,
			).Scan(&existingTitle)

			if err == nil && existingTitle != nil && strings.TrimSpace(*existingTitle) != "" {
				return
			}

			firstMessage := ""
			for _, m := range messages {
				if m.Role == "user" {
					firstMessage = m.Content
					break
				}
			}

			if firstMessage == "" {
				return
			}

			title := generateTitle(hfToken, req.ModelID, firstMessage)

			_, _ = db.Exec(
				"UPDATE conversations SET title = $1 WHERE id = $2",
				title,
				conversationID,
			)
		}()
	}
}
