package routes

import (
	"database/sql"

	handlers "Synapse/handlers/llm"
	"Synapse/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterLLMRoutes(rg *gin.RouterGroup, db *sql.DB) {
	rg.POST("/chat/stream", middleware.AuthMiddleware(db), handlers.ChatStream(db))
}
