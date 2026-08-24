package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"Synapse/structs"

	"github.com/gin-gonic/gin"
)

func Add(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req structs.FavoriteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hf_id is required"})
			return
		}
		req.HFID = strings.TrimSpace(req.HFID)
		if req.HFID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hf_id is required"})
			return
		}
		provider := strings.SplitN(req.HFID, "/", 2)[0]
		_, err := db.ExecContext(c, `WITH existing AS (
            SELECT id FROM llms WHERE name = $2 LIMIT 1
        ), created AS (
            INSERT INTO llms (name, provider) SELECT $2, $3 WHERE NOT EXISTS (SELECT 1 FROM existing) RETURNING id
        ), model AS (
            SELECT id FROM existing UNION ALL SELECT id FROM created LIMIT 1
        )
        INSERT INTO user_favorites (user_id, llm_id) SELECT $1::uuid, id FROM model ON CONFLICT DO NOTHING`, c.GetString("userID"), req.HFID, provider)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not add favourite"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Favourite added"})
	}
}

func Remove(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req structs.FavoriteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hf_id is required"})
			return
		}
		req.HFID = strings.TrimSpace(req.HFID)
		if req.HFID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hf_id is required"})
			return
		}
		_, err := db.ExecContext(c, `DELETE FROM user_favorites f USING llms l
			WHERE f.llm_id = l.id AND f.user_id = $1::uuid AND l.name = $2`, c.GetString("userID"), req.HFID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not remove favourite"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "success", "message": "Favourite removed"})
	}
}
