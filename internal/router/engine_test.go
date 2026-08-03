package router

import (
	"testing"

	"github.com/agentmesh/agentmesh/internal/registry"
)

func TestRankLeastLatency(t *testing.T) {
	e := NewEngine(NoopScoreProvider{})
	ranked := e.Rank(RankInput{
		OrgID:      "default",
		Capability: "code_gen",
		Strategy:   StrategyLeastLatency,
		Agents: []registry.AgentRecord{
			{AgentID: "a", Healthy: true, Capabilities: []string{"code_gen"}, LatencyEWMA: 200},
			{AgentID: "b", Healthy: true, Capabilities: []string{"code_gen"}, LatencyEWMA: 50},
		},
	})
	if len(ranked) != 2 || ranked[0].Agent.AgentID != "b" {
		t.Fatalf("expected b first, got %+v", ranked)
	}
}
