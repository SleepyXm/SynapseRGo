package routes

import (
	"api/test/handlers"
	"api/test/middleware"
	"database/sql"

	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(rg *gin.RouterGroup, db *sql.DB, jwtSecret []byte) {
	rg.POST("/signup", handlers.Signup(db))
	rg.POST("/login", handlers.Login(db, jwtSecret))
	rg.GET("/me", middleware.AuthMiddleware(db, jwtSecret), handlers.Me(db))
	rg.POST("/logout", handlers.Logout)
	rg.GET("/hi", handlers.Hi)
}
