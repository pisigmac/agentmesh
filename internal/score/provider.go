package score

import "context"

type Provider interface {
	Score(ctx context.Context, agentType string) float64
}

type Adapter struct {
	Fn func(agentType string) float64
}

func (a Adapter) Score(_ context.Context, agentType string) float64 {
	if a.Fn == nil {
		return 50
	}
	return a.Fn(agentType)
}
