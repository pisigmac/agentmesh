package keymint

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

	clientv3 "go.etcd.io/etcd/client/v3"
)

var ErrBindingNotFound = errors.New("keymint binding not found")

// Binding links an AgentMesh org+provider to Keymint ids and a virtual key token.
type Binding struct {
	Provider          string    `json:"provider"`
	KeymintProviderID string    `json:"keymint_provider_id"`
	ProviderKeyID     string    `json:"provider_key_id"`
	VirtualKeyID      string    `json:"virtual_key_id"`
	VirtualKeyToken   string    `json:"virtual_key_token,omitempty"`
	BaseURL           string    `json:"base_url,omitempty"`
	APIModel          string    `json:"api_model,omitempty"`
	KeyHint           string    `json:"key_hint,omitempty"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type BindStore struct {
	client *clientv3.Client
	prefix string
	gcm    cipher.AEAD
}

func NewBindStore(endpoints []string, prefix, masterKey string) (*BindStore, error) {
	if strings.TrimSpace(masterKey) == "" {
		return nil, errors.New("empty secrets master key for keymint bindings")
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(masterKey)))
	block, err := aes.NewCipher(sum[:])
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
	return &BindStore{
		client: cli,
		prefix: strings.TrimSuffix(prefix, "/"),
		gcm:    gcm,
	}, nil
}

func (s *BindStore) Close() error { return s.client.Close() }

func (s *BindStore) key(orgID, provider string) string {
	return path.Join(s.prefix, "orgs", orgID, "keymint", "providers", provider)
}

func (s *BindStore) Put(ctx context.Context, orgID string, b Binding) error {
	if orgID == "" || b.Provider == "" || b.VirtualKeyToken == "" {
		return fmt.Errorf("org_id, provider, and virtual_key_token required")
	}
	b.UpdatedAt = time.Now().UTC()
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	nonce := make([]byte, s.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	sealed := s.gcm.Seal(nonce, nonce, raw, nil)
	_, err = s.client.Put(ctx, s.key(orgID, b.Provider), base64.StdEncoding.EncodeToString(sealed))
	return err
}

func (s *BindStore) Get(ctx context.Context, orgID, provider string) (Binding, error) {
	resp, err := s.client.Get(ctx, s.key(orgID, provider))
	if err != nil {
		return Binding{}, err
	}
	if len(resp.Kvs) == 0 {
		return Binding{}, ErrBindingNotFound
	}
	raw, err := base64.StdEncoding.DecodeString(string(resp.Kvs[0].Value))
	if err != nil {
		return Binding{}, err
	}
	ns := s.gcm.NonceSize()
	if len(raw) < ns {
		return Binding{}, errors.New("corrupt binding")
	}
	plain, err := s.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return Binding{}, err
	}
	var b Binding
	if err := json.Unmarshal(plain, &b); err != nil {
		return Binding{}, err
	}
	return b, nil
}

func (s *BindStore) Delete(ctx context.Context, orgID, provider string) error {
	_, err := s.client.Delete(ctx, s.key(orgID, provider))
	return err
}

func (s *BindStore) List(ctx context.Context, orgID string) ([]Binding, error) {
	prefix := path.Join(s.prefix, "orgs", orgID, "keymint", "providers") + "/"
	resp, err := s.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	var out []Binding
	for _, kv := range resp.Kvs {
		raw, err := base64.StdEncoding.DecodeString(string(kv.Value))
		if err != nil {
			continue
		}
		ns := s.gcm.NonceSize()
		if len(raw) < ns {
			continue
		}
		plain, err := s.gcm.Open(nil, raw[:ns], raw[ns:], nil)
		if err != nil {
			continue
		}
		var b Binding
		if err := json.Unmarshal(plain, &b); err != nil {
			continue
		}
		b.VirtualKeyToken = ""
		out = append(out, b)
	}
	return out, nil
}

func Hint(apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	if len(apiKey) <= 8 {
		return "****"
	}
	return apiKey[:4] + "…" + apiKey[len(apiKey)-4:]
}
