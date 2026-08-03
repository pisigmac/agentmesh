package agentmesh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	OrgID      string
	HTTPClient *http.Client
}

type Config struct {
	BaseURL string
	APIKey  string
	OrgID   string
}

func New(cfg Config) *Client {
	return &Client{
		BaseURL:    cfg.BaseURL,
		APIKey:     cfg.APIKey,
		OrgID:      cfg.OrgID,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

type Registration struct {
	OrgID        string            `json:"org_id"`
	AgentID      string            `json:"agent_id"`
	AgentType    string            `json:"agent_type"`
	Capabilities []string          `json:"capabilities"`
	EndpointURL  string            `json:"endpoint_url"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Health       *Health           `json:"health,omitempty"`
	CostProfile  *CostProfile      `json:"cost_profile,omitempty"`
}

type Health struct {
	AgentID  string `json:"agent_id"`
	Healthy  bool   `json:"healthy"`
	Message  string `json:"message,omitempty"`
}

type CostProfile struct {
	CostPerRequestUsd   float64 `json:"cost_per_request_usd"`
	CostPer1KTokensUsd  float64 `json:"cost_per_1k_tokens_usd"`
}

func (c *Client) Register(ctx context.Context, reg Registration) error {
	if reg.OrgID == "" {
		reg.OrgID = c.OrgID
	}
	return c.post(ctx, "/v1/agents/register", reg, nil)
}

func (c *Client) Discover(ctx context.Context, capability string, maxCost float64) ([]map[string]any, error) {
	url := fmt.Sprintf("%s/v1/agents/discover?capability=%s", c.BaseURL, capability)
	if maxCost > 0 {
		url += fmt.Sprintf("&max_cost=%v", maxCost)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		Agents []map[string]any `json:"agents"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Agents, nil
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.applyHeaders(req)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("X-Org-ID", c.OrgID)
	req.Header.Set("X-Workspace-ID", c.OrgID)
}
