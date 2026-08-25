package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"Synapse/rag"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

type Runtime struct {
	repository Repository
	search     KnowledgeSearcher
	tokens     rag.TokenResolver
	registry   *Registry
	models     modelFactory
}

type modelFactory func(token string, config RunConfig) (llms.Model, error)

func NewRuntime(repository Repository, search KnowledgeSearcher, tokens rag.TokenResolver, registry *Registry) *Runtime {
	return &Runtime{repository: repository, search: search, tokens: tokens, registry: registry, models: newHuggingFaceModel}
}

func newHuggingFaceModel(token string, config RunConfig) (llms.Model, error) {
	return openai.New(
		openai.WithToken(token),
		openai.WithModel(config.ModelID),
		openai.WithBaseURL("https://router.huggingface.co/v1"),
		openai.WithHTTPClient(&http.Client{Timeout: time.Duration(config.Limits.TimeoutSeconds) * time.Second}),
	)
}

func (r *Runtime) Run(ctx context.Context, config RunConfig, emit Emitter) (RunResult, error) {
	run, err := r.repository.CreateRun(ctx, config)
	if err != nil {
		return RunResult{}, err
	}
	emitEvent := func(event Event) error {
		if emit == nil {
			return nil
		}
		return emit(ctx, event)
	}
	_ = emitEvent(Event{Type: "run.started", Data: map[string]any{"run_id": run.ID}})
	fail := func(code string, runErr error) (RunResult, error) {
		_ = r.repository.FailRun(context.Background(), run.ID, code, runErr.Error())
		_ = emitEvent(Event{Type: "run.failed", Data: map[string]any{
			"run_id": run.ID, "code": code, "message": runErr.Error(),
		}})
		return RunResult{RunID: run.ID}, runErr
	}

	if config.Limits.MaxSteps <= 0 {
		config.Limits.MaxSteps = 6
	}
	if config.Limits.MaxSteps > 10 {
		return fail("invalid_limits", errors.New("agent max_steps cannot exceed 10"))
	}
	if config.Limits.TimeoutSeconds <= 0 {
		config.Limits.TimeoutSeconds = 120
	}
	runContext, cancel := context.WithTimeout(ctx, time.Duration(config.Limits.TimeoutSeconds)*time.Second)
	defer cancel()

	token, err := r.tokens(runContext, config.UserID, config.HFTokenName)
	if err != nil {
		return fail("credential_not_found", errors.New("the selected Hugging Face token could not be loaded"))
	}
	model, err := r.models(token, config)
	if err != nil {
		return fail("model_initialization_failed", err)
	}

	messages := []llms.MessageContent{llms.TextParts(llms.ChatMessageTypeSystem, BuildSystemPrompt(config.Instructions, config.KnowledgeBases))}
	for _, history := range config.History {
		role := llms.ChatMessageTypeAI
		if history.Role == "user" {
			role = llms.ChatMessageTypeHuman
		}
		messages = append(messages, llms.TextParts(role, history.Content))
	}
	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, config.Input))

	allowedBases := make(map[string]struct{}, len(config.KnowledgeBases))
	baseIDs := make([]string, 0, len(config.KnowledgeBases))
	for _, base := range config.KnowledgeBases {
		allowedBases[base.ID] = struct{}{}
		baseIDs = append(baseIDs, base.ID)
	}
	tools := r.modelTools(config.ToolIDs, baseIDs)
	evidence := []rag.SearchResult{}
	ordinal := 0
	toolSteps := 0
	for {
		options := []llms.CallOption{
			llms.WithModel(config.ModelID),
			llms.WithMaxTokens(config.Settings.MaxTokens),
			llms.WithTemperature(config.Settings.Temperature),
			llms.WithTopP(config.Settings.TopP),
			llms.WithPresencePenalty(config.Settings.PresencePenalty),
			llms.WithFrequencyPenalty(config.Settings.FrequencyPenalty),
		}
		if len(tools) > 0 {
			options = append(options, llms.WithTools(tools), llms.WithToolChoice("auto"))
		}
		response, err := model.GenerateContent(runContext, messages, options...)
		if err != nil {
			code := "model_request_failed"
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "tool") && (strings.Contains(lower, "support") || strings.Contains(lower, "invalid")) {
				code = "model_tools_unsupported"
			}
			return fail(code, err)
		}
		if response == nil || len(response.Choices) == 0 {
			return fail("model_empty_response", errors.New("model returned no response"))
		}
		choice := response.Choices[0]
		if len(choice.ToolCalls) == 0 {
			output := strings.TrimSpace(choice.Content)
			if output == "" {
				return fail("model_empty_response", errors.New("model returned an empty response"))
			}
			if err := emitEvent(Event{Type: "assistant.delta", Data: map[string]any{"text": output}}); err != nil {
				return fail("stream_closed", err)
			}
			if err := r.repository.CompleteRun(context.Background(), run.ID, output); err != nil {
				return fail("run_persist_failed", err)
			}
			_ = emitEvent(Event{Type: "run.completed", Data: map[string]any{
				"run_id": run.ID, "evidence": evidence,
			}})
			return RunResult{RunID: run.ID, Output: output, Evidence: evidence}, nil
		}
		if toolSteps+len(choice.ToolCalls) > config.Limits.MaxSteps {
			return fail("max_steps_exceeded", errors.New("agent reached its tool-step limit"))
		}

		assistantParts := make([]llms.ContentPart, 0, len(choice.ToolCalls)+1)
		if strings.TrimSpace(choice.Content) != "" {
			assistantParts = append(assistantParts, llms.TextContent{Text: choice.Content})
		}
		for _, call := range choice.ToolCalls {
			assistantParts = append(assistantParts, call)
		}
		messages = append(messages, llms.MessageContent{Role: llms.ChatMessageTypeAI, Parts: assistantParts})

		for _, call := range choice.ToolCalls {
			toolSteps++
			if call.FunctionCall == nil || call.FunctionCall.Name != "knowledge_search" {
				return fail("tool_not_allowed", errors.New("model requested an unavailable tool"))
			}
			ordinal++
			started := map[string]any{"step": ordinal, "tool": "knowledge.search", "arguments": json.RawMessage(call.FunctionCall.Arguments)}
			toolName := KnowledgeSearchToolID
			if err := r.repository.AddRunStep(runContext, run.ID, RunStep{Ordinal: ordinal, EventType: "tool.started", ToolName: &toolName, Payload: started}); err != nil {
				return fail("run_persist_failed", err)
			}
			if err := emitEvent(Event{Type: "tool.started", Data: started}); err != nil {
				return fail("stream_closed", err)
			}
			results, knowledgeBaseID, err := executeKnowledgeSearch(runContext, r.search, config.UserID, allowedBases, call.FunctionCall.Arguments)
			if err != nil {
				return fail("tool_execution_failed", err)
			}
			evidence = append(evidence, results...)
			ordinal++
			completed := map[string]any{
				"step": ordinal, "tool": "knowledge.search", "knowledge_base_id": knowledgeBaseID,
				"result_count": len(results), "citations": results,
			}
			if err := r.repository.AddRunStep(runContext, run.ID, RunStep{Ordinal: ordinal, EventType: "tool.completed", ToolName: &toolName, Payload: completed}); err != nil {
				return fail("run_persist_failed", err)
			}
			if err := emitEvent(Event{Type: "tool.completed", Data: completed}); err != nil {
				return fail("stream_closed", err)
			}
			encoded, err := json.Marshal(map[string]any{"results": results})
			if err != nil {
				return fail("tool_result_failed", err)
			}
			messages = append(messages, llms.MessageContent{
				Role: llms.ChatMessageTypeTool,
				Parts: []llms.ContentPart{llms.ToolCallResponse{
					ToolCallID: call.ID, Name: call.FunctionCall.Name, Content: string(encoded),
				}},
			})
		}
	}
}

func (r *Runtime) modelTools(toolIDs, baseIDs []string) []llms.Tool {
	enabled := false
	for _, id := range toolIDs {
		if id == KnowledgeSearchToolID {
			enabled = true
		}
	}
	if !enabled || len(baseIDs) == 0 {
		return nil
	}
	return []llms.Tool{{
		Type: "function",
		Function: &llms.FunctionDefinition{
			Name:        "knowledge_search",
			Description: "Search one attached knowledge base for passages relevant to a question. Use the returned citation IDs when discussing evidence.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"knowledge_base_id": map[string]any{"type": "string", "enum": baseIDs},
					"query":             map[string]any{"type": "string"},
					"limit":             map[string]any{"type": "integer", "minimum": 1, "maximum": 20, "default": 6},
				},
				"required":             []string{"knowledge_base_id", "query"},
				"additionalProperties": false,
			},
		},
	}}
}

func ValidateRunConfig(config RunConfig, registry *Registry) error {
	if strings.TrimSpace(config.UserID) == "" || strings.TrimSpace(config.ConversationID) == "" {
		return errors.New("user and conversation are required")
	}
	if strings.TrimSpace(config.Input) == "" || strings.TrimSpace(config.ModelID) == "" || strings.TrimSpace(config.HFTokenName) == "" {
		return errors.New("input, model, and credential are required")
	}
	if err := registry.Validate(config.ToolIDs); err != nil {
		return err
	}
	if len(config.KnowledgeBases) > 0 {
		hasSearch := false
		for _, id := range config.ToolIDs {
			hasSearch = hasSearch || id == KnowledgeSearchToolID
		}
		if !hasSearch {
			return fmt.Errorf("%s is required when knowledge bases are attached", KnowledgeSearchToolID)
		}
	}
	return nil
}
