package score

import (
	"context"
	"fmt"
	"hash/fnv"
	"strings"
)

// Provider returns a quality score for an agent type (higher is better). Errors trigger least-latency fallback.
type Provider interface {
	Score(ctx context.Context, agentType string) (float64, error)
	Name() string
}

// Noop always returns a neutral score (50).
type Noop struct{}

func (Noop) Name() string { return "noop" }
func (Noop) Score(context.Context, string) (float64, error) { return 50, nil }

// Fixture returns deterministic demo scores per agent type (stable across processes).
type Fixture struct{}

func (Fixture) Name() string { return "fixture" }

func (Fixture) Score(_ context.Context, agentType string) (float64, error) {
	agentType = strings.TrimSpace(strings.ToLower(agentType))
	if agentType == "" {
		return 50, nil
	}
	// Curated defaults for common adapter types; others hash into 40–90.
	switch agentType {
	case "claude_code":
		return 88, nil
	case "cursor":
		return 82, nil
	case "github_copilot":
		return 76, nil
	case "aider":
		return 74, nil
	case "devin":
		return 70, nil
	case "custom":
		return 60, nil
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(agentType))
	return 40 + float64(h.Sum32()%51), nil
}

// Adapter wraps a function as a Provider (tests).
type Adapter struct {
	ProviderName string
	Fn           func(agentType string) (float64, error)
}

func (a Adapter) Name() string {
	if a.ProviderName != "" {
		return a.ProviderName
	}
	return "adapter"
}

func (a Adapter) Score(_ context.Context, agentType string) (float64, error) {
	if a.Fn == nil {
		return 50, nil
	}
	return a.Fn(agentType)
}

// ErrProvider always errors (CI fallback tests).
type ErrProvider struct{ Msg string }

func (e ErrProvider) Name() string { return "error" }
func (e ErrProvider) Score(context.Context, string) (float64, error) {
	msg := e.Msg
	if msg == "" {
		msg = "score provider unavailable"
	}
	return 0, fmt.Errorf("%s", msg)
}

// ResolveProvider picks noop|fixture|guardloop from SCORE_PROVIDER / GuardLoop flags.
func ResolveProviderName(explicit string, guardloopEnabled bool) string {
	name := strings.ToLower(strings.TrimSpace(explicit))
	switch name {
	case "noop", "fixture", "guardloop":
		return name
	case "auto", "":
		if guardloopEnabled {
			return "guardloop"
		}
		return "fixture"
	default:
		return "fixture"
	}
}
