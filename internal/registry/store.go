package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

const defaultLeaseTTL = 30

// DefaultOrgID is the platform org; discovery and routing include agents from every org.
const DefaultOrgID = "default"

type AgentRecord struct {
	OrgID        string            `json:"org_id"`
	AgentID      string            `json:"agent_id"`
	AgentType    string            `json:"agent_type"`
	Capabilities []string          `json:"capabilities"`
	EndpointURL  string            `json:"endpoint_url"`
	Healthy      bool              `json:"healthy"`
	CostPerReq   float64           `json:"cost_per_request_usd"`
	CostPer1K    float64           `json:"cost_per_1k_tokens_usd"`
	LatencyEWMA  float64           `json:"latency_ewma_ms"`
	SuccessRate  float64           `json:"success_rate"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type Store struct {
	client   *clientv3.Client
	prefix   string
	leaseTTL int
}

func NewStore(endpoints []string, prefix string, leaseTTLSeconds int) (*Store, error) {
	if leaseTTLSeconds <= 0 {
		leaseTTLSeconds = defaultLeaseTTL
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &Store{client: cli, prefix: strings.TrimSuffix(prefix, "/"), leaseTTL: leaseTTLSeconds}, nil
}

func IsPersistentRegistration(rec AgentRecord) bool {
	if rec.Metadata == nil {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(rec.Metadata["persistent"]))
	return v == "true" || v == "1" || v == "yes"
}

func (s *Store) Close() error {
	return s.client.Close()
}

func agentKey(prefix, orgID, agentID string) string {
	return path.Join(prefix, "orgs", orgID, "agents", agentID)
}

func (s *Store) Register(ctx context.Context, rec AgentRecord) (leaseTTL int64, err error) {
	if rec.AgentID == "" || rec.OrgID == "" {
		return 0, fmt.Errorf("org_id and agent_id required")
	}
	rec.UpdatedAt = time.Now().UTC()
	if rec.SuccessRate == 0 {
		rec.SuccessRate = 1.0
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return 0, err
	}
	key := agentKey(s.prefix, rec.OrgID, rec.AgentID)
	if IsPersistentRegistration(rec) {
		_, err = s.client.Put(ctx, key, string(data))
		if err != nil {
			return 0, err
		}
		return 0, nil
	}
	lease, err := s.client.Grant(ctx, int64(s.leaseTTL))
	if err != nil {
		return 0, err
	}
	_, err = s.client.Put(ctx, key, string(data), clientv3.WithLease(lease.ID))
	if err != nil {
		return 0, err
	}
	return int64(s.leaseTTL), nil
}

func (s *Store) UpdateHealth(ctx context.Context, orgID, agentID string, healthy bool, message string) error {
	rec, err := s.Get(ctx, orgID, agentID)
	if err != nil {
		return err
	}
	if rec == nil {
		return fmt.Errorf("agent not found")
	}
	rec.Healthy = healthy
	if message != "" {
		if rec.Metadata == nil {
			rec.Metadata = map[string]string{}
		}
		rec.Metadata["health_message"] = message
	}
	rec.UpdatedAt = time.Now().UTC()
	data, _ := json.Marshal(rec)
	key := agentKey(s.prefix, orgID, agentID)
	_, err = s.client.Put(ctx, key, string(data))
	return err
}

func (s *Store) Get(ctx context.Context, orgID, agentID string) (*AgentRecord, error) {
	resp, err := s.client.Get(ctx, agentKey(s.prefix, orgID, agentID))
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, nil
	}
	var rec AgentRecord
	if err := json.Unmarshal(resp.Kvs[0].Value, &rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func (s *Store) ListByOrg(ctx context.Context, orgID string) ([]AgentRecord, error) {
	if orgID == DefaultOrgID {
		return s.ListAllAgents(ctx)
	}
	prefix := path.Join(s.prefix, "orgs", orgID, "agents") + "/"
	return s.listAgentsPrefix(ctx, prefix)
}

func (s *Store) ListAllAgents(ctx context.Context) ([]AgentRecord, error) {
	prefix := path.Join(s.prefix, "orgs") + "/"
	resp, err := s.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	out := make([]AgentRecord, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		key := string(kv.Key)
		if !strings.Contains(key, "/agents/") {
			continue
		}
		var rec AgentRecord
		if err := json.Unmarshal(kv.Value, &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (s *Store) ListOrgIDs(ctx context.Context) ([]string, error) {
	prefix := path.Join(s.prefix, "orgs") + "/"
	resp, err := s.client.Get(ctx, prefix, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{DefaultOrgID: {}}
	for _, kv := range resp.Kvs {
		rel := strings.TrimPrefix(string(kv.Key), prefix)
		if rel == "" {
			continue
		}
		orgID, _, _ := strings.Cut(rel, "/")
		if orgID != "" {
			seen[orgID] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

func (s *Store) listAgentsPrefix(ctx context.Context, prefix string) ([]AgentRecord, error) {
	resp, err := s.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}
	out := make([]AgentRecord, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var rec AgentRecord
		if err := json.Unmarshal(kv.Value, &rec); err != nil {
			continue
		}
		out = append(out, rec)
	}
	return out, nil
}

func (s *Store) Watch(ctx context.Context, orgID string, ch chan<- AgentRecord) error {
	prefix := path.Join(s.prefix, "orgs", orgID, "agents") + "/"
	rch := s.client.Watch(ctx, prefix, clientv3.WithPrefix())
	go func() {
		for w := range rch {
			for _, ev := range w.Events {
				if ev.Type == clientv3.EventTypeDelete {
					continue
				}
				var rec AgentRecord
				if json.Unmarshal(ev.Kv.Value, &rec) == nil {
					select {
					case ch <- rec:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return nil
}

func HasCapability(rec AgentRecord, cap string) bool {
	for _, c := range rec.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}
