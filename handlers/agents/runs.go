package agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	agentruntime "Synapse/agents"
	conversations "Synapse/handlers/conversations"
	"Synapse/rag"
	"Synapse/structs"

	"github.com/gin-gonic/gin"
)

type RunHandler struct {
	repository          agentruntime.Repository
	knowledgeRepository rag.Repository
	runtime             *agentruntime.Runtime
	registry            *agentruntime.Registry
	db                  *sql.DB
}

func NewRunHandler(repository agentruntime.Repository, knowledgeRepository rag.Repository, runtime *agentruntime.Runtime, registry *agentruntime.Registry, db *sql.DB) *RunHandler {
	return &RunHandler{repository: repository, knowledgeRepository: knowledgeRepository, runtime: runtime, registry: registry, db: db}
}

func SSEEmitter(c *gin.Context) agentruntime.Emitter {
	return func(_ context.Context, event agentruntime.Event) error {
		encoded, err := json.Marshal(event.Data)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event.Type, encoded); err != nil {
			return err
		}
		c.Writer.Flush()
		return nil
	}
}

func PrepareSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
}

func loadKnowledgeBases(ctx context.Context, repository rag.Repository, userID string, ids []string) ([]rag.KnowledgeBase, error) {
	bases := make([]rag.KnowledgeBase, 0, len(ids))
	for _, id := range ids {
		base, err := repository.GetKnowledgeBase(ctx, userID, id)
		if err != nil {
			return nil, err
		}
		bases = append(bases, base)
	}
	return bases, nil
}

func historyMessages(manager *conversations.ConversationManager) []agentruntime.Message {
	history := manager.GetHistorySnapshot(20)
	result := make([]agentruntime.Message, 0, len(history))
	for _, message := range history {
		result = append(result, agentruntime.Message{Role: message.Role, Content: message.Content})
	}
	return result
}

func (h *RunHandler) RunSaved(c *gin.Context) {
	agentID, ok := requireID(c, "agent_id")
	if !ok {
		return
	}
	var request structs.AgentRunRequest
	if err := c.ShouldBindJSON(&request); err != nil || strings.TrimSpace(request.Input) == "" {
		apiError(c, http.StatusBadRequest, "invalid_request", "conversation_id and input are required")
		return
	}
	userID := c.GetString("userID")
	agent, err := h.repository.GetAgent(c, userID, agentID)
	if errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "agent_not_found", "Agent not found")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "agent_read_failed", "Could not load agent")
		return
	}
	bases, err := loadKnowledgeBases(c, h.knowledgeRepository, userID, agent.KnowledgeBaseIDs)
	if err != nil {
		apiError(c, http.StatusBadRequest, "knowledge_not_found", "An attached knowledge base could not be loaded")
		return
	}
	manager := conversations.NewConversationManager(request.ConversationID, userID)
	if err := manager.Load(h.db); err != nil {
		apiError(c, http.StatusForbidden, "conversation_forbidden", "Conversation not found or unavailable")
		return
	}
	config := agentruntime.RunConfig{
		AgentID: &agent.ID, UserID: userID, ConversationID: request.ConversationID,
		Input: strings.TrimSpace(request.Input), ModelID: agent.ModelID, HFTokenName: agent.HFTokenName,
		Instructions: agent.Instructions, ToolIDs: agent.ToolIDs, KnowledgeBases: bases,
		Settings: agent.Settings, Limits: agent.Limits, History: historyMessages(manager),
	}
	if err := agentruntime.ValidateRunConfig(config, h.registry); err != nil {
		apiError(c, http.StatusBadRequest, "invalid_run", err.Error())
		return
	}
	manager.Append([]map[string]any{{"role": "user", "content": config.Input}})
	if err := manager.Persist(h.db); err != nil {
		apiError(c, http.StatusInternalServerError, "conversation_save_failed", "Could not save user message")
		return
	}
	PrepareSSE(c)
	result, err := h.runtime.Run(c.Request.Context(), config, SSEEmitter(c))
	if err != nil {
		return
	}
	manager.Append([]map[string]any{{"role": "assistant", "content": result.Output}})
	_ = manager.Persist(h.db)
}

func (h *RunHandler) Get(c *gin.Context) {
	runID, ok := requireID(c, "run_id")
	if !ok {
		return
	}
	run, err := h.repository.GetRun(c, c.GetString("userID"), runID)
	if errors.Is(err, sql.ErrNoRows) {
		apiError(c, http.StatusNotFound, "run_not_found", "Run not found")
		return
	}
	if err != nil {
		apiError(c, http.StatusInternalServerError, "run_read_failed", "Could not load run")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": run})
}
