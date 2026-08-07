package dataplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	agentmeshv1 "github.com/agentmesh/agentmesh/gen/agentmesh/v1"
)

type memCircuit struct {
	failures map[string]int
	blocked  map[string]bool
}

func newMemCircuit() *memCircuit {
	return &memCircuit{failures: map[string]int{}, blocked: map[string]bool{}}
}

func (m *memCircuit) RecordFailure(_ context.Context, _, dest string) (int, error) {
	m.failures[dest]++
	return m.failures[dest], nil
}
func (m *memCircuit) RecordSuccess(_ context.Context, _, dest string) error {
	m.failures[dest] = 0
	return nil
}
func (m *memCircuit) AllowRequest(_ context.Context, _, dest string) (bool, error) {
	if m.blocked[dest] {
		return false, nil
	}
	return true, nil
}

func TestCandidatesFromRouteDedupes(t *testing.T) {
	resp := &agentmeshv1.RouteResponse{
		DestinationAgentId: "a1",
		EndpointUrl:        "http://primary",
		FallbackChain: []*agentmeshv1.AgentEndpoint{
			{AgentId: "a1", EndpointUrl: "http://primary"},
			{AgentId: "a2", EndpointUrl: "http://secondary"},
		},
	}
	c := CandidatesFromRoute(resp)
	if len(c) != 2 {
		t.Fatalf("want 2 candidates, got %d", len(c))
	}
	if c[0].AgentID != "a1" || c[1].AgentID != "a2" {
		t.Fatalf("unexpected order: %+v", c)
	}
}

func TestFailoverPrimaryDownSecondaryOK(t *testing.T) {
	var primaryHits, secondaryHits atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		primaryHits.Add(1)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("down"))
	}))
	defer primary.Close()
	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		secondaryHits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer secondary.Close()

	circ := newMemCircuit()
	client := &http.Client{Timeout: 2 * time.Second}
	res, err := ProxyWithFailover(context.Background(), http.MethodPost, []byte(`{}`), "application/json",
		[]Candidate{
			{AgentID: "primary", EndpointURL: primary.URL},
			{AgentID: "secondary", EndpointURL: secondary.URL},
		},
		ProxyConfig{
			Timeout:    2 * time.Second,
			MaxRetries: 2,
			Failover:   true,
			PathSuffix: "/invoke",
			OrgID:      "default",
			Circuit:    circ,
			HTTPClient: client,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != 200 {
		t.Fatalf("status=%d body=%s", res.Status, res.Body)
	}
	if res.DestinationID != "secondary" {
		t.Fatalf("dest=%s", res.DestinationID)
	}
	if primaryHits.Load() != 1 || secondaryHits.Load() != 1 {
		t.Fatalf("hits primary=%d secondary=%d", primaryHits.Load(), secondaryHits.Load())
	}
	if circ.failures["primary"] != 1 {
		t.Fatalf("primary failures=%d", circ.failures["primary"])
	}
	if circ.failures["secondary"] != 0 {
		t.Fatalf("secondary should succeed-clear failures, got %d", circ.failures["secondary"])
	}
}

func TestFailoverDisabledStaysOnPrimary(t *testing.T) {
	var primaryHits, secondaryHits atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		primaryHits.Add(1)
		w.WriteHeader(500)
	}))
	defer primary.Close()
	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		secondaryHits.Add(1)
		w.WriteHeader(200)
	}))
	defer secondary.Close()

	res, err := ProxyWithFailover(context.Background(), http.MethodPost, []byte(`{}`), "application/json",
		[]Candidate{
			{AgentID: "primary", EndpointURL: primary.URL},
			{AgentID: "secondary", EndpointURL: secondary.URL},
		},
		ProxyConfig{
			MaxRetries: 2,
			Failover:   false,
			HTTPClient: &http.Client{Timeout: 2 * time.Second},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status < 500 {
		t.Fatalf("expected primary failure status, got %d", res.Status)
	}
	if secondaryHits.Load() != 0 {
		t.Fatalf("secondary should not be hit when failover disabled")
	}
	if primaryHits.Load() != 3 { // MaxRetries=2 → 3 attempts
		t.Fatalf("primary hits=%d want 3", primaryHits.Load())
	}
}

func TestFailoverSkipsOpenCircuit(t *testing.T) {
	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer secondary.Close()
	circ := newMemCircuit()
	circ.blocked["primary"] = true

	res, err := ProxyWithFailover(context.Background(), http.MethodPost, nil, "",
		[]Candidate{
			{AgentID: "primary", EndpointURL: "http://127.0.0.1:1"},
			{AgentID: "secondary", EndpointURL: secondary.URL},
		},
		ProxyConfig{
			MaxRetries: 2,
			Failover:   true,
			Circuit:    circ,
			HTTPClient: &http.Client{Timeout: 2 * time.Second},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if res.DestinationID != "secondary" || res.Status != 200 {
		t.Fatalf("got dest=%s status=%d", res.DestinationID, res.Status)
	}
}
