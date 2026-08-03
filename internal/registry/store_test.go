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
