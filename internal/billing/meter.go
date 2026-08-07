package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/agentmesh/agentmesh/internal/analytics"
)

// Meter records route usage in ClickHouse and optionally Stripe.
type Meter struct {
	CH     *analytics.Store
	Stripe StripeHooks
}

// RecordRouteUsage inserts billing_events and posts Stripe when configured.
// Idempotency key: route:{traceID} (or generated).
func (m *Meter) RecordRouteUsage(ctx context.Context, orgID, traceID string, quantity float64) error {
	if m == nil {
		return nil
	}
	if quantity <= 0 {
		quantity = 1
	}
	id := "route:" + traceID
	if traceID == "" {
		id = fmt.Sprintf("route:%s:%d", orgID, time.Now().UTC().UnixNano())
	}
	if m.CH != nil {
		if err := m.CH.InsertBillingEvent(ctx, analytics.BillingEvent{
			OrgID:         orgID,
			EventType:     "route",
			Quantity:      quantity,
			StripeEventID: id,
		}); err != nil {
			return err
		}
	}
	return m.Stripe.RecordRoute(ctx, orgID, quantity, id)
}
