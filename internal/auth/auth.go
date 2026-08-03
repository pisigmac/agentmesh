package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

var (
	ErrMissingAuth   = errors.New("missing authorization")
	ErrInvalidKey    = errors.New("invalid api key")
	ErrMissingOrg    = errors.New("missing org header")
	ErrInvalidOrgID  = errors.New("invalid org id")
)

type ContextKey struct{}

type Principal struct {
	OrgID string
	KeyID string
}

type KeyStore interface {
	ValidateKey(ctx context.Context, orgID, keyPlaintext string) (keyID string, ok bool, err error)
}

type MemoryKeyStore struct {
	// org -> kid -> sha256 hex
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

func HashKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func (m *MemoryKeyStore) ValidateKey(_ context.Context, orgID, keyPlaintext string) (string, bool, error) {
	if kid, ok := m.validateOrgKey(orgID, keyPlaintext); ok {
		return kid, true, nil
	}
	if orgID != "default" {
		if kid, ok := m.validateOrgKey("default", keyPlaintext); ok {
			return kid, true, nil
		}
	}
	return "", false, nil
}

func (m *MemoryKeyStore) validateOrgKey(orgID, keyPlaintext string) (string, bool) {
	orgKeys, ok := m.keys[orgID]
	if !ok {
		return "", false
	}
	want := HashKey(keyPlaintext)
	for kid, hash := range orgKeys {
		if hash == want {
			return kid, true
		}
	}
	return "", false
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

func Authenticate(ctx context.Context, store KeyStore, orgID, workspaceID, authHeader string) (Principal, error) {
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
	return Principal{OrgID: org, KeyID: kid}, nil
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ContextKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ContextKey{}).(Principal)
	return p, ok
}
