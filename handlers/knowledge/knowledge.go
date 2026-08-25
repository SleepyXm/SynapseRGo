package knowledge

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	tokens "Synapse/handlers/tokens"
	"Synapse/rag"
	"Synapse/storage"
	"Synapse/structs"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	repository rag.Repository
	search     *rag.SearchService
	objects    storage.ObjectStore
	db         *sql.DB
}

func NewHandler(repository rag.Repository, search *rag.SearchService, objects storage.ObjectStore, db *sql.DB) *Handler {
	return &Handler{repository: repository, search: search, objects: objects, db: db}
}

func apiError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"error": gin.H{"code": code, "message": message}})
}

func requireID(c *gin.Context, name string) (string, bool) {
	value := c.Param(name)
	if _, err := uuid.Parse(value); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_id", "Invalid resource identifier")
		return "", false
	}
	return value, true
}

func validateKnowledgeText(name, description string) error {
	if len([]rune(strings.TrimSpace(name))) < 1 || len([]rune(strings.TrimSpace(name))) > 100 {
		return errors.New("name must contain between 1 and 100 characters")
	}
	if len([]rune(strings.TrimSpace(description))) < 1 || len([]rune(strings.TrimSpace(description))) > 500 {
		return errors.New("description must contain between 1 and 500 characters")
	}
	return nil
}

func (h *Handler) Create(c *gin.Context) {
	var request structs.CreateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_request", "Invalid knowledge-base request")
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.EmbeddingModelID = strings.TrimSpace(request.EmbeddingModelID)
	request.HFTokenName = strings.TrimSpace(request.HFTokenName)
	if err := validateKnowledgeText(request.Name, request.Description); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if request.EmbeddingModelID == "" || request.HFTokenName == "" {
		apiError(c, http.StatusBadRequest, "invalid_request", "Embedding model and Hugging Face token name are required")
		return
	}
	if request.Chunking.SizeRunes == 0 {
		request.Chunking.SizeRunes = 1500
	}
	if request.Chunking.OverlapRunes == 0 {
		request.Chunking.OverlapRunes = 200
	}
	if _, err := rag.NewChunker(request.Chunking.SizeRunes, request.Chunking.OverlapRunes); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_chunking", err.Error())
		return
	}
	userID := c.GetString("userID")
	if _, err := tokens.GetDecryptedToken(h.db, userID, request.HFTokenName); err != nil {
		apiError(c, http.StatusBadRequest, "credential_not_found", "The selected Hugging Face token could not be loaded")
		return
	}
	base, err := h.repository.CreateKnowledgeBase(c, rag.KnowledgeBase{
		UserID:            userID,
		Name:              request.Name,
		Description:       request.Description,
		EmbeddingModelID:  request.EmbeddingModelID,
		HFTokenName:       request.HFTokenName,
		ChunkSizeRunes:    request.Chunking.SizeRunes,
		ChunkOverlapRunes: request.Chunking.OverlapRunes,
	})
	if err != nil {
		apiError(c, http.StatusInternalServerError, "knowledge_create_failed", "Could not create knowledge base")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": base})
}

func (h *Handler) List(c *gin.Context) {
	bases, err := h.repository.ListKnowledgeBases(c, c.GetString("userID"))
	if err != nil {
		apiError(c, http.StatusInternalServerError, "knowledge_list_failed", "Could not list knowledge bases")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": bases})
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := requireID(c, "knowledge_base_id")
	if !ok {
		return
	}
	base, err := h.repository.GetKnowledgeBase(c, c.GetString("userID"), id)
	if errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "knowledge_not_found", "Knowledge base not found")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "knowledge_read_failed", "Could not load knowledge base")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": base})
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := requireID(c, "knowledge_base_id")
	if !ok {
		return
	}
	var request structs.UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_request", "Invalid knowledge-base request")
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	if err := validateKnowledgeText(request.Name, request.Description); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	base, err := h.repository.UpdateKnowledgeBase(c, c.GetString("userID"), id, request.Name, request.Description)
	if errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "knowledge_not_found", "Knowledge base not found")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "knowledge_update_failed", "Could not update knowledge base")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": base})
}

func (h *Handler) Delete(c *gin.Context) {
	id, ok := requireID(c, "knowledge_base_id")
	if !ok {
		return
	}
	userID := c.GetString("userID")
	documents, err := h.repository.ListDocuments(c, userID, id)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "knowledge_read_failed", "Could not load knowledge-base documents")
		return
	}
	if err := h.repository.DeleteKnowledgeBase(c, userID, id); errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "knowledge_not_found", "Knowledge base not found")
		return
	} else if err != nil {
		apiError(c, http.StatusInternalServerError, "knowledge_delete_failed", "Could not delete knowledge base")
		return
	}
	for _, document := range documents {
		if err := h.objects.Delete(c, document.ObjectKey); err != nil {
			apiError(c, http.StatusInternalServerError, "knowledge_object_delete_failed", "Knowledge metadata was removed but a local source file could not be deleted")
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true}})
}

func (h *Handler) Search(c *gin.Context) {
	id, ok := requireID(c, "knowledge_base_id")
	if !ok {
		return
	}
	var request structs.KnowledgeSearchRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_request", "A search query is required")
		return
	}
	results, err := h.search.Search(c, c.GetString("userID"), id, request.Query, request.Limit)
	if errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "knowledge_not_found", "Knowledge base not found")
		return
	}
	if err != nil {
		apiError(c, http.StatusBadGateway, "knowledge_search_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"results": results}})
}
