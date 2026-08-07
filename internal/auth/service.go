package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service struct {
	Store          *Store
	Tokens         *TokenIssuer
	AllowRegister  bool
	DevAPIKeyPlain string // seeded for org default in Bootstrap
}

func HashKey(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func (s *Service) Bootstrap(ctx context.Context, email, password string) error {
	n, err := s.Store.UserCount(ctx)
	if err != nil {
		return err
	}
	if n > 0 {
		return s.ensureDevAPIKey(ctx)
	}
	email = strings.TrimSpace(email)
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return s.ensureDevAPIKey(ctx)
	}
	if _, err := s.Register(ctx, email, password, true); err != nil && err != ErrUserExists {
		return err
	}
	return s.ensureDevAPIKey(ctx)
}

func (s *Service) ensureDevAPIKey(ctx context.Context) error {
	if s.DevAPIKeyPlain == "" {
		return nil
	}
	orgID := "default"
	if _, err := s.Store.GetOrg(ctx, orgID); err == ErrNotFound {
		_ = s.Store.PutOrg(ctx, Org{ID: orgID, Name: "Default", CreatedAt: time.Now().UTC()})
	}
	hash := HashKey(s.DevAPIKeyPlain)
	if _, err := s.Store.FindAPIKeyByHash(ctx, orgID, hash); err == nil {
		return nil
	}
	return s.Store.PutAPIKey(ctx, APIKey{
		KeyID:     "dev",
		OrgID:     orgID,
		Name:      "dev",
		Hash:      hash,
		Role:      RoleAdmin,
		CreatedAt: time.Now().UTC(),
	})
}

type AuthResult struct {
	Token string               `json:"token"`
	User  UserPublic           `json:"user"`
	Orgs  []OrgMembershipView  `json:"orgs"`
}

type UserPublic struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (s *Service) Register(ctx context.Context, email, password string, force bool) (AuthResult, error) {
	if !force && !s.AllowRegister {
		n, err := s.Store.UserCount(ctx)
		if err != nil {
			return AuthResult{}, err
		}
		if n > 0 {
			return AuthResult{}, ErrRegisterDisabled
		}
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || len(password) < 8 {
		return AuthResult{}, fmt.Errorf("email required and password min 8 chars")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return AuthResult{}, err
	}
	u := User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
	}
	if err := s.Store.CreateUser(ctx, u); err != nil {
		return AuthResult{}, err
	}
	orgID := "default"
	if _, err := s.Store.GetOrg(ctx, orgID); err == ErrNotFound {
		_ = s.Store.PutOrg(ctx, Org{ID: orgID, Name: "Default", CreatedAt: time.Now().UTC()})
	}
	_ = s.Store.PutMembership(ctx, Membership{OrgID: orgID, UserID: u.ID, Role: RoleAdmin})
	return s.issueForUser(ctx, u)
}

func (s *Service) Login(ctx context.Context, email, password string) (AuthResult, error) {
	u, err := s.Store.GetUserByEmail(ctx, email)
	if err != nil {
		return AuthResult{}, ErrInvalidCreds
	}
	if !CheckPassword(u.PasswordHash, password) {
		return AuthResult{}, ErrInvalidCreds
	}
	return s.issueForUser(ctx, u)
}

func (s *Service) issueForUser(ctx context.Context, u User) (AuthResult, error) {
	token, jti, exp, err := s.Tokens.Issue(u.ID, u.Email)
	if err != nil {
		return AuthResult{}, err
	}
	if err := s.Store.PutSession(ctx, Session{JTI: jti, UserID: u.ID, ExpiresAt: exp}); err != nil {
		return AuthResult{}, err
	}
	orgs, err := s.orgViews(ctx, u.ID)
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{
		Token: token,
		User:  UserPublic{ID: u.ID, Email: u.Email},
		Orgs:  orgs,
	}, nil
}

func (s *Service) orgViews(ctx context.Context, userID string) ([]OrgMembershipView, error) {
	ms, err := s.Store.ListMembershipsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []OrgMembershipView
	for _, m := range ms {
		name := m.OrgID
		if o, err := s.Store.GetOrg(ctx, m.OrgID); err == nil {
			name = o.Name
		}
		out = append(out, OrgMembershipView{ID: m.OrgID, Name: name, Role: m.Role})
	}
	return out, nil
}

func (s *Service) Logout(ctx context.Context, jti string) error {
	if jti == "" {
		return nil
	}
	return s.Store.RevokeSession(ctx, jti)
}

func (s *Service) Me(ctx context.Context, userID string) (UserPublic, []OrgMembershipView, error) {
	u, err := s.Store.GetUser(ctx, userID)
	if err != nil {
		return UserPublic{}, nil, err
	}
	orgs, err := s.orgViews(ctx, userID)
	return UserPublic{ID: u.ID, Email: u.Email}, orgs, err
}

func (s *Service) CreateOrg(ctx context.Context, userID, orgID, name string) (Org, error) {
	orgID = strings.TrimSpace(orgID)
	if !validOrgID(orgID) {
		return Org{}, ErrInvalidOrgID
	}
	if _, err := s.Store.GetOrg(ctx, orgID); err == nil {
		return Org{}, fmt.Errorf("org already exists")
	}
	if name == "" {
		name = orgID
	}
	org := Org{ID: orgID, Name: name, CreatedAt: time.Now().UTC()}
	if err := s.Store.PutOrg(ctx, org); err != nil {
		return Org{}, err
	}
	if err := s.Store.PutMembership(ctx, Membership{OrgID: orgID, UserID: userID, Role: RoleAdmin}); err != nil {
		return Org{}, err
	}
	return org, nil
}

func (s *Service) CreateAPIKey(ctx context.Context, orgID, name string, role Role) (APIKey, string, error) {
	if role == "" {
		role = RoleOperator
	}
	plain, keyID, err := NewAPIKeyPlaintext()
	if err != nil {
		return APIKey{}, "", err
	}
	if name == "" {
		name = keyID
	}
	k := APIKey{
		KeyID:     keyID,
		OrgID:     orgID,
		Name:      name,
		Hash:      HashKey(plain),
		Role:      role,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.Store.PutAPIKey(ctx, k); err != nil {
		return APIKey{}, "", err
	}
	k.Hash = ""
	return k, plain, nil
}

func (s *Service) Authenticate(ctx context.Context, orgHeader, workspaceHeader, authHeader string) (Principal, error) {
	org, err := OrgFromHeaders(orgHeader, workspaceHeader)
	if err != nil {
		return Principal{}, err
	}
	token, err := ParseBearer(authHeader)
	if err != nil {
		return Principal{}, err
	}

	// Prefer JWT when it parses as one (three segments).
	if strings.Count(token, ".") == 2 {
		claims, err := s.Tokens.Parse(token)
		if err != nil {
			return Principal{}, err
		}
		sess, err := s.Store.GetSession(ctx, claims.ID)
		if err != nil {
			return Principal{}, ErrBadToken
		}
		if sess.Revoked || time.Now().UTC().After(sess.ExpiresAt) {
			return Principal{}, ErrSessionRevoked
		}
		m, err := s.Store.GetMembership(ctx, org, claims.Subject)
		if err != nil {
			return Principal{}, ErrForbidden
		}
		return Principal{
			OrgID:      org,
			UserID:     claims.Subject,
			Email:      claims.Email,
			Role:       m.Role,
			AuthType:   AuthTypeJWT,
			SessionJTI: claims.ID,
		}, nil
	}

	k, err := s.Store.FindAPIKeyByHash(ctx, org, HashKey(token))
	if err != nil {
		// Dev convenience: allow default org key when requesting another org only if membership exists — not for API keys.
		return Principal{}, ErrInvalidKey
	}
	return Principal{
		OrgID:    org,
		KeyID:    k.KeyID,
		Role:     k.Role,
		AuthType: AuthTypeAPIKey,
	}, nil
}

func RequireRole(p Principal, min Role) error {
	if p.Role.Rank() >= min.Rank() {
		return nil
	}
	return ErrForbidden
}
