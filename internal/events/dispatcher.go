package events

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type Dispatcher struct {
	URL    string
	Secret string
	Client *http.Client
}

type Event struct {
	Type      string         `json:"type"`
	OrgID     string         `json:"org_id"`
	Timestamp string         `json:"timestamp"`
	Payload   map[string]any `json:"payload"`
}

func NewDispatcher(url, secret string) *Dispatcher {
	return &Dispatcher{
		URL:    url,
		Secret: secret,
		Client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (d *Dispatcher) Emit(ctx context.Context, ev Event) error {
	if d.URL == "" {
		return nil
	}
	ev.Timestamp = time.Now().UTC().Format(time.RFC3339)
	body, _ := json.Marshal(ev)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if d.Secret != "" {
		req.Header.Set("X-AgentMesh-Webhook-Secret", d.Secret)
	}
	resp, err := d.Client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
