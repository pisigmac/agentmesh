package policy

import (
	"context"
	"encoding/json"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/agentmesh/agentmesh/internal/config"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type Store struct {
	client     *clientv3.Client
	prefix     string
	envDefault config.Config

	mu    sync.RWMutex
	cache map[string]OrgPolicy
}

func NewStore(endpoints []string, prefix string, envDefault config.Config) (*Store, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	s := &Store{
		client:     cli,
		prefix:     strings.TrimSuffix(prefix, "/"),
		envDefault: envDefault,
		cache:      map[string]OrgPolicy{},
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.client.Close()
}

func policyKey(prefix, orgID string) string {
	return path.Join(prefix, "orgs", orgID, "policy")
}

func (s *Store) Get(ctx context.Context, orgID string) (OrgPolicy, error) {
	s.mu.RLock()
	if p, ok := s.cache[orgID]; ok {
		s.mu.RUnlock()
		return p, nil
	}
	s.mu.RUnlock()

	resp, err := s.client.Get(ctx, policyKey(s.prefix, orgID))
	if err != nil {
		return OrgPolicy{}, err
	}
	base := DefaultOrgPolicy(orgID, s.envDefault)
	if len(resp.Kvs) == 0 {
		s.mu.Lock()
		s.cache[orgID] = base
		s.mu.Unlock()
		return base, nil
	}
	var stored OrgPolicy
	if err := json.Unmarshal(resp.Kvs[0].Value, &stored); err != nil {
		return base, nil
	}
	merged := Merge(base, stored)
	merged.OrgID = orgID
	s.mu.Lock()
	s.cache[orgID] = merged
	s.mu.Unlock()
	return merged, nil
}

func (s *Store) Put(ctx context.Context, p OrgPolicy) error {
	if p.OrgID == "" {
		return errOrgRequired
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.client.Put(ctx, policyKey(s.prefix, p.OrgID), string(data))
	if err != nil {
		return err
	}
	base := DefaultOrgPolicy(p.OrgID, s.envDefault)
	merged := Merge(base, p)
	s.mu.Lock()
	s.cache[p.OrgID] = merged
	s.mu.Unlock()
	return nil
}

func (s *Store) Watch(ctx context.Context) {
	prefix := path.Join(s.prefix, "orgs") + "/"
	rch := s.client.Watch(ctx, prefix, clientv3.WithPrefix())
	go func() {
		for w := range rch {
			for _, ev := range w.Events {
				key := string(ev.Kv.Key)
				if !strings.HasSuffix(key, "/policy") {
					continue
				}
				parts := strings.Split(strings.Trim(key, "/"), "/")
				orgID := parts[len(parts)-2]
				if ev.Type == clientv3.EventTypeDelete {
					s.mu.Lock()
					delete(s.cache, orgID)
					s.mu.Unlock()
					continue
				}
				var stored OrgPolicy
				if json.Unmarshal(ev.Kv.Value, &stored) == nil {
					base := DefaultOrgPolicy(orgID, s.envDefault)
					s.mu.Lock()
					s.cache[orgID] = Merge(base, stored)
					s.mu.Unlock()
				}
			}
		}
	}()
}

var errOrgRequired = &policyError{"org_id required"}

type policyError struct{ msg string }

func (e *policyError) Error() string { return e.msg }
