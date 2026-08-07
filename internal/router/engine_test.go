package router

import (
	"testing"

	"github.com/agentmesh/agentmesh/internal/registry"
	"github.com/agentmesh/agentmesh/internal/score"
)

func TestRankLeastLatency(t *testing.T) {
	e := NewEngine(score.Noop{})
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

func TestRankFiltersMaxCostAndLatency(t *testing.T) {
	e := NewEngine(score.Noop{})
	var stats FilterStats
	ranked := e.Rank(RankInput{
		Capability:   "code_gen",
		MaxCost:      0.01,
		MaxLatencyMs: 100,
		Strategy:     StrategyLeastCost,
		Stats:        &stats,
		Agents: []registry.AgentRecord{
			{AgentID: "cheap-fast", Healthy: true, Capabilities: []string{"code_gen"}, CostPerReq: 0.005, LatencyEWMA: 40},
			{AgentID: "expensive", Healthy: true, Capabilities: []string{"code_gen"}, CostPerReq: 0.05, LatencyEWMA: 40},
			{AgentID: "slow", Healthy: true, Capabilities: []string{"code_gen"}, CostPerReq: 0.001, LatencyEWMA: 500},
		},
	})
	if len(ranked) != 1 || ranked[0].Agent.AgentID != "cheap-fast" {
		t.Fatalf("ranked=%+v stats=%+v", ranked, stats)
	}
	if stats.RejectedCost != 1 || stats.RejectedLatency != 1 {
		t.Fatalf("stats=%+v", stats)
	}
}

func TestScoreBasedFixtureOrdersByProviderScore(t *testing.T) {
	e := NewEngine(score.Fixture{})
	var meta RankMeta
	ranked := e.Rank(RankInput{
		Capability: "code_gen",
		Strategy:   StrategyScoreBased,
		Meta:       &meta,
		Agents: []registry.AgentRecord{
			{AgentID: "copilot", AgentType: "github_copilot", Healthy: true, Capabilities: []string{"code_gen"}, SuccessRate: 0.9, LatencyEWMA: 80},
			{AgentID: "claude", AgentType: "claude_code", Healthy: true, Capabilities: []string{"code_gen"}, SuccessRate: 0.9, LatencyEWMA: 80},
		},
	})
	if len(ranked) < 2 || ranked[0].Agent.AgentID != "claude" {
		t.Fatalf("want claude first (higher fixture score), got %+v", ranked)
	}
	if meta.Provider != "fixture" || meta.Fallback != "" || len(meta.Contributions) != 2 {
		t.Fatalf("meta=%+v", meta)
	}
}

func TestScoreBasedFallsBackToLeastLatency(t *testing.T) {
	e := NewEngine(score.ErrProvider{Msg: "boom"})
	var meta RankMeta
	ranked := e.Rank(RankInput{
		Capability: "code_gen",
		Strategy:   StrategyScoreBased,
		Meta:       &meta,
		Agents: []registry.AgentRecord{
			{AgentID: "slow", AgentType: "cursor", Healthy: true, Capabilities: []string{"code_gen"}, LatencyEWMA: 200},
			{AgentID: "fast", AgentType: "cursor", Healthy: true, Capabilities: []string{"code_gen"}, LatencyEWMA: 40},
		},
	})
	if len(ranked) < 1 || ranked[0].Agent.AgentID != "fast" {
		t.Fatalf("fallback least-latency want fast, got %+v", ranked)
	}
	if meta.Fallback != string(StrategyLeastLatency) {
		t.Fatalf("meta=%+v", meta)
	}
}

func TestPreferSLOFreeAndLocal(t *testing.T) {
	ranked := []Ranked{
		{Agent: registry.AgentRecord{AgentID: "cloud", CostPerReq: 0.02, EndpointURL: "https://api.example.com"}},
		{Agent: registry.AgentRecord{AgentID: "local", CostPerReq: 0.02, EndpointURL: "http://127.0.0.1:9000"}},
		{Agent: registry.AgentRecord{AgentID: "free", CostPerReq: 0, EndpointURL: "https://api.example.com"}},
	}
	out := PreferSLO(ranked, true, true)
	if out[0].Agent.AgentID != "free" {
		t.Fatalf("want free first, got %s", out[0].Agent.AgentID)
	}
	if out[1].Agent.AgentID != "local" {
		t.Fatalf("want local second, got %s", out[1].Agent.AgentID)
	}
}
