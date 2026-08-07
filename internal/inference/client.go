package inference

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agentmesh/agentmesh/internal/keymint"
	"github.com/agentmesh/agentmesh/internal/models"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

// ChatViaKeymint calls Keymint's OpenAI-compatible proxy with a virtual key.
func (c *Client) ChatViaKeymint(ctx context.Context, km *keymint.Client, virtualKey string, m models.Model, apiModel string, msgs []Message, route *RouteInfo) (ChatResult, error) {
	if km == nil || !km.Configured() {
		return ChatResult{}, fmt.Errorf("keymint not configured")
	}
	if virtualKey == "" {
		return ChatResult{}, fmt.Errorf("missing virtual key")
	}
	if apiModel == "" {
		apiModel = m.APIModel
	}
	body := map[string]any{
		"model":    apiModel,
		"messages": msgs,
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, km.ProxyChatURL(), bytes.NewReader(raw))
	if err != nil {
		return ChatResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+virtualKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return ChatResult{}, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return ChatResult{}, fmt.Errorf("keymint proxy %d: %s", resp.StatusCode, truncate(string(b), 400))
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
		return ChatResult{}, err
	}
	if len(parsed.Choices) == 0 {
		return ChatResult{}, fmt.Errorf("no choices in keymint proxy response")
	}
	out := ChatResult{
		Answer:   parsed.Choices[0].Message.Content,
		ModelID:  m.ID,
		Provider: m.Provider,
		APIModel: apiModel,
		Usage:    parsed.Usage,
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

// StreamTokenHandler is called for each text delta from a streaming chat.
type StreamTokenHandler func(text string) error

// ChatStreamViaKeymint streams OpenAI-compatible SSE from Keymint proxy.
func (c *Client) ChatStreamViaKeymint(ctx context.Context, km *keymint.Client, virtualKey string, m models.Model, apiModel string, msgs []Message, onToken StreamTokenHandler) (map[string]any, error) {
	if km == nil || !km.Configured() {
		return nil, fmt.Errorf("keymint not configured")
	}
	if apiModel == "" {
		apiModel = m.APIModel
	}
	body := map[string]any{
		"model":    apiModel,
		"messages": msgs,
		"stream":   true,
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, km.ProxyChatURL(), bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+virtualKey)
	req.Header.Set("Accept", "text/event-stream")

	httpClient := &http.Client{Timeout: 0} // stream until cancel
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("keymint proxy stream %d: %s", resp.StatusCode, truncate(string(b), 400))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var usage map[string]any
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage map[string]any `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if err := onToken(chunk.Choices[0].Delta.Content); err != nil {
				return usage, err
			}
		}
	}
	return usage, scanner.Err()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
