package auth

import (
	"errors"
	"time"
)

var (
	ErrMissingAuth     = errors.New("missing authorization")
	ErrInvalidKey      = errors.New("invalid api key")
	ErrMissingOrg      = errors.New("missing org header")
	ErrInvalidOrgID    = errors.New("invalid org id")
	ErrInvalidCreds    = errors.New("invalid email or password")
	ErrUserExists      = errors.New("user already exists")
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden")
	ErrRegisterDisabled = errors.New("registration disabled")
	ErrBadToken        = errors.New("invalid or expired token")
	ErrSessionRevoked  = errors.New("session revoked")
)

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

func (r Role) Rank() int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleOperator:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

func ParseRole(s string) Role {
	switch Role(s) {
	case RoleAdmin, RoleOperator, RoleViewer:
		return Role(s)
	default:
		return RoleViewer
	}
}

type AuthType string

const (
	AuthTypeJWT    AuthType = "jwt"
	AuthTypeAPIKey AuthType = "api_key"
)

type ContextKey struct{}

type Principal struct {
	OrgID    string
	KeyID    string
	UserID   string
	Email    string
	Role     Role
	AuthType AuthType
	SessionJTI string
}

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type Session struct {
	JTI       string    `json:"jti"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `json:"revoked"`
}

type Org struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Membership struct {
	OrgID  string `json:"org_id"`
	UserID string `json:"user_id"`
	Role   Role   `json:"role"`
}

type APIKey struct {
	KeyID     string     `json:"key_id"`
	OrgID     string     `json:"org_id"`
	Name      string     `json:"name"`
	Hash      string     `json:"hash,omitempty"`
	Role      Role       `json:"role"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type OrgMembershipView struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	Role Role   `json:"role"`
}
