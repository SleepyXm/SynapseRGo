package agents

import "context"

type Repository interface {
	CreateAgent(ctx context.Context, agent Agent) (Agent, error)
	ListAgents(ctx context.Context, userID string) ([]Agent, error)
	GetAgent(ctx context.Context, userID, id string) (Agent, error)
	UpdateAgent(ctx context.Context, agent Agent) (Agent, error)
	DeleteAgent(ctx context.Context, userID, id string) error
	CreateRun(ctx context.Context, config RunConfig) (Run, error)
	AddRunStep(ctx context.Context, runID string, step RunStep) error
	CompleteRun(ctx context.Context, runID, output string) error
	FailRun(ctx context.Context, runID, code, message string) error
	GetRun(ctx context.Context, userID, id string) (Run, error)
}
