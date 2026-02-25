package routes

import (
	"database/sql"

	"github.com/SleepyXm/SynapseRGo/middleware"

	"github.com/SleepyXm/SynapseRGo/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterConversationRoutes(rg *gin.RouterGroup, db *sql.DB, jwtSecret []byte) {
	auth := middleware.AuthMiddleware(db, jwtSecret)

	rg.POST("/create", auth, handlers.CreateConversation(db))
	rg.GET("/list", middleware.AuthMiddleware(db, jwtSecret), handlers.ListConversations(db))
	rg.GET("/:conversation_id/chunk", middleware.AuthMiddleware(db, jwtSecret), handlers.LoadChunks(db))
}
