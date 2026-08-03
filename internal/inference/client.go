package inference

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agentmesh/agentmesh/internal/models"
	"github.com/agentmesh/agentmesh/internal/secrets"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type ChatResult struct {
	Answer               string         `json:"answer"`
	ModelID              string         `json:"model_id"`
	Provider             string         `json:"provider"`
	APIModel             string         `json:"api_model"`
	DestinationAgentID   string         `json:"destination_agent_id,omitempty"`
	DestinationAgentType string         `json:"destination_agent_type,omitempty"`
	EndpointURL          string         `json:"endpoint_url,omitempty"`
	TraceID              string         `json:"trace_id,omitempty"`
	RoutedViaMesh        bool           `json:"routed_via_mesh"`
	Usage                map[string]any `json:"usage,omitempty"`
}

type RouteInfo struct {
	DestinationAgentID   string
	DestinationAgentType string
	EndpointURL          string
	TraceID              string
}

type Client struct {
	HTTP *http.Client
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 90 * time.Second}}
}

func (c *Client) Chat(ctx context.Context, catalog *models.Catalog, m models.Model, sec secrets.ProviderSecret, msgs []Message, route *RouteInfo) (ChatResult, error) {
	prov, ok := catalog.ProviderByID(m.Provider)
	if !ok {
		return ChatResult{}, fmt.Errorf("unknown provider %s", m.Provider)
	}
	apiModel := m.APIModel
	if prov.RequiresBaseURL && sec.APIModel != "" {
		apiModel = sec.APIModel
	}
	base := strings.TrimRight(sec.BaseURL, "/")
	if base == "" {
		base = strings.TrimRight(m.DefaultBaseURL, "/")
	}
	if base == "" {
		base = strings.TrimRight(prov.DefaultBaseURL, "/")
	}
	if base == "" && prov.RequiresBaseURL {
		return ChatResult{}, fmt.Errorf("provider %s requires base_url in secrets", prov.ID)
	}

	var (
		answer string
		usage  map[string]any
		err    error
	)
	switch prov.APIStyle {
	case models.APIStyleAnthropic:
		answer, usage, err = c.anthropic(ctx, base, sec.APIKey, apiModel, msgs)
	default:
		answer, usage, err = c.openaiCompat(ctx, base, sec.APIKey, apiModel, msgs)
	}
	if err != nil {
		return ChatResult{}, err
	}
	out := ChatResult{
		Answer:   answer,
		ModelID:  m.ID,
		Provider: m.Provider,
		APIModel: apiModel,
		Usage:    usage,
	}
	if route != nil {
		out.DestinationAgentID = route.DestinationAgentID
		out.DestinationAgentType = route.DestinationAgentType
		out.EndpointURL = route.EndpointURL
		out.TraceID = route.TraceID
		out.RoutedViaMesh = true
	}
	return out, nil
}

func (c *Client) openaiCompat(ctx context.Context, base, apiKey, model string, msgs []Message) (string, map[string]any, error) {
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	body := map[string]any{
		"model":    model,
		"messages": msgs,
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("openai-compatible %d: %s", resp.StatusCode, truncate(string(b), 400))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage map[string]any `json:"usage"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return "", nil, err
	}
	if len(parsed.Choices) == 0 {
		return "", nil, fmt.Errorf("no choices in provider response")
	}
	return parsed.Choices[0].Message.Content, parsed.Usage, nil
}

func (c *Client) anthropic(ctx context.Context, base, apiKey, model string, msgs []Message) (string, map[string]any, error) {
	if base == "" {
		base = "https://api.anthropic.com"
	}
	var system string
	var converted []map[string]string
	for _, m := range msgs {
		if m.Role == "system" {
			if system != "" {
				system += "\n"
			}
			system += m.Content
			continue
		}
		role := m.Role
		if role != "assistant" {
			role = "user"
		}
		converted = append(converted, map[string]string{"role": role, "content": m.Content})
	}
	body := map[string]any{
		"model":      model,
		"max_tokens": 2048,
		"messages":   converted,
	}
	if system != "" {
		body["system"] = system
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("anthropic %d: %s", resp.StatusCode, truncate(string(b), 400))
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Usage map[string]any `json:"usage"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		return "", nil, err
	}
	var out strings.Builder
	for _, part := range parsed.Content {
		if part.Type == "text" || part.Text != "" {
			out.WriteString(part.Text)
		}
	}
	if out.Len() == 0 {
		return "", nil, fmt.Errorf("empty anthropic response")
	}
	return out.String(), parsed.Usage, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
