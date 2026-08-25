package routes

import (
	"database/sql"

	knowledgehandlers "Synapse/handlers/knowledge"
	"Synapse/middleware"
	"Synapse/rag"
	"Synapse/storage"

	"github.com/gin-gonic/gin"
)

func RegisterKnowledgeRoutes(
	rg *gin.RouterGroup,
	db *sql.DB,
	repository rag.Repository,
	search *rag.SearchService,
	objects storage.ObjectStore,
) {
	auth := middleware.AuthMiddleware(db)
	knowledge := knowledgehandlers.NewHandler(repository, search, objects, db)
	documents := knowledgehandlers.NewDocumentHandler(repository, objects)

	rg.Use(auth)
	rg.POST("", knowledge.Create)
	rg.GET("", knowledge.List)
	rg.GET("/:knowledge_base_id", knowledge.Get)
	rg.PATCH("/:knowledge_base_id", knowledge.Update)
	rg.DELETE("/:knowledge_base_id", knowledge.Delete)
	rg.POST("/:knowledge_base_id/search", knowledge.Search)
	rg.POST("/:knowledge_base_id/documents", documents.Upload)
	rg.GET("/:knowledge_base_id/documents", documents.List)
	rg.GET("/:knowledge_base_id/documents/:document_id", documents.Get)
	rg.POST("/:knowledge_base_id/documents/:document_id/retry", documents.Retry)
	rg.DELETE("/:knowledge_base_id/documents/:document_id", documents.Delete)
}
