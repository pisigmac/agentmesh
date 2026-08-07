package auth

import (
	"context"
	"fmt"
	"strings"
)

// MemoryKeyStore is retained for unit tests of hash helpers / legacy paths.
type MemoryKeyStore struct {
	keys map[string]map[string]string
}

func NewMemoryKeyStore(devKey string) *MemoryKeyStore {
	h := HashKey(devKey)
	return &MemoryKeyStore{
		keys: map[string]map[string]string{
			"default": {"dev": h},
		},
	}
}

func (m *MemoryKeyStore) ValidateKey(_ context.Context, orgID, keyPlaintext string) (string, bool, error) {
	if orgKeys, ok := m.keys[orgID]; ok {
		want := HashKey(keyPlaintext)
		for kid, hash := range orgKeys {
			if hash == want {
				return kid, true, nil
			}
		}
	}
	return "", false, nil
}

func ParseBearer(authHeader string) (string, error) {
	if authHeader == "" {
		return "", ErrMissingAuth
	}
	const p = "Bearer "
	if !strings.HasPrefix(authHeader, p) {
		return "", ErrInvalidKey
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, p))
	if token == "" {
		return "", ErrInvalidKey
	}
	return token, nil
}

func OrgFromHeaders(orgID, workspaceID string) (string, error) {
	orgID = strings.TrimSpace(orgID)
	workspaceID = strings.TrimSpace(workspaceID)
	if orgID != "" && workspaceID != "" && orgID != workspaceID {
		return "", fmt.Errorf("%w: X-Org-ID and X-Workspace-ID conflict", ErrInvalidOrgID)
	}
	id := orgID
	if id == "" {
		id = workspaceID
	}
	if id == "" {
		id = "default"
	}
	if !validOrgID(id) {
		return "", ErrInvalidOrgID
	}
	return id, nil
}

func validOrgID(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ContextKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ContextKey{}).(Principal)
	return p, ok
}

// Authenticate is a thin wrapper used by older tests; production uses Service.Authenticate.
func Authenticate(ctx context.Context, store interface {
	ValidateKey(ctx context.Context, orgID, keyPlaintext string) (keyID string, ok bool, err error)
}, orgID, workspaceID, authHeader string) (Principal, error) {
	org, err := OrgFromHeaders(orgID, workspaceID)
	if err != nil {
		return Principal{}, err
	}
	token, err := ParseBearer(authHeader)
	if err != nil {
		return Principal{}, err
	}
	kid, ok, err := store.ValidateKey(ctx, org, token)
	if err != nil {
		return Principal{}, err
	}
	if !ok {
		return Principal{}, ErrInvalidKey
	}
	return Principal{OrgID: org, KeyID: kid, Role: RoleAdmin, AuthType: AuthTypeAPIKey}, nil
}
