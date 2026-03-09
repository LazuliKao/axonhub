package runner

import (
	"context"
	"fmt"

	"github.com/Khan/genqlient/graphql"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/looplj/axonhub/axon/agent"
	"github.com/looplj/axonhub/axon/thread"
	"github.com/looplj/axonhub/cmd/axonclaw/bootstrap"
)

type ResetTool struct {
	client    graphql.Client
	agent     *agent.Agent
	threadMgr *thread.Manager
	threadID  string
	workspace string
	boot      *bootstrap.Result
	logger    interface{ Info(msg string, args ...any) }
}

type ResetToolOptions struct {
	Client    graphql.Client
	Agent     *agent.Agent
	ThreadMgr *thread.Manager
	ThreadID  string
	Workspace string
	Boot      *bootstrap.Result
	Logger    interface{ Info(msg string, args ...any) }
}

func NewResetTool(opts ResetToolOptions) *ResetTool {
	return &ResetTool{
		client:    opts.Client,
		agent:     opts.Agent,
		threadMgr: opts.ThreadMgr,
		threadID:  opts.ThreadID,
		workspace: opts.Workspace,
		boot:      opts.Boot,
		logger:    opts.Logger,
	}
}

func (t *ResetTool) Definition() agent.ToolDefinition {
	return agent.ToolDefinition{
		Name:        "Reset",
		Description: "Refresh bootstrap configuration and reset the agent context. This clears in-memory messages and deletes the persisted thread history, without restarting the agent instance.",
		Parameters: jsonschema.Schema{
			Schema:               "https://json-schema.org/draft/2020-12/schema",
			Type:                 "object",
			AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
		},
	}
}

func (t *ResetTool) Execute(ctx context.Context, _ map[string]any) agent.ToolResult {
	newBoot, err := bootstrap.Do(ctx, t.client, bootstrap.SystemPromptData{
		Workspace:  t.workspace,
		SkillsRoot: t.boot.SkillsRoot,
		ConfigDir:  t.boot.ConfigDir,
	})
	if err != nil {
		return agent.ToolResult{Error: fmt.Errorf("reset bootstrap failed: %w", err)}
	}

	t.boot.AgentID = newBoot.AgentID
	t.boot.AgentName = newBoot.AgentName
	t.boot.Model = newBoot.Model
	t.boot.SystemPrompt = newBoot.SystemPrompt
	t.boot.Tools = newBoot.Tools
	t.boot.Skills = newBoot.Skills
	t.boot.BuiltinTools = newBoot.BuiltinTools
	t.boot.AxonClawPath = newBoot.AxonClawPath
	t.boot.Date = newBoot.Date
	t.boot.Timezone = newBoot.Timezone
	t.boot.OS = newBoot.OS

	env := buildPromptEnv(newBoot, t.workspace)
	serverPrompt := buildServerSystemPrompt(newBoot.SystemPrompt, env)
	serverPrompt = appendSkillsToPrompt(serverPrompt, newBoot.Skills)
	localPrompt := buildLocalSystemPrompt(env)

	t.agent.UpdateConfig(func(cfg agent.Config) agent.Config {
		cfg.SystemPrompts = []string{serverPrompt, localPrompt}
		return cfg
	})

	t.agent.ClearMessages()
	if t.threadMgr != nil && t.threadID != "" {
		if err := t.threadMgr.Delete(t.threadID); err != nil {
			if t.logger != nil {
				t.logger.Info("reset: failed to delete thread", "error", err)
			}
		}
	}

	result := fmt.Sprintf("Reset completed successfully.\n- Agent: %s (%s)\n- Model: %s\n- Thread cleared: %v",
		t.boot.AgentName, t.boot.AgentID, t.boot.Model, true)

	return agent.ToolResult{Content: agent.Content{Text: &result}}
}
