package routes

import (
	"database/sql"

	handlers "Synapse/handlers/favorites"
	"Synapse/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterFavoriteRoutes(rg *gin.RouterGroup, db *sql.DB) {
	auth := middleware.AuthMiddleware(db)
	rg.POST("/add_fav", auth, handlers.Add(db))
	rg.POST("/remove_fav", auth, handlers.Remove(db))
}
