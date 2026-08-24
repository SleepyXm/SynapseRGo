package routes

import (
	"database/sql"

	handlers "Synapse/handlers/tokens"
	"Synapse/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterTokenRoutes(rg *gin.RouterGroup, db *sql.DB) {
	auth := middleware.AuthMiddleware(db)

	rg.POST("/hf_token", auth, handlers.AddHFToken(db))
	rg.DELETE("/hf_token", auth, handlers.RemoveHFToken(db))
}
