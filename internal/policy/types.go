package policy

import (
	"github.com/agentmesh/agentmesh/internal/circuit"
	"github.com/agentmesh/agentmesh/internal/config"
	"github.com/agentmesh/agentmesh/internal/router"
)

// OrgPolicy is the config-driven policy document stored in etcd per org.
type OrgPolicy struct {
	OrgID    string                 `json:"org_id"`
	Routing  RoutingPolicy          `json:"routing"`
	RateLimit RateLimitPolicy       `json:"rate_limit"`
	Circuit  CircuitPolicy          `json:"circuit"`
	Metadata map[string]string      `json:"metadata,omitempty"`
}

type RoutingPolicy struct {
	DefaultStrategy   string  `json:"default_strategy"`
	DefaultMaxCost    float64 `json:"default_max_cost,omitempty"` // legacy alias for max_cost_usd
	MaxCostUSD        float64 `json:"max_cost_usd,omitempty"`
	MaxLatencyMs      float64 `json:"max_latency_ms,omitempty"`
	PreferFree        bool    `json:"prefer_free,omitempty"`
	PreferLocal       bool    `json:"prefer_local,omitempty"`
	DefaultTimeoutMs  int32   `json:"default_timeout_ms"`
	DefaultMaxRetries int32   `json:"default_max_retries"`
}

// EffectiveMaxCost returns max_cost_usd, falling back to default_max_cost.
func (r RoutingPolicy) EffectiveMaxCost() float64 {
	if r.MaxCostUSD > 0 {
		return r.MaxCostUSD
	}
	return r.DefaultMaxCost
}

type RateLimitPolicy struct {
	RoutesPerMinute           int `json:"routes_per_minute"`
	PerAgentRoutesPerMinute   int `json:"per_agent_routes_per_minute"`
	RegisterPerMinute         int `json:"register_per_minute"`
}

type CircuitPolicy struct {
	Default            circuit.Config            `json:"default"`
	ByAgentType        map[string]circuit.Config `json:"by_agent_type,omitempty"`
	ByDestinationAgent map[string]circuit.Config `json:"by_destination_agent,omitempty"`
}

func DefaultOrgPolicy(orgID string, cfg config.Config) OrgPolicy {
	return OrgPolicy{
		OrgID: orgID,
		Routing: RoutingPolicy{
			DefaultStrategy:   string(router.StrategyCapabilityMatch),
			DefaultTimeoutMs:  30000,
			DefaultMaxRetries: 2,
		},
		RateLimit: RateLimitPolicy{
			RoutesPerMinute:         1000,
			PerAgentRoutesPerMinute: 200,
			RegisterPerMinute:       60,
		},
		Circuit: CircuitPolicy{
			Default: circuit.Config{
				FailureThreshold: cfg.Circuit.FailureThreshold,
				WindowSeconds:    cfg.Circuit.WindowSeconds,
				HalfOpenSeconds:  cfg.Circuit.HalfOpenSeconds,
			},
			ByAgentType:        map[string]circuit.Config{},
			ByDestinationAgent: map[string]circuit.Config{},
		},
	}
}

func Merge(base, overlay OrgPolicy) OrgPolicy {
	out := base
	if overlay.Routing.DefaultStrategy != "" {
		out.Routing.DefaultStrategy = overlay.Routing.DefaultStrategy
	}
	if overlay.Routing.DefaultMaxCost > 0 {
		out.Routing.DefaultMaxCost = overlay.Routing.DefaultMaxCost
	}
	if overlay.Routing.MaxCostUSD > 0 {
		out.Routing.MaxCostUSD = overlay.Routing.MaxCostUSD
	}
	if overlay.Routing.MaxLatencyMs > 0 {
		out.Routing.MaxLatencyMs = overlay.Routing.MaxLatencyMs
	}
	if overlay.Routing.PreferFree {
		out.Routing.PreferFree = true
	}
	if overlay.Routing.PreferLocal {
		out.Routing.PreferLocal = true
	}
	if overlay.Routing.DefaultTimeoutMs > 0 {
		out.Routing.DefaultTimeoutMs = overlay.Routing.DefaultTimeoutMs
	}
	if overlay.Routing.DefaultMaxRetries > 0 {
		out.Routing.DefaultMaxRetries = overlay.Routing.DefaultMaxRetries
	}
	if overlay.RateLimit.RoutesPerMinute > 0 {
		out.RateLimit.RoutesPerMinute = overlay.RateLimit.RoutesPerMinute
	}
	if overlay.RateLimit.PerAgentRoutesPerMinute > 0 {
		out.RateLimit.PerAgentRoutesPerMinute = overlay.RateLimit.PerAgentRoutesPerMinute
	}
	if overlay.RateLimit.RegisterPerMinute > 0 {
		out.RateLimit.RegisterPerMinute = overlay.RateLimit.RegisterPerMinute
	}
	if overlay.Circuit.Default.FailureThreshold > 0 {
		out.Circuit.Default = overlay.Circuit.Default
	}
	for k, v := range overlay.Circuit.ByAgentType {
		if out.Circuit.ByAgentType == nil {
			out.Circuit.ByAgentType = map[string]circuit.Config{}
		}
		out.Circuit.ByAgentType[k] = v
	}
	for k, v := range overlay.Circuit.ByDestinationAgent {
		if out.Circuit.ByDestinationAgent == nil {
			out.Circuit.ByDestinationAgent = map[string]circuit.Config{}
		}
		out.Circuit.ByDestinationAgent[k] = v
	}
	return out
}

func ResolveCircuit(p OrgPolicy, destAgentID, agentType string) circuit.Config {
	if c, ok := p.Circuit.ByDestinationAgent[destAgentID]; ok && c.FailureThreshold > 0 {
		return c
	}
	if c, ok := p.Circuit.ByAgentType[agentType]; ok && c.FailureThreshold > 0 {
		return c
	}
	return p.Circuit.Default
}
