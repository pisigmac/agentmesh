package guardloop

import (
	"context"
	"encoding/json"
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

func (c *Client) Enabled() bool {
	return c.cfg.Enabled && c.cfg.BaseURL != ""
}

func (c *Client) Score(_ context.Context, agentType string) float64 {
	if !c.Enabled() {
		return 50
	}
	c.mu.RLock()
	if cs, ok := c.cache[agentType]; ok && time.Now().Before(cs.expiresAt) {
		c.mu.RUnlock()
		return cs.score
	}
	c.mu.RUnlock()

	score := c.fetchAggregate(agentType)
	c.mu.Lock()
	c.cache[agentType] = cachedScore{score: score, expiresAt: time.Now().Add(c.cfg.CacheTTL)}
	c.mu.Unlock()
	return score
}

func (c *Client) fetchAggregate(agentType string) float64 {
	req, err := http.NewRequest(http.MethodGet, c.cfg.BaseURL+"/scores?limit=100", nil)
	if err != nil {
		return 50
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	resp, err := c.client.Do(req)
	if err != nil || resp.StatusCode >= 400 {
		return 50
	}
	defer resp.Body.Close()
	var body struct {
		Scores []struct {
			AgentType string  `json:"agent_type"`
			Score     float64 `json:"score"`
		} `json:"scores"`
	}
	if json.NewDecoder(resp.Body).Decode(&body) != nil {
		return 50
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
		return 50
	}
	return sum / float64(n)
}
