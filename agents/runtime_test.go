package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"Synapse/rag"

	"github.com/tmc/langchaingo/llms"
)

type fakeAgentRepository struct {
	completed string
	failed    string
	steps     []RunStep
}

func (r *fakeAgentRepository) CreateAgent(context.Context, Agent) (Agent, error)   { return Agent{}, nil }
func (r *fakeAgentRepository) ListAgents(context.Context, string) ([]Agent, error) { return nil, nil }
func (r *fakeAgentRepository) GetAgent(context.Context, string, string) (Agent, error) {
	return Agent{}, nil
}
func (r *fakeAgentRepository) UpdateAgent(context.Context, Agent) (Agent, error)   { return Agent{}, nil }
func (r *fakeAgentRepository) DeleteAgent(context.Context, string, string) error   { return nil }
func (r *fakeAgentRepository) GetRun(context.Context, string, string) (Run, error) { return Run{}, nil }
func (r *fakeAgentRepository) CreateRun(_ context.Context, config RunConfig) (Run, error) {
	return Run{ID: "run-id", UserID: config.UserID, ConversationID: config.ConversationID}, nil
}
func (r *fakeAgentRepository) AddRunStep(_ context.Context, _ string, step RunStep) error {
	r.steps = append(r.steps, step)
	return nil
}
func (r *fakeAgentRepository) CompleteRun(_ context.Context, _ string, output string) error {
	r.completed = output
	return nil
}
func (r *fakeAgentRepository) FailRun(_ context.Context, _ string, code, _ string) error {
	r.failed = code
	return nil
}

type fakeKnowledgeSearcher struct {
	results []rag.SearchResult
}

func (s fakeKnowledgeSearcher) Search(context.Context, string, string, string, int) ([]rag.SearchResult, error) {
	return s.results, nil
}

type fakeModel struct {
	responses []*llms.ContentResponse
	calls     [][]llms.MessageContent
}

func (m *fakeModel) GenerateContent(_ context.Context, messages []llms.MessageContent, options ...llms.CallOption) (*llms.ContentResponse, error) {
	copyOfMessages := append([]llms.MessageContent(nil), messages...)
	m.calls = append(m.calls, copyOfMessages)
	if len(m.responses) == 0 {
		return nil, errors.New("unexpected model call")
	}
	response := m.responses[0]
	m.responses = m.responses[1:]
	return response, nil
}

func (m *fakeModel) Call(context.Context, string, ...llms.CallOption) (string, error) {
	return "", errors.New("not used")
}

func testRunConfig() RunConfig {
	return RunConfig{
		UserID: "user-id", ConversationID: "conversation-id", Input: "Find the notice",
		ModelID: "test/model", HFTokenName: "primary", Instructions: "Answer from evidence.",
		Settings: ModelSettings{Temperature: 0.2, TopP: 0.9, MaxTokens: 100},
		Limits:   Limits{MaxSteps: 2, TimeoutSeconds: 5},
	}
}

func testRuntime(repository *fakeAgentRepository, search KnowledgeSearcher, model llms.Model) *Runtime {
	runtime := NewRuntime(repository, search, func(context.Context, string, string) (string, error) {
		return "test-token", nil
	}, NewRegistry())
	runtime.models = func(string, RunConfig) (llms.Model, error) { return model, nil }
	return runtime
}

func TestRuntimeCompletesWithoutToolCall(t *testing.T) {
	repository := &fakeAgentRepository{}
	model := &fakeModel{responses: []*llms.ContentResponse{{Choices: []*llms.ContentChoice{{Content: "Direct answer"}}}}}
	runtime := testRuntime(repository, fakeKnowledgeSearcher{}, model)
	var events []string
	result, err := runtime.Run(context.Background(), testRunConfig(), func(_ context.Context, event Event) error {
		events = append(events, event.Type)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Output != "Direct answer" || repository.completed != "Direct answer" {
		t.Fatalf("unexpected completion: %+v, persisted %q", result, repository.completed)
	}
	if strings.Join(events, ",") != "run.started,assistant.delta,run.completed" {
		t.Fatalf("unexpected event order: %v", events)
	}
}

func TestRuntimePlacesRetrievedTextInToolMessage(t *testing.T) {
	const sourceText = "SOURCE TEXT THAT MUST NOT BECOME A SYSTEM INSTRUCTION"
	repository := &fakeAgentRepository{}
	model := &fakeModel{responses: []*llms.ContentResponse{
		{Choices: []*llms.ContentChoice{{ToolCalls: []llms.ToolCall{{
			ID: "call-1", Type: "function", FunctionCall: &llms.FunctionCall{
				Name: "knowledge_search", Arguments: `{"knowledge_base_id":"base-id","query":"notice"}`,
			},
		}}}}},
		{Choices: []*llms.ContentChoice{{Content: "Evidence-backed answer"}}},
	}}
	search := fakeKnowledgeSearcher{results: []rag.SearchResult{{
		CitationID: "document-id:0", DocumentID: "document-id", Filename: "case.txt",
		ChunkIndex: 0, Content: sourceText, Score: 0.9, KnowledgeID: "base-id",
	}}}
	runtime := testRuntime(repository, search, model)
	config := testRunConfig()
	config.ToolIDs = []string{KnowledgeSearchToolID}
	config.KnowledgeBases = []rag.KnowledgeBase{{ID: "base-id", Name: "Case", Description: "Case evidence", ReadyDocuments: 1}}
	var events []string
	result, err := runtime.Run(context.Background(), config, func(_ context.Context, event Event) error {
		events = append(events, event.Type)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(model.calls) != 2 || len(result.Evidence) != 1 {
		t.Fatalf("expected one tool round trip: calls=%d evidence=%d", len(model.calls), len(result.Evidence))
	}
	for _, part := range model.calls[1][0].Parts {
		if textPart, ok := part.(llms.TextContent); ok && strings.Contains(textPart.Text, sourceText) {
			t.Fatal("retrieved source text was inserted into the system message")
		}
	}
	foundToolSource := false
	for _, message := range model.calls[1] {
		if message.Role != llms.ChatMessageTypeTool {
			continue
		}
		for _, part := range message.Parts {
			if response, ok := part.(llms.ToolCallResponse); ok && strings.Contains(response.Content, sourceText) {
				foundToolSource = true
			}
		}
	}
	if !foundToolSource {
		t.Fatal("retrieved source text was not returned as a tool message")
	}
	if strings.Join(events, ",") != "run.started,tool.started,tool.completed,assistant.delta,run.completed" {
		t.Fatalf("unexpected event order: %v", events)
	}
}

func TestRuntimeEnforcesToolStepLimitAcrossParallelCalls(t *testing.T) {
	repository := &fakeAgentRepository{}
	toolCall := func(id string) llms.ToolCall {
		return llms.ToolCall{ID: id, Type: "function", FunctionCall: &llms.FunctionCall{
			Name: "knowledge_search", Arguments: `{"knowledge_base_id":"base-id","query":"notice"}`,
		}}
	}
	model := &fakeModel{responses: []*llms.ContentResponse{{Choices: []*llms.ContentChoice{{
		ToolCalls: []llms.ToolCall{toolCall("call-1"), toolCall("call-2")},
	}}}}}
	runtime := testRuntime(repository, fakeKnowledgeSearcher{}, model)
	config := testRunConfig()
	config.Limits.MaxSteps = 1
	config.ToolIDs = []string{KnowledgeSearchToolID}
	config.KnowledgeBases = []rag.KnowledgeBase{{ID: "base-id", Name: "Case", Description: "Evidence"}}
	_, err := runtime.Run(context.Background(), config, nil)
	if err == nil || repository.failed != "max_steps_exceeded" {
		t.Fatalf("expected max_steps_exceeded, got error %v and code %q", err, repository.failed)
	}
	if len(repository.steps) != 0 {
		t.Fatalf("tool calls should be rejected before execution: %+v", repository.steps)
	}
}
