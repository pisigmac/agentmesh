package billing

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// StripeHooks posts usage meter events to Stripe (test or live).
// See ops/01-PAYMENTS.md. When SecretKey is empty or Enabled is false, RecordRoute is a no-op.
type StripeHooks struct {
	Enabled        bool
	SecretKey      string
	WebhookSecret  string
	MeterEventName string
	// OrgCustomers maps org_id → Stripe customer id (cus_…).
	OrgCustomers map[string]string
	HTTPClient   *http.Client
	APIBase      string // default https://api.stripe.com
}

func ParseOrgCustomers(raw string) map[string]string {
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		org, cus, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		org = strings.TrimSpace(org)
		cus = strings.TrimSpace(cus)
		if org != "" && cus != "" {
			out[org] = cus
		}
	}
	return out
}

func (s StripeHooks) customerFor(orgID string) string {
	if s.OrgCustomers == nil {
		return ""
	}
	return s.OrgCustomers[orgID]
}

// RecordRoute sends a Stripe Billing Meter Event when configured.
// identifier must be stable for idempotency (reuse on retries).
func (s StripeHooks) RecordRoute(ctx context.Context, orgID string, quantity float64, identifier string) error {
	if !s.Enabled || strings.TrimSpace(s.SecretKey) == "" {
		return nil
	}
	cus := s.customerFor(orgID)
	if cus == "" {
		// No customer mapping — skip Stripe API; CH still holds the event.
		return nil
	}
	eventName := s.MeterEventName
	if eventName == "" {
		eventName = "agentmesh_routes"
	}
	if quantity <= 0 {
		quantity = 1
	}
	if identifier == "" {
		identifier = fmt.Sprintf("route:%s:%d", orgID, time.Now().UnixNano())
	}
	base := s.APIBase
	if base == "" {
		base = "https://api.stripe.com"
	}
	form := url.Values{}
	form.Set("event_name", eventName)
	form.Set("payload[stripe_customer_id]", cus)
	form.Set("payload[value]", fmt.Sprintf("%g", quantity))
	form.Set("identifier", identifier)

	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/v1/billing/meter_events", strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(s.SecretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("stripe meter_events: %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
