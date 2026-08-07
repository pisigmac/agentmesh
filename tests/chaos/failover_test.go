package chaos_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentmesh/agentmesh/internal/dataplane"
)

// Chaos: primary returns 5xx; with DATAPLANE_FAILOVER semantics secondary succeeds.
func TestChaosPrimaryDownSecondarySucceeds(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("unavailable"))
	}))
	defer primary.Close()

	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/invoke" {
			t.Fatalf("path=%s", r.URL.Path)
		}
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
			Timeout:    time.Second,
			MaxRetries: 2,
			Failover:   true,
			PathSuffix: "/invoke",
			OrgID:      "chaos",
			HTTPClient: &http.Client{Timeout: 2 * time.Second},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != http.StatusOK {
		t.Fatalf("status=%d body=%s", res.Status, string(res.Body))
	}
	if res.DestinationID != "agent-secondary" {
		t.Fatalf("destination=%s", res.DestinationID)
	}
	if res.Attempts != 2 {
		t.Fatalf("attempts=%d want 2", res.Attempts)
	}
}
