package billing

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseOrgCustomers(t *testing.T) {
	m := ParseOrgCustomers("acme:cus_1, beta:cus_2")
	if m["acme"] != "cus_1" || m["beta"] != "cus_2" {
		t.Fatalf("%v", m)
	}
}

func TestRecordRoutePostsMeterEvent(t *testing.T) {
	var gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"mevt_1"}`))
	}))
	defer srv.Close()

	h := StripeHooks{
		Enabled:        true,
		SecretKey:      "sk_test_x",
		MeterEventName: "agentmesh_routes",
		OrgCustomers:   map[string]string{"acme": "cus_123"},
		HTTPClient:     srv.Client(),
		APIBase:        srv.URL,
	}
	if err := h.RecordRoute(context.Background(), "acme", 1, "route:trace-1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gotAuth, "Basic") {
		t.Fatalf("auth=%s", gotAuth)
	}
	if !strings.Contains(gotBody, "event_name=agentmesh_routes") || !strings.Contains(gotBody, "cus_123") {
		t.Fatalf("body=%s", gotBody)
	}
	if !strings.Contains(gotBody, "identifier=route%3Atrace-1") && !strings.Contains(gotBody, "identifier=route:trace-1") {
		t.Fatalf("idempotency missing: %s", gotBody)
	}
}

func TestRecordRouteNoopWithoutKey(t *testing.T) {
	h := StripeHooks{Enabled: true}
	if err := h.RecordRoute(context.Background(), "acme", 1, "x"); err != nil {
		t.Fatal(err)
	}
}
