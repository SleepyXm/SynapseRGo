package agents

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"Synapse/agents"
	tokens "Synapse/handlers/tokens"
	"Synapse/rag"
	"Synapse/structs"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	repository          agents.Repository
	knowledgeRepository rag.Repository
	registry            *agents.Registry
	db                  *sql.DB
}

func NewHandler(repository agents.Repository, knowledgeRepository rag.Repository, registry *agents.Registry, db *sql.DB) *Handler {
	return &Handler{repository: repository, knowledgeRepository: knowledgeRepository, registry: registry, db: db}
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

func (h *Handler) Tools(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": h.registry.List()})
}

func (h *Handler) requestAgent(c *gin.Context, id string) (agents.Agent, bool) {
	var request structs.AgentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_request", "Invalid agent request")
		return agents.Agent{}, false
	}
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.Instructions = strings.TrimSpace(request.Instructions)
	request.ModelID = strings.TrimSpace(request.ModelID)
	request.HFTokenName = strings.TrimSpace(request.HFTokenName)
	if len([]rune(request.Name)) < 1 || len([]rune(request.Name)) > 100 ||
		len([]rune(request.Description)) < 1 || len([]rune(request.Description)) > 500 ||
		request.Instructions == "" || request.ModelID == "" || request.HFTokenName == "" {
		apiError(c, http.StatusBadRequest, "invalid_request", "Agent name, description, instructions, model, and token name are required")
		return agents.Agent{}, false
	}
	if err := h.registry.Validate(request.ToolIDs); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_tools", err.Error())
		return agents.Agent{}, false
	}
	if len(request.KnowledgeBaseIDs) > 0 {
		hasSearch := false
		for _, toolID := range request.ToolIDs {
			hasSearch = hasSearch || toolID == agents.KnowledgeSearchToolID
		}
		if !hasSearch {
			apiError(c, http.StatusBadRequest, "knowledge_tool_required", "knowledge.search is required when knowledge bases are attached")
			return agents.Agent{}, false
		}
	}
	userID := c.GetString("userID")
	seenKnowledgeBases := make(map[string]struct{}, len(request.KnowledgeBaseIDs))
	for _, baseID := range request.KnowledgeBaseIDs {
		if _, err := uuid.Parse(baseID); err != nil {
			apiError(c, http.StatusBadRequest, "invalid_knowledge_id", "Invalid knowledge-base identifier")
			return agents.Agent{}, false
		}
		if _, duplicate := seenKnowledgeBases[baseID]; duplicate {
			apiError(c, http.StatusBadRequest, "duplicate_knowledge_id", "A knowledge base can only be attached once")
			return agents.Agent{}, false
		}
		seenKnowledgeBases[baseID] = struct{}{}
		if _, err := h.knowledgeRepository.GetKnowledgeBase(c, userID, baseID); err != nil {
			apiError(c, http.StatusBadRequest, "knowledge_not_found", "An attached knowledge base could not be loaded")
			return agents.Agent{}, false
		}
	}
	if _, err := tokens.GetDecryptedToken(h.db, userID, request.HFTokenName); err != nil {
		apiError(c, http.StatusBadRequest, "credential_not_found", "The selected Hugging Face token could not be loaded")
		return agents.Agent{}, false
	}
	if request.Limits.MaxSteps == 0 {
		request.Limits.MaxSteps = 6
	}
	if request.Limits.TimeoutSeconds == 0 {
		request.Limits.TimeoutSeconds = 120
	}
	if request.Limits.MaxSteps < 1 || request.Limits.MaxSteps > 10 || request.Limits.TimeoutSeconds < 1 || request.Limits.TimeoutSeconds > 600 {
		apiError(c, http.StatusBadRequest, "invalid_limits", "max_steps must be 1-10 and timeout_seconds must be 1-600")
		return agents.Agent{}, false
	}
	if request.Settings.MaxTokens == nil {
		defaultTokens := 2048
		request.Settings.MaxTokens = &defaultTokens
	}
	return agents.Agent{
		ID:               id,
		UserID:           userID,
		Name:             request.Name,
		Description:      request.Description,
		Instructions:     request.Instructions,
		ModelID:          request.ModelID,
		HFTokenName:      request.HFTokenName,
		ToolIDs:          append([]string(nil), request.ToolIDs...),
		KnowledgeBaseIDs: append([]string(nil), request.KnowledgeBaseIDs...),
		Settings: agents.ModelSettings{
			Temperature:      valueOr(request.Settings.Temperature, 0.2),
			TopP:             valueOr(request.Settings.TopP, 0.9),
			MaxTokens:        *request.Settings.MaxTokens,
			PresencePenalty:  valueOr(request.Settings.PresencePenalty, 0),
			FrequencyPenalty: valueOr(request.Settings.FrequencyPenalty, 0),
		},
		Limits: agents.Limits{MaxSteps: request.Limits.MaxSteps, TimeoutSeconds: request.Limits.TimeoutSeconds},
	}, true
}

func valueOr(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func (h *Handler) Create(c *gin.Context) {
	agent, ok := h.requestAgent(c, "")
	if !ok {
		return
	}
	created, err := h.repository.CreateAgent(c, agent)
	if err != nil {
		apiError(c, http.StatusInternalServerError, "agent_create_failed", "Could not create agent")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

func (h *Handler) List(c *gin.Context) {
	items, err := h.repository.ListAgents(c, c.GetString("userID"))
	if err != nil {
		apiError(c, http.StatusInternalServerError, "agent_list_failed", "Could not list agents")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items})
}

func (h *Handler) Get(c *gin.Context) {
	id, ok := requireID(c, "agent_id")
	if !ok {
		return
	}
	agent, err := h.repository.GetAgent(c, c.GetString("userID"), id)
	if errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "agent_not_found", "Agent not found")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "agent_read_failed", "Could not load agent")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": agent})
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := requireID(c, "agent_id")
	if !ok {
		return
	}
	agent, ok := h.requestAgent(c, id)
	if !ok {
		return
	}
	updated, err := h.repository.UpdateAgent(c, agent)
	if errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "agent_not_found", "Agent not found")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "agent_update_failed", "Could not update agent")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": updated})
}

func (h *Handler) Delete(c *gin.Context) {
	id, ok := requireID(c, "agent_id")
	if !ok {
		return
	}
	if err := h.repository.DeleteAgent(c, c.GetString("userID"), id); errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "agent_not_found", "Agent not found")
		return
	} else if err != nil {
		apiError(c, http.StatusInternalServerError, "agent_delete_failed", "Could not delete agent")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"deleted": true}})
}
