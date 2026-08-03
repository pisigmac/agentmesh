package adapters

import (
	"context"
	"fmt"

	"github.com/agentmesh/agentmesh/pkg/agentmesh"
)

// Agent types aligned with GuardLoop registry.
const (
	TypeCursor        = "cursor"
	TypeClaudeCode    = "claude_code"
	TypeGitHubCopilot = "github_copilot"
	TypeOpenAICodex   = "openai_codex"
	TypeAider         = "aider"
	TypeContinueDev   = "continue_dev"
	TypeWindsurf      = "windsurf"
	TypeDevin         = "devin"
	TypeCustom        = "custom"
)

var AllTypes = []string{
	TypeCursor, TypeClaudeCode, TypeGitHubCopilot, TypeOpenAICodex,
	TypeAider, TypeContinueDev, TypeWindsurf, TypeDevin, TypeCustom,
}

type Manifest struct {
	AgentType    string
	Capabilities []string
	DefaultCost  agentmesh.CostProfile
}

var Manifests = map[string]Manifest{
	TypeCursor:        {TypeCursor, []string{"code_gen", "refactor", "browser_verify"}, agentmesh.CostProfile{0.02, 0.015}},
	TypeClaudeCode:    {TypeClaudeCode, []string{"code_gen", "refactor"}, agentmesh.CostProfile{0.025, 0.018}},
	TypeGitHubCopilot: {TypeGitHubCopilot, []string{"code_gen", "completion"}, agentmesh.CostProfile{0.01, 0.008}},
	TypeOpenAICodex:   {TypeOpenAICodex, []string{"code_gen"}, agentmesh.CostProfile{0.03, 0.02}},
	TypeAider:         {TypeAider, []string{"code_gen", "refactor"}, agentmesh.CostProfile{0.005, 0.004}},
	TypeContinueDev:   {TypeContinueDev, []string{"code_gen", "completion"}, agentmesh.CostProfile{0.008, 0.006}},
	TypeWindsurf:      {TypeWindsurf, []string{"code_gen", "refactor"}, agentmesh.CostProfile{0.015, 0.012}},
	TypeDevin:         {TypeDevin, []string{"code_gen", "browser_verify", "refactor"}, agentmesh.CostProfile{0.05, 0.04}},
	TypeCustom:        {TypeCustom, []string{"custom"}, agentmesh.CostProfile{0.01, 0.01}},
}

func Register(ctx context.Context, client *agentmesh.Client, agentType, agentID, endpointURL string, extraCaps []string, persistent bool) error {
	m, ok := Manifests[agentType]
	if !ok {
		return fmt.Errorf("unknown agent type: %s", agentType)
	}
	caps := append(append([]string{}, m.Capabilities...), extraCaps...)
	reg := agentmesh.Registration{
		AgentID:      agentID,
		AgentType:    agentType,
		Capabilities: caps,
		EndpointURL:  endpointURL,
		Health:       &agentmesh.Health{AgentID: agentID, Healthy: true},
		CostProfile:  &m.DefaultCost,
	}
	if persistent {
		reg.Metadata = map[string]string{"persistent": "true"}
	}
	return client.Register(ctx, reg)
}
