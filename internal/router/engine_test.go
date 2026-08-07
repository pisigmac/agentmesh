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

func TestPreferAgentIDs(t *testing.T) {
	ranked := []Ranked{
		{Agent: registry.AgentRecord{AgentID: "a"}},
		{Agent: registry.AgentRecord{AgentID: "b"}},
		{Agent: registry.AgentRecord{AgentID: "c"}},
	}
	out := PreferAgentIDs(ranked, []string{"c", "a"})
	if len(out) != 3 || out[0].Agent.AgentID != "c" || out[1].Agent.AgentID != "a" || out[2].Agent.AgentID != "b" {
		t.Fatalf("unexpected order: %+v", out)
	}
}

func TestFindAgent(t *testing.T) {
	agents := []registry.AgentRecord{{AgentID: "x"}, {AgentID: "y"}}
	if FindAgent(agents, "missing") != nil {
		t.Fatal("expected nil")
	}
	got := FindAgent(agents, "y")
	if got == nil || got.AgentID != "y" {
		t.Fatalf("got %+v", got)
	}
}
