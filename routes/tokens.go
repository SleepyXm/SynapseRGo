package routes

import (
	"api/test/handlers"
	"api/test/middleware"
	"database/sql"

	"github.com/gin-gonic/gin"
)

func RegisterTokenRoutes(rg *gin.RouterGroup, db *sql.DB, jwtSecret []byte) {
	auth := middleware.AuthMiddleware(db, jwtSecret)

	rg.POST("/hf_token", auth, handlers.AddHFToken(db))
	rg.DELETE("/hf_token", middleware.AuthMiddleware(db, jwtSecret), handlers.RemoveHFToken(db))
}
