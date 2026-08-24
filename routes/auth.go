package routes

import (
	"database/sql"

	handlers "Synapse/handlers/auth"
	"Synapse/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(rg *gin.RouterGroup, db *sql.DB) {
	rg.POST("/signup", handlers.Signup(db))
	rg.POST("/login", handlers.Login(db))
	rg.POST("/refresh", handlers.Refresh())
	rg.POST("/logout", handlers.Logout())
	rg.GET("/me", middleware.AuthMiddleware(db), handlers.Me(db))
	rg.GET("/hi", handlers.Hi())
}
