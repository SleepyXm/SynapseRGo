package handlers

import (
	"api/test/structs"
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func SaveChunk(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		conversationID := c.Param("conversation_id")
		userID := c.GetString("userID")

		var messages []map[string]any
		if err := c.ShouldBindJSON(&messages); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		manager := NewConversationManager(conversationID, userID)
		if err := manager.Load(db); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}

		manager.Append(messages)

		if err := manager.Persist(db); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok", "chunk_size": len(messages)})
	}
}

func LoadChunks(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		conversationID := c.Param("conversation_id")
		userID := c.GetString("userID")

		manager := NewConversationManager(conversationID, userID)
		if err := manager.Load(db); err != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}

		messages := []map[string]any{}
		for _, m := range manager.Messages {
			messages = append(messages, map[string]any{
				"id":         m.ID,
				"role":       m.Role,
				"message":    m.Message,
				"created_at": m.CreatedAt,
				"metadata":   m.Metadata,
			})
		}

		c.JSON(http.StatusOK, gin.H{"messages": messages})
	}
}

func ListConversations(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")

		rows, err := db.Query(
			`SELECT id, title, llm_model, created_at, updated_at FROM conversations
             WHERE user_id = $1::uuid ORDER BY updated_at DESC`,
			userID,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		var conversations []gin.H
		for rows.Next() {
			var id, llmModel string
			var title *string
			var createdAt time.Time
			var updatedAt *time.Time
			if err := rows.Scan(&id, &title, &llmModel, &createdAt, &updatedAt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "scan failed"})
				return
			}
			conversations = append(conversations, gin.H{
				"id":        id,
				"title":     title,
				"llm_model": llmModel,
			})
		}

		c.JSON(http.StatusOK, gin.H{"conversations": conversations})
	}
}

func CreateConversation(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")

		var req structs.CreateConversationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		conversationID := uuid.New().String()
		_, err := db.Exec(
			`INSERT INTO conversations (id, user_id, llm_model, title, created_at, updated_at)
             VALUES ($1, $2, $3, NULL, NOW(), NOW())`,
			conversationID, userID, req.LLMModel,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create conversation"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"id": conversationID})
	}
}
