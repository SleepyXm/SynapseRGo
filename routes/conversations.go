package routes

import (
	"database/sql"

	"Synapse/middleware"

	handlers "Synapse/handlers/conversations"

	"github.com/gin-gonic/gin"
)

func RegisterConversationRoutes(rg *gin.RouterGroup, db *sql.DB) {
	auth := middleware.AuthMiddleware(db)

	rg.POST("/create", auth, handlers.CreateConversation(db))
	rg.GET("/list", auth, handlers.ListConversations(db))
	rg.DELETE("/:conversation_id", auth, handlers.DeleteConversation(db))
	rg.PATCH("/:conversation_id", auth, handlers.UpdateConversation(db))
	rg.GET("/:conversation_id/chunk", auth, handlers.LoadChunks(db))
}
