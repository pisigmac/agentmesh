package dataplane

import (
	"context"

	"github.com/agentmesh/agentmesh/internal/circuit"
)

// BreakerAdapter adapts circuit.Breaker to CircuitRecorder.
type BreakerAdapter struct {
	B *circuit.Breaker
}

func (a BreakerAdapter) RecordFailure(ctx context.Context, orgID, destAgentID string) (int, error) {
	if a.B == nil {
		return 0, nil
	}
	return a.B.RecordFailure(ctx, orgID, destAgentID)
}

func (a BreakerAdapter) RecordSuccess(ctx context.Context, orgID, destAgentID string) error {
	if a.B == nil {
		return nil
	}
	return a.B.RecordSuccess(ctx, orgID, destAgentID)
}

func (a BreakerAdapter) AllowRequest(ctx context.Context, orgID, destAgentID string) (bool, error) {
	if a.B == nil {
		return true, nil
	}
	ok, _, err := a.B.AllowRequest(ctx, orgID, destAgentID)
	return ok, err
}
