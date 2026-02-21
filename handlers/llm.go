package handlers

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/SleepyXm/SynapseRGo/structs"

	"github.com/gin-gonic/gin"
)

func generateTitle(hfToken, modelID, firstMessage string) string {
	messages := []structs.LLMMessage{
		{Role: "system", Content: "You are an assistant that creates short, descriptive titles for conversations."},
		{Role: "user", Content: "Generate a short concise title for the following: " + firstMessage},
	}

	payload, _ := json.Marshal(structs.OpenAIRequest{
		Model:     modelID,
		Messages:  messages,
		Stream:    false,
		MaxTokens: 12,
	})

	req, _ := http.NewRequest("POST", "https://router.huggingface.co/v1/chat/completions", bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+hfToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "Untitled Conversation"
	}
	defer resp.Body.Close()

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
		return title
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

		// Load conversation memory
		manager := NewConversationManager(conversationID, userID)
		if err := manager.Load(db); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}

		// Memory + current request
		messages := manager.GetMemorySnapshot(20)
		for _, m := range req.Conversation {
			messages = append(messages, structs.LLMMessage{Role: m.Role, Content: m.Content})
		}

		// Persist new messages in background
		newMessages := []map[string]any{}
		for _, m := range req.Conversation {
			newMessages = append(newMessages, map[string]any{
				"role":    m.Role,
				"content": m.Content,
			})
		}
		manager.Append(newMessages)
		go manager.Persist(db)

		// Build OpenAI compatible request
		payload, _ := json.Marshal(structs.OpenAIRequest{
			Model:    req.ModelID,
			Messages: messages,
			Stream:   true,
		})

		httpReq, err := http.NewRequest("POST", "https://router.huggingface.co/v1/chat/completions", bytes.NewBuffer(payload))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build request"})
			return
		}
		httpReq.Header.Set("Authorization", "Bearer "+req.HFToken)
		httpReq.Header.Set("Content-Type", "application/json")

		httpClient := &http.Client{Timeout: 120 * time.Second}
		resp, err := httpClient.Do(httpReq)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reach LLM"})
			return
		}
		defer resp.Body.Close()

		// Stream back to client
		c.Header("Content-Type", "text/plain")
		c.Header("Transfer-Encoding", "chunked")
		c.Status(http.StatusOK)

		var assistantContent string
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" || line == "data: [DONE]" {
				continue
			}
			if len(line) > 6 && line[:6] == "data: " {
				var chunk structs.StreamChunk
				if err := json.Unmarshal([]byte(line[6:]), &chunk); err != nil {
					continue
				}
				if len(chunk.Choices) > 0 {
					delta := chunk.Choices[0].Delta.Content
					if delta != "" {
						assistantContent += delta
						fmt.Fprint(c.Writer, delta)
						c.Writer.Flush()
					}
				}
			}
		}

		// Persist assistant response + generate title in background
		go func() {
			manager.Append([]map[string]any{
				{"role": "assistant", "content": assistantContent},
			})
			manager.Persist(db)

			// Only generate title if none exists
			var existingTitle *string
			db.QueryRow("SELECT title FROM conversations WHERE id = $1", conversationID).Scan(&existingTitle)
			if existingTitle != nil {
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

			title := generateTitle(req.HFToken, req.ModelID, firstMessage)
			db.Exec("UPDATE conversations SET title = $1 WHERE id = $2", title, conversationID)
		}()
	}
}
