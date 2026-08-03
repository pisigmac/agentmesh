package auth

import (
	"context"
	"testing"
)

func TestAuthenticateDevKey(t *testing.T) {
	store := NewMemoryKeyStore("agentmesh-dev-key")
	p, err := Authenticate(context.Background(), store, "default", "", "Bearer agentmesh-dev-key")
	if err != nil {
		t.Fatal(err)
	}
	if p.OrgID != "default" {
		t.Fatalf("org %s", p.OrgID)
	}
}
