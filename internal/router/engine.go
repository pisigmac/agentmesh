package router

import (
	"context"
	"math"
	"sort"

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

type RankInput struct {
	OrgID      string
	Capability string
	MaxCost    float64
	Strategy   Strategy
	Agents     []registry.AgentRecord
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
	switch strategy {
	case StrategyLeastCost:
		return rankBy(candidates, func(a registry.AgentRecord) float64 {
			return -costScore(a)
		})
	case StrategyLeastLatency:
		return rankBy(candidates, func(a registry.AgentRecord) float64 {
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
		return out
	case StrategyScoreBased:
		return rankBy(candidates, func(a registry.AgentRecord) float64 {
			gl := e.Scores.Score(context.Background(), a.AgentType)
			return gl*0.4 + a.SuccessRate*30 + (1/latencyScore(a))*20 - costScore(a)*10
		})
	default:
		return rankBy(candidates, func(a registry.AgentRecord) float64 {
			return a.SuccessRate*40 + (1 / math.Max(latencyScore(a), 1)) * 30 - costScore(a)*20
		})
	}
}

func filter(in RankInput) []registry.AgentRecord {
	var out []registry.AgentRecord
	for _, a := range in.Agents {
		if !a.Healthy {
			continue
		}
		if in.Capability != "" && !registry.HasCapability(a, in.Capability) {
			continue
		}
		if in.MaxCost > 0 && costScore(a) > in.MaxCost {
			continue
		}
		out = append(out, a)
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
