package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"Synapse/structs"
	"Synapse/utils"

	"github.com/gin-gonic/gin"
)

func AddHFToken(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")

		var req structs.HFTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || req.HFToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name and hf_token are required"})
			return
		}

		var raw []byte
		if err := db.QueryRow("SELECT hf_tokens FROM users WHERE id = $1", userID).Scan(&raw); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
			return
		}

		var tokens []structs.HFToken
		if raw != nil {
			json.Unmarshal(raw, &tokens)
		}

		for _, t := range tokens {
			if t.Name == req.Name {
				c.JSON(http.StatusBadRequest, gin.H{"error": "token name already exists"})
				return
			}
		}

		encrypted, err := utils.Encrypt(req.HFToken)
		if err != nil {
			log.Printf("HF token encryption failed for user %s: %v", userID, err)

			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "encryption failed",
			})
			return
		}

		tokens = append(tokens, structs.HFToken{Name: req.Name, Value: encrypted})
		updated, _ := json.Marshal(tokens)

		if _, err := db.Exec("UPDATE users SET hf_tokens = $1 WHERE id = $2", updated, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update"})
			return
		}

		names := make([]string, len(tokens))
		for i, t := range tokens {
			names[i] = t.Name
		}

		c.JSON(http.StatusOK, gin.H{"message": "HF Token added successfully", "hf_token_names": names})
	}
}

func RemoveHFToken(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("userID")

		var req structs.RemoveHFTokenRequest
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
			return
		}

		var raw []byte
		if err := db.QueryRow("SELECT hf_tokens FROM users WHERE id = $1", userID).Scan(&raw); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "query failed"})
			return
		}

		var tokens []structs.HFToken
		if raw != nil {
			json.Unmarshal(raw, &tokens)
		}

		found := false
		filtered := []structs.HFToken{}
		for _, t := range tokens {
			if t.Name == req.Name {
				found = true
				continue
			}
			filtered = append(filtered, t)
		}

		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
			return
		}

		updated, _ := json.Marshal(filtered)
		if _, err := db.Exec("UPDATE users SET hf_tokens = $1 WHERE id = $2", updated, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update"})
			return
		}

		names := make([]string, len(filtered))
		for i, t := range filtered {
			names[i] = t.Name
		}

		c.JSON(http.StatusOK, gin.H{"message": "HF Token removed successfully", "hf_token_names": names})
	}
}

// GetDecryptedToken — call this from your chat handler instead of receiving the raw token from frontend
func GetDecryptedToken(db *sql.DB, userID, tokenName string) (string, error) {
	var raw []byte
	if err := db.QueryRow("SELECT hf_tokens FROM users WHERE id = $1", userID).Scan(&raw); err != nil {
		return "", err
	}

	var tokens []structs.HFToken
	if err := json.Unmarshal(raw, &tokens); err != nil {
		return "", err
	}

	for _, t := range tokens {
		if t.Name == tokenName {
			return utils.Decrypt(t.Value)
		}
	}

	return "", errors.New("token not found")
}
