package routes

import (
	"database/sql"

	"github.com/SleepyXm/SynapseRGo/middleware"

	"github.com/SleepyXm/SynapseRGo/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterAuthRoutes(rg *gin.RouterGroup, db *sql.DB, jwtSecret []byte) {
	rg.POST("/signup", handlers.Signup(db))
	rg.POST("/login", handlers.Login(db, jwtSecret))
	rg.GET("/me", middleware.AuthMiddleware(db, jwtSecret), handlers.Me(db))
	rg.POST("/logout", handlers.Logout)
	rg.GET("/hi", handlers.Hi)
}
