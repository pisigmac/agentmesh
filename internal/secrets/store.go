package secrets

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/agentmesh/agentmesh/internal/models"
	clientv3 "go.etcd.io/etcd/client/v3"
)

var (
	ErrNotFound = errors.New("secret not found")
	ErrBadKey   = errors.New("invalid secrets master key")
)

// ProviderSecret is stored encrypted; API responses never include APIKey.
type ProviderSecret struct {
	Provider  string    `json:"provider"`
	APIKey    string    `json:"api_key,omitempty"`
	BaseURL   string    `json:"base_url,omitempty"`
	APIModel  string    `json:"api_model,omitempty"` // override for custom provider
	UpdatedAt time.Time `json:"updated_at"`
}

// ProviderSecretView is a safe listing (no raw key).
type ProviderSecretView struct {
	Provider   string    `json:"provider"`
	Label      string    `json:"label,omitempty"`
	Configured bool      `json:"configured"`
	KeyHint    string    `json:"key_hint,omitempty"`
	BaseURL    string    `json:"base_url,omitempty"`
	APIModel   string    `json:"api_model,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
}

type Store struct {
	client *clientv3.Client
	prefix string
	gcm    cipher.AEAD
}

func NewStore(endpoints []string, prefix, masterKey string) (*Store, error) {
	key, err := deriveKey(masterKey)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &Store{
		client: cli,
		prefix: strings.TrimSuffix(prefix, "/"),
		gcm:    gcm,
	}, nil
}

func (s *Store) Close() error { return s.client.Close() }

func deriveKey(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrBadKey
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:], nil
}

func secretKey(prefix, orgID, provider string) string {
	return path.Join(prefix, "orgs", orgID, "secrets", "providers", provider)
}

func (s *Store) Put(ctx context.Context, orgID string, sec ProviderSecret) error {
	if orgID == "" || sec.Provider == "" {
		return fmt.Errorf("org_id and provider required")
	}
	if sec.APIKey == "" {
		existing, err := s.Get(ctx, orgID, sec.Provider)
		if err != nil {
			return err
		}
		sec.APIKey = existing.APIKey
	}
	if sec.APIKey == "" {
		return fmt.Errorf("api_key required")
	}
	sec.UpdatedAt = time.Now().UTC()
	plain, err := json.Marshal(sec)
	if err != nil {
		return err
	}
	ct, err := s.encrypt(plain)
	if err != nil {
		return err
	}
	_, err = s.client.Put(ctx, secretKey(s.prefix, orgID, sec.Provider), ct)
	return err
}

func (s *Store) Get(ctx context.Context, orgID, provider string) (ProviderSecret, error) {
	resp, err := s.client.Get(ctx, secretKey(s.prefix, orgID, provider))
	if err != nil {
		return ProviderSecret{}, err
	}
	if len(resp.Kvs) == 0 {
		return ProviderSecret{}, ErrNotFound
	}
	plain, err := s.decrypt(string(resp.Kvs[0].Value))
	if err != nil {
		return ProviderSecret{}, err
	}
	var sec ProviderSecret
	if err := json.Unmarshal(plain, &sec); err != nil {
		return ProviderSecret{}, err
	}
	return sec, nil
}

func (s *Store) Delete(ctx context.Context, orgID, provider string) error {
	_, err := s.client.Delete(ctx, secretKey(s.prefix, orgID, provider))
	return err
}

func (s *Store) ListViews(ctx context.Context, orgID string, catalog *models.Catalog) ([]ProviderSecretView, error) {
	provs := catalog.Providers()
	out := make([]ProviderSecretView, 0, len(provs))
	for _, p := range provs {
		sec, err := s.Get(ctx, orgID, p.ID)
		if errors.Is(err, ErrNotFound) {
			out = append(out, ProviderSecretView{Provider: p.ID, Label: p.Label, Configured: false})
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, ProviderSecretView{
			Provider:   p.ID,
			Label:      p.Label,
			Configured: true,
			KeyHint:    hint(sec.APIKey),
			BaseURL:    sec.BaseURL,
			APIModel:   sec.APIModel,
			UpdatedAt:  sec.UpdatedAt,
		})
	}
	return out, nil
}

func hint(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "…" + key[len(key)-4:]
}

func (s *Store) encrypt(plain []byte) (string, error) {
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := s.gcm.Seal(nonce, nonce, plain, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func (s *Store) decrypt(b64 string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	ns := s.gcm.NonceSize()
	if len(raw) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:ns], raw[ns:]
	return s.gcm.Open(nil, nonce, ct, nil)
}
