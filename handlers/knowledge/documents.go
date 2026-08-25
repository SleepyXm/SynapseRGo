package knowledge

import (
	"database/sql"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"Synapse/rag"
	"Synapse/storage"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const maxUploadBytes = 100 << 20

type DocumentHandler struct {
	repository rag.Repository
	objects    storage.ObjectStore
}

func NewDocumentHandler(repository rag.Repository, objects storage.ObjectStore) *DocumentHandler {
	return &DocumentHandler{repository: repository, objects: objects}
}

func normalizeMediaType(filename, supplied string) (string, bool) {
	extension := strings.ToLower(filepath.Ext(filename))
	switch extension {
	case ".pdf":
		return "application/pdf", true
	case ".md", ".markdown":
		return "text/markdown", true
	case ".txt":
		return "text/plain", true
	}
	parsed, _, _ := mime.ParseMediaType(supplied)
	if parsed == "application/pdf" || parsed == "text/plain" || parsed == "text/markdown" {
		return parsed, true
	}
	return "", false
}

func (h *DocumentHandler) Upload(c *gin.Context) {
	baseID, ok := requireID(c, "knowledge_base_id")
	if !ok {
		return
	}
	userID := c.GetString("userID")
	if _, err := h.repository.GetKnowledgeBase(c, userID, baseID); errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "knowledge_not_found", "Knowledge base not found")
		return
	} else if err != nil {
		apiError(c, http.StatusInternalServerError, "knowledge_read_failed", "Could not load knowledge base")
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		apiError(c, http.StatusBadRequest, "file_required", "A PDF, Markdown, or text file is required")
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > maxUploadBytes {
		apiError(c, http.StatusRequestEntityTooLarge, "file_too_large", "File must be between 1 byte and 100 MiB")
		return
	}
	mediaType, accepted := normalizeMediaType(fileHeader.Filename, fileHeader.Header.Get("Content-Type"))
	if !accepted {
		apiError(c, http.StatusUnsupportedMediaType, "unsupported_document", "Only PDF, Markdown, and plain text files are supported")
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		apiError(c, http.StatusBadRequest, "file_open_failed", "Could not read uploaded file")
		return
	}
	defer file.Close()
	documentID := uuid.NewString()
	objectKey := fmt.Sprintf("users/%s/knowledge/%s/%s", userID, baseID, documentID)
	info, err := h.objects.Put(c, objectKey, http.MaxBytesReader(c.Writer, file, maxUploadBytes))
	if err != nil {
		apiError(c, http.StatusInternalServerError, "file_store_failed", "Could not store uploaded file")
		return
	}
	document, err := h.repository.CreateDocument(c, rag.DocumentRecord{
		ID:              documentID,
		KnowledgeBaseID: baseID,
		Filename:        filepath.Base(fileHeader.Filename),
		MediaType:       mediaType,
		ObjectKey:       objectKey,
		SHA256:          info.SHA256,
		SizeBytes:       info.SizeBytes,
	})
	if err != nil {
		_ = h.objects.Delete(c, objectKey)
		apiError(c, http.StatusConflict, "document_exists", "This file already exists in the knowledge base")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": document})
}

func (h *DocumentHandler) List(c *gin.Context) {
	baseID, ok := requireID(c, "knowledge_base_id")
	if !ok {
		return
	}
	documents, err := h.repository.ListDocuments(c, c.GetString("userID"), baseID)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "document_list_failed", "Could not list documents")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": documents})
}

func (h *DocumentHandler) Get(c *gin.Context) {
	baseID, ok := requireID(c, "knowledge_base_id")
	if !ok {
		return
	}
	documentID, ok := requireID(c, "document_id")
	if !ok {
		return
	}
	document, err := h.repository.GetDocument(c, c.GetString("userID"), baseID, documentID)
	if errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "document_not_found", "Document not found")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "document_read_failed", "Could not load document")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": document})
}

func (h *DocumentHandler) Retry(c *gin.Context) {
	baseID, ok := requireID(c, "knowledge_base_id")
	if !ok {
		return
	}
	documentID, ok := requireID(c, "document_id")
	if !ok {
		return
	}
	if err := h.repository.RetryDocument(c, c.GetString("userID"), baseID, documentID); errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusConflict, "document_not_failed", "Only a failed document can be retried")
		return
	} else if err != nil {
		apiError(c, http.StatusInternalServerError, "document_retry_failed", "Could not retry document")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"data": gin.H{"id": documentID, "status": "queued"}})
}

func (h *DocumentHandler) Delete(c *gin.Context) {
	baseID, ok := requireID(c, "knowledge_base_id")
	if !ok {
		return
	}
	documentID, ok := requireID(c, "document_id")
	if !ok {
		return
	}
	document, err := h.repository.DeleteDocument(c, c.GetString("userID"), baseID, documentID)
	if errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "document_not_found", "Document not found")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "document_delete_failed", "Could not delete document")
		return
	}
	if err := h.objects.Delete(c, document.ObjectKey); err != nil {
		apiError(c, http.StatusInternalServerError, "document_object_delete_failed", "Document metadata was removed but its local source file could not be deleted")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true}})
}
