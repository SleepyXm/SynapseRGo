package routes

import (
	"database/sql"

	"Synapse/handlers"
	"Synapse/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterLLMRoutes(rg *gin.RouterGroup, db *sql.DB, jwtSecret []byte) {

	rg.POST("/chat/stream", middleware.AuthMiddleware(db, jwtSecret), handlers.ChatStream(db))
}
