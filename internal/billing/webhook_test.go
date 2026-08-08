package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestVerifyStripeSignature(t *testing.T) {
	secret := "whsec_test"
	payload := []byte(`{"id":"evt_1","type":"invoice.paid"}`)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ts + "." + string(payload)))
	sig := hex.EncodeToString(mac.Sum(nil))
	header := "t=" + ts + ",v1=" + sig
	if err := VerifyStripeSignature(payload, header, secret, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := VerifyStripeSignature(payload, header, "wrong", time.Minute); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestApplyStripeEventPaymentFailed(t *testing.T) {
	obj, _ := json.Marshal(map[string]any{
		"customer": "cus_x",
		"metadata": map[string]string{"agentmesh_org_id": "acme"},
	})
	ev := StripeEvent{Type: "invoice.payment_failed"}
	ev.Data.Object = obj
	org, st, _, err := ApplyStripeEvent(ev, nil)
	if err != nil {
		t.Fatal(err)
	}
	if org != "acme" || st != BillingPastDue {
		t.Fatalf("org=%s status=%s", org, st)
	}
}

func TestEnforceRoute(t *testing.T) {
	if msg, _ := EnforceRoute(OrgBillingStatus{Status: BillingBlocked}); msg == "" {
		t.Fatal("expected block")
	}
	if msg, warn := EnforceRoute(OrgBillingStatus{Status: BillingPastDue}); msg != "" || warn == "" {
		t.Fatalf("past_due soft: msg=%q warn=%q", msg, warn)
	}
	if msg, warn := EnforceRoute(OrgBillingStatus{Status: BillingOK}); msg != "" || warn != "" {
		t.Fatal("ok should allow")
	}
}

func TestResolveOrgFromMap(t *testing.T) {
	org := ResolveOrgFromStripeObject("cus_1", nil, map[string]string{"beta": "cus_1"})
	if org != "beta" {
		t.Fatalf("got %s", org)
	}
}
