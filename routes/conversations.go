package routes

import (
	"api/test/middleware"
	"database/sql"

	"api/test/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterConversationRoutes(rg *gin.RouterGroup, db *sql.DB, jwtSecret []byte) {
	auth := middleware.AuthMiddleware(db, jwtSecret)

	rg.POST("/create", auth, handlers.CreateConversation(db))
	rg.GET("/list", middleware.AuthMiddleware(db, jwtSecret), handlers.ListConversations(db))
	rg.POST("/:conversation_id/chunk", middleware.AuthMiddleware(db, jwtSecret), handlers.SaveChunk(db))
	rg.GET("/:conversation_id/chunk", middleware.AuthMiddleware(db, jwtSecret), handlers.LoadChunks(db))
}
