package auth

import (
	"context"
	"testing"
	"time"
)

func TestAuthenticateDevKeyMemory(t *testing.T) {
	store := NewMemoryKeyStore("agentmesh-dev-key")
	p, err := Authenticate(context.Background(), store, "default", "", "Bearer agentmesh-dev-key")
	if err != nil {
		t.Fatal(err)
	}
	if p.OrgID != "default" {
		t.Fatalf("org %s", p.OrgID)
	}
}

func TestRoleRank(t *testing.T) {
	if RoleViewer.Rank() >= RoleOperator.Rank() {
		t.Fatal("viewer should be below operator")
	}
	p := Principal{Role: RoleViewer}
	if RequireRole(p, RoleOperator) == nil {
		t.Fatal("expected forbidden")
	}
	p.Role = RoleAdmin
	if err := RequireRole(p, RoleOperator); err != nil {
		t.Fatal(err)
	}
}

func TestPasswordAndJWT(t *testing.T) {
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPassword(hash, "password123") {
		t.Fatal("password mismatch")
	}
	issuer, err := NewTokenIssuer("01234567890123456789012345678901", 1)
	if err != nil {
		t.Fatal(err)
	}
	tok, jti, exp, err := issuer.Issue("user-1", "a@b.com")
	if err != nil {
		t.Fatal(err)
	}
	if jti == "" || tok == "" || time.Until(exp) < time.Minute {
		t.Fatalf("bad token issue")
	}
	claims, err := issuer.Parse(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" || claims.Email != "a@b.com" {
		t.Fatalf("claims %+v", claims)
	}
}

func TestAPIKeyPlaintextPrefix(t *testing.T) {
	plain, id, err := NewAPIKeyPlaintext()
	if err != nil {
		t.Fatal(err)
	}
	if len(id) == 0 || len(plain) < 10 || plain[:4] != "amk_" {
		t.Fatalf("plain=%s id=%s", plain, id)
	}
}
