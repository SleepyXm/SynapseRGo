package routes

import (
	"database/sql"

	"Synapse/agents"
	agenthandlers "Synapse/handlers/agents"
	"Synapse/middleware"
	"Synapse/rag"

	"github.com/gin-gonic/gin"
)

func RegisterAgentRoutes(
	api *gin.RouterGroup,
	db *sql.DB,
	repository agents.Repository,
	knowledgeRepository rag.Repository,
	runtime *agents.Runtime,
	registry *agents.Registry,
) {
	auth := middleware.AuthMiddleware(db)
	handler := agenthandlers.NewHandler(repository, knowledgeRepository, registry, db)
	runs := agenthandlers.NewRunHandler(repository, knowledgeRepository, runtime, registry, db)

	api.GET("/tools", auth, handler.Tools)
	agentRoutes := api.Group("/agents", auth)
	agentRoutes.POST("", handler.Create)
	agentRoutes.GET("", handler.List)
	agentRoutes.GET("/:agent_id", handler.Get)
	agentRoutes.PATCH("/:agent_id", handler.Update)
	agentRoutes.DELETE("/:agent_id", handler.Delete)
	agentRoutes.POST("/:agent_id/runs/stream", runs.RunSaved)
	api.GET("/runs/:run_id", auth, runs.Get)
}
