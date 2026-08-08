package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// StripeEvent is a minimal Stripe webhook event payload.
type StripeEvent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Data struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type stripeCustomer struct {
	ID       string            `json:"id"`
	Metadata map[string]string `json:"metadata"`
}

type stripeInvoice struct {
	Customer string `json:"customer"`
	Metadata map[string]string `json:"metadata"`
}

type stripeSubscription struct {
	Customer string            `json:"customer"`
	Status   string            `json:"status"`
	Metadata map[string]string `json:"metadata"`
}

// VerifyStripeSignature validates Stripe-Signature header (t=…,v1=…).
// tolerance is max age of the timestamp (0 → 5 minutes).
func VerifyStripeSignature(payload []byte, header, secret string, tolerance time.Duration) error {
	if secret == "" {
		return fmt.Errorf("webhook secret not configured")
	}
	if header == "" {
		return fmt.Errorf("missing Stripe-Signature")
	}
	if tolerance <= 0 {
		tolerance = 5 * time.Minute
	}
	var ts string
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			ts = v
		case "v1":
			sigs = append(sigs, v)
		}
	}
	if ts == "" || len(sigs) == 0 {
		return fmt.Errorf("invalid Stripe-Signature")
	}
	sec, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid signature timestamp")
	}
	if time.Since(time.Unix(sec, 0)) > tolerance || time.Until(time.Unix(sec, 0)) > tolerance {
		return fmt.Errorf("signature timestamp outside tolerance")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ts + "." + string(payload)))
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, s := range sigs {
		if hmac.Equal([]byte(expected), []byte(s)) {
			return nil
		}
	}
	return fmt.Errorf("signature mismatch")
}

// ResolveOrgFromStripeObject maps a Stripe customer/metadata to an AgentMesh org_id.
func ResolveOrgFromStripeObject(customerID string, metadata map[string]string, orgCustomers map[string]string) string {
	if metadata != nil {
		if o := strings.TrimSpace(metadata["agentmesh_org_id"]); o != "" {
			return o
		}
	}
	for org, cus := range orgCustomers {
		if cus == customerID {
			return org
		}
	}
	return ""
}

// ApplyStripeEvent updates org billing status from a verified Stripe event.
// Returns orgID and new status (empty org if unmapped / ignored).
func ApplyStripeEvent(ev StripeEvent, orgCustomers map[string]string) (orgID, status, reason string, err error) {
	switch ev.Type {
	case "invoice.payment_failed":
		var inv stripeInvoice
		if err := json.Unmarshal(ev.Data.Object, &inv); err != nil {
			return "", "", "", err
		}
		orgID = ResolveOrgFromStripeObject(inv.Customer, inv.Metadata, orgCustomers)
		return orgID, BillingPastDue, "invoice.payment_failed", nil
	case "invoice.paid", "invoice.payment_succeeded":
		var inv stripeInvoice
		if err := json.Unmarshal(ev.Data.Object, &inv); err != nil {
			return "", "", "", err
		}
		orgID = ResolveOrgFromStripeObject(inv.Customer, inv.Metadata, orgCustomers)
		return orgID, BillingOK, ev.Type, nil
	case "customer.subscription.deleted":
		var sub stripeSubscription
		if err := json.Unmarshal(ev.Data.Object, &sub); err != nil {
			return "", "", "", err
		}
		orgID = ResolveOrgFromStripeObject(sub.Customer, sub.Metadata, orgCustomers)
		return orgID, BillingBlocked, "subscription.deleted", nil
	case "customer.subscription.updated":
		var sub stripeSubscription
		if err := json.Unmarshal(ev.Data.Object, &sub); err != nil {
			return "", "", "", err
		}
		orgID = ResolveOrgFromStripeObject(sub.Customer, sub.Metadata, orgCustomers)
		switch strings.ToLower(sub.Status) {
		case "active", "trialing":
			return orgID, BillingOK, "subscription." + sub.Status, nil
		case "past_due", "unpaid":
			return orgID, BillingPastDue, "subscription." + sub.Status, nil
		case "canceled", "cancelled", "incomplete_expired":
			return orgID, BillingBlocked, "subscription." + sub.Status, nil
		default:
			return orgID, "", "", nil
		}
	default:
		return "", "", "", nil
	}
}
