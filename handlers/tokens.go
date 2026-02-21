package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/SleepyXm/SynapseRGo/structs"

	"github.com/gin-gonic/gin"
)

func AddHFToken(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")

		var req structs.HFTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		// Get current tokens
		var hfTokens []byte
		err := db.QueryRow("SELECT hf_tokens FROM users WHERE id = $1", userID).Scan(&hfTokens)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
			return
		}

		var currentTokens []string
		if hfTokens != nil {
			json.Unmarshal(hfTokens, &currentTokens)
		}

		// Check duplicate
		for _, t := range currentTokens {
			if t == req.HFToken {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Token already exists"})
				return
			}
		}

		currentTokens = append(currentTokens, req.HFToken)
		updated, _ := json.Marshal(currentTokens)

		_, err = db.Exec("UPDATE users SET hf_tokens = $1 WHERE id = $2", updated, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "HF Token added successfully", "hf_tokens": currentTokens})
	}
}

func RemoveHFToken(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")

		var req structs.HFTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		var hfTokens []byte
		err := db.QueryRow("SELECT hf_tokens FROM users WHERE id = $1", userID).Scan(&hfTokens)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
			return
		}

		var currentTokens []string
		if hfTokens != nil {
			json.Unmarshal(hfTokens, &currentTokens)
		}

		// Find and remove
		found := false
		filtered := []string{}
		for _, t := range currentTokens {
			if t == req.HFToken {
				found = true
				continue
			}
			filtered = append(filtered, t)
		}

		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
			return
		}

		updated, _ := json.Marshal(filtered)
		_, err = db.Exec("UPDATE users SET hf_tokens = $1 WHERE id = $2", updated, userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "HF Token deleted successfully", "hf_tokens": filtered})
	}
}
