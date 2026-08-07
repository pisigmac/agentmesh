package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"path"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Store struct {
	client *clientv3.Client
	prefix string
}

func NewStore(endpoints []string, prefix string) (*Store, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &Store{
		client: cli,
		prefix: strings.TrimSuffix(prefix, "/") + "/auth",
	}, nil
}

func (s *Store) Close() error { return s.client.Close() }

func (s *Store) putJSON(ctx context.Context, key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.client.Put(ctx, key, string(raw))
	return err
}

func (s *Store) getJSON(ctx context.Context, key string, dest any) error {
	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return err
	}
	if len(resp.Kvs) == 0 {
		return ErrNotFound
	}
	return json.Unmarshal(resp.Kvs[0].Value, dest)
}

func (s *Store) userKey(id string) string {
	return path.Join(s.prefix, "users", id)
}
func (s *Store) emailKey(email string) string {
	return path.Join(s.prefix, "users_by_email", strings.ToLower(email))
}
func (s *Store) sessionKey(jti string) string {
	return path.Join(s.prefix, "sessions", jti)
}
func (s *Store) orgKey(id string) string {
	return path.Join(s.prefix, "orgs", id)
}
func (s *Store) membershipKey(orgID, userID string) string {
	return path.Join(s.prefix, "memberships", orgID, userID)
}
func (s *Store) apiKeyKey(orgID, keyID string) string {
	return path.Join(s.prefix, "apikeys", orgID, keyID)
}

func (s *Store) CreateUser(ctx context.Context, u User) error {
	email := strings.ToLower(strings.TrimSpace(u.Email))
	u.Email = email
	if _, err := s.GetUserByEmail(ctx, email); err == nil {
		return ErrUserExists
	} else if err != ErrNotFound {
		return err
	}
	if err := s.putJSON(ctx, s.userKey(u.ID), u); err != nil {
		return err
	}
	_, err := s.client.Put(ctx, s.emailKey(email), u.ID)
	return err
}

func (s *Store) GetUser(ctx context.Context, id string) (User, error) {
	var u User
	err := s.getJSON(ctx, s.userKey(id), &u)
	return u, err
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	resp, err := s.client.Get(ctx, s.emailKey(strings.ToLower(strings.TrimSpace(email))))
	if err != nil {
		return User{}, err
	}
	if len(resp.Kvs) == 0 {
		return User{}, ErrNotFound
	}
	return s.GetUser(ctx, string(resp.Kvs[0].Value))
}

func (s *Store) UserCount(ctx context.Context) (int, error) {
	resp, err := s.client.Get(ctx, path.Join(s.prefix, "users")+"/", clientv3.WithPrefix(), clientv3.WithCountOnly())
	if err != nil {
		return 0, err
	}
	return int(resp.Count), nil
}

func (s *Store) PutSession(ctx context.Context, sess Session) error {
	return s.putJSON(ctx, s.sessionKey(sess.JTI), sess)
}

func (s *Store) GetSession(ctx context.Context, jti string) (Session, error) {
	var sess Session
	err := s.getJSON(ctx, s.sessionKey(jti), &sess)
	return sess, err
}

func (s *Store) RevokeSession(ctx context.Context, jti string) error {
	sess, err := s.GetSession(ctx, jti)
	if err != nil {
		return err
	}
	sess.Revoked = true
	return s.PutSession(ctx, sess)
}

func (s *Store) PutOrg(ctx context.Context, org Org) error {
	return s.putJSON(ctx, s.orgKey(org.ID), org)
}

func (s *Store) GetOrg(ctx context.Context, id string) (Org, error) {
	var o Org
	err := s.getJSON(ctx, s.orgKey(id), &o)
	return o, err
}

func (s *Store) PutMembership(ctx context.Context, m Membership) error {
	return s.putJSON(ctx, s.membershipKey(m.OrgID, m.UserID), m)
}

func (s *Store) GetMembership(ctx context.Context, orgID, userID string) (Membership, error) {
	var m Membership
	err := s.getJSON(ctx, s.membershipKey(orgID, userID), &m)
	return m, err
}

func (s *Store) ListMembershipsByUser(ctx context.Context, userID string) ([]Membership, error) {
	prefix := path.Join(s.prefix, "memberships") + "/"
	resp, err := s.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	var out []Membership
	for _, kv := range resp.Kvs {
		var m Membership
		if err := json.Unmarshal(kv.Value, &m); err != nil {
			continue
		}
		if m.UserID == userID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (s *Store) PutAPIKey(ctx context.Context, k APIKey) error {
	return s.putJSON(ctx, s.apiKeyKey(k.OrgID, k.KeyID), k)
}

func (s *Store) GetAPIKey(ctx context.Context, orgID, keyID string) (APIKey, error) {
	var k APIKey
	err := s.getJSON(ctx, s.apiKeyKey(orgID, keyID), &k)
	return k, err
}

func (s *Store) ListAPIKeys(ctx context.Context, orgID string) ([]APIKey, error) {
	prefix := path.Join(s.prefix, "apikeys", orgID) + "/"
	resp, err := s.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	var out []APIKey
	for _, kv := range resp.Kvs {
		var k APIKey
		if err := json.Unmarshal(kv.Value, &k); err != nil {
			continue
		}
		k.Hash = ""
		out = append(out, k)
	}
	return out, nil
}

func (s *Store) FindAPIKeyByHash(ctx context.Context, orgID, hash string) (APIKey, error) {
	prefix := path.Join(s.prefix, "apikeys", orgID) + "/"
	resp, err := s.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return APIKey{}, err
	}
	for _, kv := range resp.Kvs {
		var k APIKey
		if err := json.Unmarshal(kv.Value, &k); err != nil {
			continue
		}
		if k.Hash == hash && k.RevokedAt == nil {
			return k, nil
		}
	}
	return APIKey{}, ErrNotFound
}

func (s *Store) RevokeAPIKey(ctx context.Context, orgID, keyID string) error {
	k, err := s.GetAPIKey(ctx, orgID, keyID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	k.RevokedAt = &now
	return s.PutAPIKey(ctx, k)
}

// NewAPIKeyPlaintext returns amk_<32 hex> plaintext and a short key id.
func NewAPIKeyPlaintext() (plaintext, keyID string, err error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", err
	}
	keyID = hex.EncodeToString(b[:8])
	plaintext = "amk_" + hex.EncodeToString(b[:])
	return plaintext, keyID, nil
}
