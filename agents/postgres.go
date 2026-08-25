package agents

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

const agentSelect = `
SELECT a.id, a.user_id, a.name, a.description, a.instructions, a.model_id,
       a.hf_token_name, to_json(a.tool_ids), a.settings, a.max_steps, a.timeout_seconds,
       a.created_at, a.updated_at,
       COALESCE(array_agg(akb.knowledge_base_id::text) FILTER (WHERE akb.knowledge_base_id IS NOT NULL), '{}')
FROM agents a
LEFT JOIN agent_knowledge_bases akb ON akb.agent_id = a.id
`

func scanAgent(scanner interface{ Scan(...any) error }) (Agent, error) {
	var agent Agent
	var settings []byte
	var toolIDs []byte
	err := scanner.Scan(&agent.ID, &agent.UserID, &agent.Name, &agent.Description,
		&agent.Instructions, &agent.ModelID, &agent.HFTokenName, &toolIDs,
		&settings, &agent.Limits.MaxSteps, &agent.Limits.TimeoutSeconds,
		&agent.CreatedAt, &agent.UpdatedAt, &agent.KnowledgeBaseIDs)
	if err != nil {
		return Agent{}, err
	}
	if err := json.Unmarshal(settings, &agent.Settings); err != nil {
		return Agent{}, err
	}
	if err := json.Unmarshal(toolIDs, &agent.ToolIDs); err != nil {
		return Agent{}, err
	}
	if agent.ToolIDs == nil {
		agent.ToolIDs = []string{}
	}
	if agent.KnowledgeBaseIDs == nil {
		agent.KnowledgeBaseIDs = []string{}
	}
	return agent, nil
}

func (r *PostgresRepository) writeAgent(ctx context.Context, tx *sql.Tx, agent Agent, insert bool) error {
	settings, err := json.Marshal(agent.Settings)
	if err != nil {
		return err
	}
	toolIDs, err := json.Marshal(agent.ToolIDs)
	if err != nil {
		return err
	}
	if insert {
		_, err = tx.ExecContext(ctx, `
INSERT INTO agents
    (id, user_id, name, description, instructions, model_id, hf_token_name, tool_ids, settings, max_steps, timeout_seconds)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7,
        ARRAY(SELECT jsonb_array_elements_text($8::jsonb)), $9::jsonb, $10, $11)`,
			agent.ID, agent.UserID, agent.Name, agent.Description, agent.Instructions,
			agent.ModelID, agent.HFTokenName,
			string(toolIDs), string(settings),
			agent.Limits.MaxSteps, agent.Limits.TimeoutSeconds)
	} else {
		var result sql.Result
		result, err = tx.ExecContext(ctx, `
UPDATE agents SET name = $1, description = $2, instructions = $3, model_id = $4,
    hf_token_name = $5,
    tool_ids = ARRAY(SELECT jsonb_array_elements_text($6::jsonb)),
    settings = $7::jsonb, max_steps = $8,
    timeout_seconds = $9, updated_at = NOW()
WHERE id = $10::uuid AND user_id = $11::uuid`,
			agent.Name, agent.Description, agent.Instructions, agent.ModelID,
			agent.HFTokenName, string(toolIDs), string(settings), agent.Limits.MaxSteps,
			agent.Limits.TimeoutSeconds, agent.ID, agent.UserID)
		if err == nil {
			if affected, _ := result.RowsAffected(); affected == 0 {
				return sql.ErrNoRows
			}
		}
	}
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM agent_knowledge_bases WHERE agent_id = $1::uuid`, agent.ID); err != nil {
		return err
	}
	for _, knowledgeBaseID := range agent.KnowledgeBaseIDs {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO agent_knowledge_bases (agent_id, knowledge_base_id) VALUES ($1::uuid, $2::uuid)`,
			agent.ID, knowledgeBaseID); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepository) CreateAgent(ctx context.Context, agent Agent) (Agent, error) {
	agent.ID = uuid.NewString()
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Agent{}, err
	}
	defer tx.Rollback()
	if err := r.writeAgent(ctx, tx, agent, true); err != nil {
		return Agent{}, err
	}
	if err := tx.Commit(); err != nil {
		return Agent{}, err
	}
	return r.GetAgent(ctx, agent.UserID, agent.ID)
}

func (r *PostgresRepository) ListAgents(ctx context.Context, userID string) ([]Agent, error) {
	rows, err := r.db.QueryContext(ctx, agentSelect+`
WHERE a.user_id = $1::uuid
GROUP BY a.id
ORDER BY a.updated_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	agents := []Agent{}
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func (r *PostgresRepository) GetAgent(ctx context.Context, userID, id string) (Agent, error) {
	return scanAgent(r.db.QueryRowContext(ctx, agentSelect+`
WHERE a.id = $1::uuid AND a.user_id = $2::uuid
GROUP BY a.id`, id, userID))
}

func (r *PostgresRepository) UpdateAgent(ctx context.Context, agent Agent) (Agent, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Agent{}, err
	}
	defer tx.Rollback()
	if err := r.writeAgent(ctx, tx, agent, false); err != nil {
		return Agent{}, err
	}
	if err := tx.Commit(); err != nil {
		return Agent{}, err
	}
	return r.GetAgent(ctx, agent.UserID, agent.ID)
}

func (r *PostgresRepository) DeleteAgent(ctx context.Context, userID, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1::uuid AND user_id = $2::uuid`, id, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *PostgresRepository) CreateRun(ctx context.Context, config RunConfig) (Run, error) {
	run := Run{ID: uuid.NewString(), UserID: config.UserID, AgentID: config.AgentID,
		ConversationID: config.ConversationID, ModelID: config.ModelID, Status: "running", Input: config.Input}
	err := r.db.QueryRowContext(ctx, `
INSERT INTO agent_runs (id, user_id, agent_id, conversation_id, model_id, status, input)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, 'running', $6)
RETURNING created_at`, run.ID, run.UserID, run.AgentID, run.ConversationID, run.ModelID, run.Input).Scan(&run.CreatedAt)
	return run, err
}

func (r *PostgresRepository) AddRunStep(ctx context.Context, runID string, step RunStep) error {
	payload, err := json.Marshal(step.Payload)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO agent_run_steps (run_id, ordinal, event_type, tool_name, payload)
VALUES ($1::uuid, $2, $3, $4, $5::jsonb)`, runID, step.Ordinal, step.EventType, step.ToolName, string(payload))
	return err
}

func (r *PostgresRepository) CompleteRun(ctx context.Context, runID, output string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE agent_runs SET status = 'completed', output = $1, completed_at = NOW()
WHERE id = $2::uuid`, output, runID)
	return err
}

func (r *PostgresRepository) FailRun(ctx context.Context, runID, code, message string) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE agent_runs SET status = 'failed', failure_code = $1, failure_message = $2, completed_at = NOW()
WHERE id = $3::uuid`, code, message, runID)
	return err
}

func (r *PostgresRepository) GetRun(ctx context.Context, userID, id string) (Run, error) {
	var run Run
	err := r.db.QueryRowContext(ctx, `
SELECT id, user_id, agent_id, conversation_id, model_id, status, input, output,
       failure_code, failure_message, created_at, completed_at
FROM agent_runs WHERE id = $1::uuid AND user_id = $2::uuid`, id, userID).Scan(
		&run.ID, &run.UserID, &run.AgentID, &run.ConversationID, &run.ModelID,
		&run.Status, &run.Input, &run.Output, &run.FailureCode, &run.FailureMessage,
		&run.CreatedAt, &run.CompletedAt,
	)
	if err != nil {
		return Run{}, err
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT ordinal, event_type, tool_name, payload, created_at
FROM agent_run_steps WHERE run_id = $1::uuid ORDER BY ordinal`, id)
	if err != nil {
		return Run{}, err
	}
	defer rows.Close()
	run.Steps = []RunStep{}
	for rows.Next() {
		var step RunStep
		var payload []byte
		if err := rows.Scan(&step.Ordinal, &step.EventType, &step.ToolName, &payload, &step.CreatedAt); err != nil {
			return Run{}, err
		}
		if err := json.Unmarshal(payload, &step.Payload); err != nil {
			return Run{}, err
		}
		run.Steps = append(run.Steps, step)
	}
	if err := rows.Err(); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Run{}, err
	}
	return run, nil
}
