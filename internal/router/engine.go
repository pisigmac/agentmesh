package router

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/agentmesh/agentmesh/internal/registry"
	"github.com/agentmesh/agentmesh/internal/score"
)

type Strategy string

const (
	StrategyLeastLatency    Strategy = "least-latency"
	StrategyLeastCost       Strategy = "least-cost"
	StrategyRoundRobin      Strategy = "round-robin"
	StrategyCapabilityMatch Strategy = "capability-match"
	StrategyScoreBased      Strategy = "score-based"
)

type ScoreProvider = score.Provider

type NoopScoreProvider struct{}

func (NoopScoreProvider) Score(_ context.Context, _ string) float64 { return 50 }

type Engine struct {
	Scores ScoreProvider
	rr     map[string]int
}

func NewEngine(scores ScoreProvider) *Engine {
	if scores == nil {
		scores = NoopScoreProvider{}
	}
	return &Engine{Scores: scores, rr: map[string]int{}}
}

// FilterStats counts hard SLO drops during candidate filtering.
type FilterStats struct {
	RejectedCost    int
	RejectedLatency int
	Kept            int
}

type RankInput struct {
	OrgID        string
	Capability   string
	MaxCost      float64
	MaxLatencyMs float64
	PreferFree   bool
	PreferLocal  bool
	Strategy     Strategy
	Agents       []registry.AgentRecord
	Stats        *FilterStats // optional; filled by Rank/filter
}

type Ranked struct {
	Agent registry.AgentRecord
	Score float64
}

func (e *Engine) Rank(in RankInput) []Ranked {
	candidates := filter(in)
	if len(candidates) == 0 {
		return nil
	}
	strategy := in.Strategy
	if strategy == "" {
		strategy = StrategyCapabilityMatch
	}
	var ranked []Ranked
	switch strategy {
	case StrategyLeastCost:
		ranked = rankBy(candidates, func(a registry.AgentRecord) float64 {
			return -costScore(a)
		})
	case StrategyLeastLatency:
		ranked = rankBy(candidates, func(a registry.AgentRecord) float64 {
			return -latencyScore(a)
		})
	case StrategyRoundRobin:
		key := in.OrgID + ":" + string(in.Capability)
		idx := e.rr[key] % len(candidates)
		e.rr[key]++
		rotated := append(candidates[idx:], candidates[:idx]...)
		out := make([]Ranked, len(rotated))
		for i, a := range rotated {
			out[i] = Ranked{Agent: a, Score: float64(len(rotated) - i)}
		}
		ranked = out
	case StrategyScoreBased:
		ranked = rankBy(candidates, func(a registry.AgentRecord) float64 {
			gl := e.Scores.Score(context.Background(), a.AgentType)
			return gl*0.4 + a.SuccessRate*30 + (1/latencyScore(a))*20 - costScore(a)*10
		})
	default:
		ranked = rankBy(candidates, func(a registry.AgentRecord) float64 {
			return a.SuccessRate*40 + (1/math.Max(latencyScore(a), 1))*30 - costScore(a)*20
		})
	}
	return PreferSLO(ranked, in.PreferFree, in.PreferLocal)
}

func filter(in RankInput) []registry.AgentRecord {
	var out []registry.AgentRecord
	stats := in.Stats
	for _, a := range in.Agents {
		if !a.Healthy {
			continue
		}
		if in.Capability != "" && !registry.HasCapability(a, in.Capability) {
			continue
		}
		if in.MaxCost > 0 && costScore(a) > in.MaxCost {
			if stats != nil {
				stats.RejectedCost++
			}
			continue
		}
		if in.MaxLatencyMs > 0 && latencyScore(a) > in.MaxLatencyMs {
			if stats != nil {
				stats.RejectedLatency++
			}
			continue
		}
		out = append(out, a)
	}
	if stats != nil {
		stats.Kept = len(out)
	}
	return out
}

func rankBy(agents []registry.AgentRecord, fn func(registry.AgentRecord) float64) []Ranked {
	out := make([]Ranked, len(agents))
	for i, a := range agents {
		out[i] = Ranked{Agent: a, Score: fn(a)}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func costScore(a registry.AgentRecord) float64 {
	if a.CostPerReq > 0 {
		return a.CostPerReq
	}
	return a.CostPer1K / 1000
}

func latencyScore(a registry.AgentRecord) float64 {
	if a.LatencyEWMA > 0 {
		return a.LatencyEWMA
	}
	return 100
}

// CostOf is exported for REST/discover post-filters.
func CostOf(a registry.AgentRecord) float64 { return costScore(a) }

// LatencyOf is exported for REST/discover post-filters.
func LatencyOf(a registry.AgentRecord) float64 { return latencyScore(a) }

func IsFreeAgent(a registry.AgentRecord) bool {
	if costScore(a) == 0 {
		return true
	}
	if a.Metadata != nil {
		if strings.EqualFold(a.Metadata["tier"], "free") || strings.EqualFold(a.Metadata["free"], "true") {
			return true
		}
	}
	return false
}

func IsLocalAgent(a registry.AgentRecord) bool {
	if a.Metadata != nil {
		if strings.EqualFold(a.Metadata["local"], "true") || strings.EqualFold(a.Metadata["deployment"], "local") {
			return true
		}
	}
	ep := strings.ToLower(a.EndpointURL)
	return strings.Contains(ep, "127.0.0.1") || strings.Contains(ep, "localhost") || strings.Contains(ep, "[::1]")
}

// PreferSLO soft-ranks free and/or local agents ahead of others.
func PreferSLO(ranked []Ranked, preferFree, preferLocal bool) []Ranked {
	if len(ranked) == 0 || (!preferFree && !preferLocal) {
		return ranked
	}
	score := func(r Ranked) int {
		n := 0
		if preferFree && IsFreeAgent(r.Agent) {
			n += 2
		}
		if preferLocal && IsLocalAgent(r.Agent) {
			n += 1
		}
		return n
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		return score(ranked[i]) > score(ranked[j])
	})
	return ranked
}

func FilterByFallbackTypes(ranked []Ranked, types []string) []Ranked {
	if len(types) == 0 {
		return ranked
	}
	allowed := map[string]struct{}{}
	for _, t := range types {
		allowed[t] = struct{}{}
	}
	var out []Ranked
	for _, r := range ranked {
		if _, ok := allowed[r.Agent.AgentType]; ok {
			out = append(out, r)
		}
	}
	if len(out) == 0 {
		return ranked
	}
	return out
}

// PreferAgentIDs moves matching eligible agents to the front of the ranked list
// (preserving relative order among preferred and among the rest).
func PreferAgentIDs(ranked []Ranked, prefer []string) []Ranked {
	if len(ranked) == 0 || len(prefer) == 0 {
		return ranked
	}
	order := make(map[string]int, len(prefer))
	for i, id := range prefer {
		if id == "" {
			continue
		}
		if _, ok := order[id]; !ok {
			order[id] = i
		}
	}
	if len(order) == 0 {
		return ranked
	}
	var front, rest []Ranked
	for _, r := range ranked {
		if _, ok := order[r.Agent.AgentID]; ok {
			front = append(front, r)
		} else {
			rest = append(rest, r)
		}
	}
	sort.SliceStable(front, func(i, j int) bool {
		return order[front[i].Agent.AgentID] < order[front[j].Agent.AgentID]
	})
	return append(front, rest...)
}

// FindAgent returns the agent with the given ID, or nil.
func FindAgent(agents []registry.AgentRecord, agentID string) *registry.AgentRecord {
	for i := range agents {
		if agents[i].AgentID == agentID {
			rec := agents[i]
			return &rec
		}
	}
	return nil
}

// MeetsSLOs reports whether an agent passes hard cost/latency ceilings.
func MeetsSLOs(a registry.AgentRecord, maxCost, maxLatencyMs float64) bool {
	if maxCost > 0 && costScore(a) > maxCost {
		return false
	}
	if maxLatencyMs > 0 && latencyScore(a) > maxLatencyMs {
		return false
	}
	return true
}
