package guardloop

import "context"

func (c *Client) ScoreCtx(ctx context.Context, agentType string) float64 {
	return c.Score(ctx, agentType)
}

type ScoreBridge struct{ *Client }

func (b ScoreBridge) Score(ctx context.Context, agentType string) float64 {
	return b.Client.Score(ctx, agentType)
}
