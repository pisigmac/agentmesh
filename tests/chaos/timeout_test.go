package chaos_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentmesh/agentmesh/internal/dataplane"
)

// Chaos: primary hangs past timeout; failover walks to secondary.
func TestChaosPrimaryTimeoutSecondarySucceeds(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(800 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"agent":"primary-late"}`))
	}))
	defer primary.Close()

	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"agent":"secondary"}`))
	}))
	defer secondary.Close()

	res, err := dataplane.ProxyWithFailover(
		context.Background(),
		http.MethodPost,
		[]byte(`{"q":"ping"}`),
		"application/json",
		[]dataplane.Candidate{
			{AgentID: "agent-primary", EndpointURL: primary.URL},
			{AgentID: "agent-secondary", EndpointURL: secondary.URL},
		},
		dataplane.ProxyConfig{
			Timeout:    100 * time.Millisecond,
			MaxRetries: 2,
			Failover:   true,
			PathSuffix: "/invoke",
			OrgID:      "chaos",
			// Use Timeout via reverse-proxy transport (HTTPClient bypasses ResponseHeaderTimeout).
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Status, string(res.Body))
	}
	if res.DestinationID != "agent-secondary" {
		t.Fatalf("destination=%s want agent-secondary", res.DestinationID)
	}
}
