package registry

import "testing"

func TestIsPersistentRegistration(t *testing.T) {
	if IsPersistentRegistration(AgentRecord{Metadata: map[string]string{"persistent": "true"}}) != true {
		t.Fatal("expected persistent")
	}
	if IsPersistentRegistration(AgentRecord{}) {
		t.Fatal("expected ephemeral")
	}
}

func TestShouldAggregateDefault(t *testing.T) {
	if !ShouldAggregateDefault(DefaultOrgID, true) {
		t.Fatal("default+enabled should aggregate")
	}
	if ShouldAggregateDefault(DefaultOrgID, false) {
		t.Fatal("default+disabled must not aggregate (E21)")
	}
	if ShouldAggregateDefault("acme", true) {
		t.Fatal("non-default org must never aggregate")
	}
	if ShouldAggregateDefault("acme", false) {
		t.Fatal("non-default org must never aggregate")
	}
}

func TestApplyLatencyEWMA(t *testing.T) {
	if got := ApplyLatencyEWMA(0, 100, 0.3); got != 100 {
		t.Fatalf("cold start got %v", got)
	}
	got := ApplyLatencyEWMA(100, 40, 0.3)
	want := 0.3*40 + 0.7*100
	if got != want {
		t.Fatalf("got %v want %v", got, want)
	}
}
