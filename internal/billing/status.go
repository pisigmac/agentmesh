package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Org billing enforcement levels (Stripe dunning).
const (
	BillingOK       = "ok"
	BillingPastDue  = "past_due"  // soft: allow routes, surface warning
	BillingBlocked  = "blocked"   // hard: reject routes
)

type OrgBillingStatus struct {
	OrgID     string    `json:"org_id"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

type StatusStore struct {
	client *clientv3.Client
	prefix string
}

func NewStatusStore(endpoints []string, prefix string) (*StatusStore, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return &StatusStore{client: cli, prefix: strings.TrimSuffix(prefix, "/")}, nil
}

func (s *StatusStore) Close() error {
	return s.client.Close()
}

func (s *StatusStore) key(orgID string) string {
	return path.Join(s.prefix, "orgs", orgID, "billing")
}

func (s *StatusStore) Get(ctx context.Context, orgID string) (OrgBillingStatus, error) {
	resp, err := s.client.Get(ctx, s.key(orgID))
	if err != nil {
		return OrgBillingStatus{}, err
	}
	if len(resp.Kvs) == 0 {
		return OrgBillingStatus{OrgID: orgID, Status: BillingOK}, nil
	}
	var st OrgBillingStatus
	if err := json.Unmarshal(resp.Kvs[0].Value, &st); err != nil {
		return OrgBillingStatus{}, err
	}
	if st.Status == "" {
		st.Status = BillingOK
	}
	return st, nil
}

func (s *StatusStore) Put(ctx context.Context, st OrgBillingStatus) error {
	if st.OrgID == "" {
		return fmt.Errorf("org_id required")
	}
	if st.Status == "" {
		st.Status = BillingOK
	}
	st.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	_, err = s.client.Put(ctx, s.key(st.OrgID), string(data))
	return err
}

// EnforceRoute returns an error message when billing blocks the org; empty string allows the route.
// Soft past_due returns ("", warn) so callers can inject a warning header.
func EnforceRoute(st OrgBillingStatus) (blockMsg string, warn string) {
	switch strings.ToLower(strings.TrimSpace(st.Status)) {
	case BillingBlocked:
		msg := "billing blocked for org"
		if st.Reason != "" {
			msg += ": " + st.Reason
		}
		return msg, ""
	case BillingPastDue:
		w := "billing past_due"
		if st.Reason != "" {
			w += ": " + st.Reason
		}
		return "", w
	default:
		return "", ""
	}
}
