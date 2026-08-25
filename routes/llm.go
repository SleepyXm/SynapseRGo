package routes

import (
	"database/sql"

	"Synapse/agents"
	handlers "Synapse/handlers/llm"
	"Synapse/middleware"
	"Synapse/rag"

	"github.com/gin-gonic/gin"
)

func RegisterLLMRoutes(rg *gin.RouterGroup, db *sql.DB, runtime *agents.Runtime, knowledge rag.Repository, registry *agents.Registry) {
	rg.POST("/chat/stream", middleware.AuthMiddleware(db), handlers.ChatStream(db, runtime, knowledge, registry))
}
