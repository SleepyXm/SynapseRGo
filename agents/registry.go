package agents

import (
	"errors"
	"sort"
)

const KnowledgeSearchToolID = "knowledge.search"

type ToolDescriptor struct {
	ID          string `json:"id"`
	Function    string `json:"function"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Registry struct {
	tools map[string]ToolDescriptor
}

func NewRegistry() *Registry {
	return &Registry{tools: map[string]ToolDescriptor{
		KnowledgeSearchToolID: {
			ID:          KnowledgeSearchToolID,
			Function:    "knowledge_search",
			Name:        "Knowledge search",
			Description: "Search an attached knowledge base for source passages relevant to a question.",
		},
	}}
}

func (r *Registry) List() []ToolDescriptor {
	tools := make([]ToolDescriptor, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].ID < tools[j].ID })
	return tools
}

func (r *Registry) Validate(ids []string) error {
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, exists := r.tools[id]; !exists {
			return errors.New("unknown or unapproved tool: " + id)
		}
		if _, duplicate := seen[id]; duplicate {
			return errors.New("duplicate tool: " + id)
		}
		seen[id] = struct{}{}
	}
	return nil
}
