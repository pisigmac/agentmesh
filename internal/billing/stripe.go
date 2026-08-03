package billing

import "context"

// StripeHooks documents usage-based billing integration (see ops/payments.md).
type StripeHooks struct {
	SecretKey       string
	WebhookSecret   string
	MeterEventName  string
}

func (s StripeHooks) RecordRoute(ctx context.Context, orgID string, quantity float64) error {
	_ = ctx
	_ = orgID
	_ = quantity
	return nil
}
