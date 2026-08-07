package policy

import (
	"testing"

	"github.com/agentmesh/agentmesh/internal/circuit"
	"github.com/agentmesh/agentmesh/internal/config"
)

func TestResolveCircuitPrecedence(t *testing.T) {
	p := DefaultOrgPolicy("acme", config.Config{
		Circuit: config.CircuitConfig{FailureThreshold: 5, WindowSeconds: 60, HalfOpenSeconds: 30},
	})
	p.Circuit.ByAgentType["cursor"] = circuit.Config{FailureThreshold: 3, WindowSeconds: 30, HalfOpenSeconds: 15}
	p.Circuit.ByDestinationAgent["agent-99"] = circuit.Config{FailureThreshold: 10, WindowSeconds: 120, HalfOpenSeconds: 60}

	c := ResolveCircuit(p, "agent-99", "cursor")
	if c.FailureThreshold != 10 {
		t.Fatalf("dest override: got %d", c.FailureThreshold)
	}
	c = ResolveCircuit(p, "agent-1", "cursor")
	if c.FailureThreshold != 3 {
		t.Fatalf("type override: got %d", c.FailureThreshold)
	}
	c = ResolveCircuit(p, "agent-1", "custom")
	if c.FailureThreshold != 5 {
		t.Fatalf("default: got %d", c.FailureThreshold)
	}
}

func TestMergeRoutingDefaults(t *testing.T) {
	base := DefaultOrgPolicy("o", config.Config{})
	overlay := OrgPolicy{Routing: RoutingPolicy{DefaultStrategy: "least-latency", DefaultTimeoutMs: 5000}}
	m := Merge(base, overlay)
	if m.Routing.DefaultStrategy != "least-latency" || m.Routing.DefaultTimeoutMs != 5000 {
		t.Fatalf("merge failed: %+v", m.Routing)
	}
}

func TestEffectiveMaxCostAndMergeSLOs(t *testing.T) {
	base := DefaultOrgPolicy("o", config.Config{})
	base.Routing.DefaultMaxCost = 0.02
	if base.Routing.EffectiveMaxCost() != 0.02 {
		t.Fatalf("legacy default_max_cost: %v", base.Routing.EffectiveMaxCost())
	}
	overlay := OrgPolicy{Routing: RoutingPolicy{
		MaxCostUSD: 0.01, MaxLatencyMs: 250, PreferFree: true, PreferLocal: true,
	}}
	m := Merge(base, overlay)
	if m.Routing.EffectiveMaxCost() != 0.01 {
		t.Fatalf("max_cost_usd wins: %v", m.Routing.EffectiveMaxCost())
	}
	if m.Routing.MaxLatencyMs != 250 || !m.Routing.PreferFree || !m.Routing.PreferLocal {
		t.Fatalf("SLO merge: %+v", m.Routing)
	}
}
