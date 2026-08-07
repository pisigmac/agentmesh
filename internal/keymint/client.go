package keymint

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to the Keymint control plane (provider keys + virtual keys + proxy).
type Client struct {
	baseURL    string
	apiKey     string
	teamID     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey, teamID string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  apiKey,
		teamID:  teamID,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.baseURL != "" && c.apiKey != "" && c.teamID != ""
}

func (c *Client) BaseURL() string { return c.baseURL }
func (c *Client) TeamID() string  { return c.teamID }

type Provider struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	AuthType  string `json:"auth_type"`
	CreatedAt string `json:"created_at"`
}

type ProviderKey struct {
	ID         string         `json:"id"`
	ProviderID string         `json:"provider_id"`
	Status     string         `json:"status"`
	CreatedAt  string         `json:"created_at"`
	Metadata   map[string]any `json:"metadata"`
}

type VirtualKey struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ProviderKeyID string `json:"provider_key_id"`
}

func (c *Client) ListProviders(ctx context.Context) ([]Provider, error) {
	var out struct {
		Providers []Provider `json:"providers"`
	}
	if err := c.get(ctx, "/v1/providers", &out); err != nil {
		return nil, err
	}
	return out.Providers, nil
}

func (c *Client) CreateProvider(ctx context.Context, name, baseURL, authType string) (Provider, error) {
	body := map[string]string{
		"name":      name,
		"base_url":  baseURL,
		"auth_type": authType,
	}
	var p Provider
	if err := c.post(ctx, "/v1/providers", body, &p); err != nil {
		return Provider{}, err
	}
	return p, nil
}

func (c *Client) EnsureProvider(ctx context.Context, name, baseURL, authType string) (Provider, error) {
	list, err := c.ListProviders(ctx)
	if err != nil {
		return Provider{}, err
	}
	for _, p := range list {
		if p.Name == name {
			return p, nil
		}
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if authType == "" {
		authType = "bearer"
	}
	return c.CreateProvider(ctx, name, baseURL, authType)
}

func (c *Client) CreateProviderKey(ctx context.Context, providerID, plaintext string, metadata map[string]any) (ProviderKey, error) {
	body := map[string]any{
		"provider_id": providerID,
		"plaintext":   plaintext,
		"metadata":    metadata,
	}
	var pk ProviderKey
	if err := c.post(ctx, "/v1/provider-keys", body, &pk); err != nil {
		return ProviderKey{}, err
	}
	return pk, nil
}

func (c *Client) ListProviderKeys(ctx context.Context) ([]ProviderKey, error) {
	var out struct {
		ProviderKeys []ProviderKey `json:"provider_keys"`
	}
	if err := c.get(ctx, "/v1/provider-keys", &out); err != nil {
		return nil, err
	}
	return out.ProviderKeys, nil
}

func (c *Client) DeleteProviderKey(ctx context.Context, id string) error {
	return c.delete(ctx, "/v1/provider-keys/"+id)
}

func (c *Client) CreateVirtualKey(ctx context.Context, name, providerKeyID string) (VirtualKey, string, error) {
	body := map[string]any{
		"name":            name,
		"team_id":         c.teamID,
		"provider_key_id": providerKeyID,
		"policy": map[string]any{
			"allowed_models":      []string{},
			"max_tokens_per_req":  0,
			"max_tokens_per_day":  0,
			"max_spend_per_day":   0,
			"max_spend_per_month": 0,
			"allowed_ips":         []string{},
			"rate_limit_rpm":      0,
		},
	}
	var out struct {
		VirtualKey VirtualKey `json:"virtual_key"`
		Token      string     `json:"token"`
	}
	if err := c.post(ctx, "/v1/virtual-keys", body, &out); err != nil {
		return VirtualKey{}, "", err
	}
	return out.VirtualKey, out.Token, nil
}

func (c *Client) ProxyChatURL() string {
	return c.baseURL + "/v1/proxy/chat/completions"
}

func (c *Client) get(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	return c.do(req, dest)
}

func (c *Client) post(ctx context.Context, path string, body any, dest any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, dest)
}

func (c *Client) delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	return c.do(req, nil)
}

func (c *Client) do(req *http.Request, dest any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("keymint %s %s: %d %s", req.Method, req.URL.Path, resp.StatusCode, truncate(string(b), 300))
	}
	if dest == nil || len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, dest)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func ProviderName(agentmeshProvider string) string {
	return "agentmesh-" + agentmeshProvider
}

func VirtualKeyName(orgID, provider string) string {
	return fmt.Sprintf("agentmesh-org-%s-%s", orgID, provider)
}
