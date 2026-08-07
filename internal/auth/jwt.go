package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type TokenIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenIssuer(secret string, ttlHours int) (*TokenIssuer, error) {
	secret = strings.TrimSpace(secret)
	if len(secret) < 32 {
		return nil, fmt.Errorf("AUTH_JWT_SECRET must be at least 32 characters")
	}
	if ttlHours <= 0 {
		ttlHours = 24
	}
	return &TokenIssuer{secret: []byte(secret), ttl: time.Duration(ttlHours) * time.Hour}, nil
}

func (t *TokenIssuer) Issue(userID, email string) (token string, jti string, expiresAt time.Time, err error) {
	jti = uuid.NewString()
	expiresAt = time.Now().UTC().Add(t.ttl)
	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(t.secret)
	if err != nil {
		return "", "", time.Time{}, err
	}
	return signed, jti, expiresAt, nil
}

func (t *TokenIssuer) Parse(tokenStr string) (*Claims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return t.secret, nil
	})
	if err != nil {
		return nil, ErrBadToken
	}
	claims, ok := tok.Claims.(*Claims)
	if !ok || !tok.Valid {
		return nil, ErrBadToken
	}
	return claims, nil
}

func (t *TokenIssuer) TTL() time.Duration { return t.ttl }
