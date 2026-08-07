package guardloop

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/agentmesh/agentmesh/internal/config"
)

type Client struct {
	cfg    config.GuardLoopConfig
	client *http.Client
	cache  map[string]cachedScore
	mu     sync.RWMutex
}

type cachedScore struct {
	score     float64
	expiresAt time.Time
}

func NewClient(cfg config.GuardLoopConfig) *Client {
	return &Client{
		cfg: cfg,
		client: &http.Client{Timeout: cfg.Timeout},
		cache:  map[string]cachedScore{},
	}
}

func (c *Client) Name() string { return "guardloop" }

func (c *Client) Enabled() bool {
	return c != nil && c.cfg.Enabled && c.cfg.BaseURL != ""
}

// Score fetches (or caches) a GuardLoop aggregate score for agentType.
func (c *Client) Score(ctx context.Context, agentType string) (float64, error) {
	if !c.Enabled() {
		return 0, fmt.Errorf("guardloop not enabled")
	}
	c.mu.RLock()
	if cs, ok := c.cache[agentType]; ok && time.Now().Before(cs.expiresAt) {
		c.mu.RUnlock()
		return cs.score, nil
	}
	c.mu.RUnlock()

	score, err := c.fetchAggregate(ctx, agentType)
	if err != nil {
		return 0, err
	}
	c.mu.Lock()
	c.cache[agentType] = cachedScore{score: score, expiresAt: time.Now().Add(c.cfg.CacheTTL)}
	c.mu.Unlock()
	return score, nil
}

func (c *Client) fetchAggregate(ctx context.Context, agentType string) (float64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.cfg.BaseURL+"/scores?limit=100", nil)
	if err != nil {
		return 0, err
	}
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("guardloop request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return 0, fmt.Errorf("guardloop status %d", resp.StatusCode)
	}
	var body struct {
		Scores []struct {
			AgentType string  `json:"agent_type"`
			Score     float64 `json:"score"`
		} `json:"scores"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("guardloop decode: %w", err)
	}
	var sum float64
	var n int
	for _, s := range body.Scores {
		if s.AgentType == agentType || s.AgentType == "" {
			sum += s.Score
			n++
		}
	}
	if n == 0 {
		return 0, fmt.Errorf("guardloop: no scores for agent_type %q", agentType)
	}
	return sum / float64(n), nil
}
