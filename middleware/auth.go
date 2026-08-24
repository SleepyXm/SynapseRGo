package middleware

import (
	"database/sql"
	"net/http"

	"Synapse/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie("access_token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authentication token"})
			c.Abort()
			return
		}

		userID, err := utils.DecodeAccessToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authentication credentials"})
			c.Abort()
			return
		}

		var id, username, email string
		err = db.QueryRowContext(c, "SELECT id, username, COALESCE(email, '') FROM users WHERE id = $1", userID).Scan(&id, &username, &email)
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			c.Abort()
			return
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not verify session"})
			c.Abort()
			return
		}

		c.Set("userID", id)
		c.Set("username", username)
		c.Set("email", email)

		c.Next()
	}
}
