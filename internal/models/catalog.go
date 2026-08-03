package models

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// API styles supported by the inference client.
const (
	APIStyleOpenAI    = "openai"
	APIStyleAnthropic = "anthropic"
)

// Provider is a vendor entry from the catalog YAML.
type Provider struct {
	ID              string `yaml:"id" json:"id"`
	Label           string `yaml:"label" json:"label"`
	APIStyle        string `yaml:"api_style" json:"api_style"`
	DefaultBaseURL  string `yaml:"default_base_url" json:"default_base_url,omitempty"`
	RequiresBaseURL bool   `yaml:"requires_base_url" json:"requires_base_url,omitempty"`
	KeyHint         string `yaml:"key_hint" json:"key_hint,omitempty"`
}

// Model is a selectable inference target.
type Model struct {
	ID          string `yaml:"id" json:"id"`
	Label       string `yaml:"label" json:"label"`
	Provider    string `yaml:"provider" json:"provider"`
	APIModel    string `yaml:"api_model" json:"api_model"`
	Capability  string `yaml:"capability" json:"capability"`
	Description string `yaml:"description" json:"description,omitempty"`
	// Optional per-model base URL override (else provider default).
	DefaultBaseURL string `yaml:"default_base_url" json:"default_base_url,omitempty"`
}

type fileDoc struct {
	Providers []Provider `yaml:"providers"`
	Models    []Model    `yaml:"models"`
}

// Catalog is the loaded YAML catalog (thread-safe reads after Load).
type Catalog struct {
	mu        sync.RWMutex
	providers []Provider
	models    []Model
	byID      map[string]Model
	byProv    map[string]Provider
	path      string
}

func NewEmpty() *Catalog {
	return &Catalog{
		byID:   map[string]Model{},
		byProv: map[string]Provider{},
	}
}

// Load reads providers and models from a YAML file.
func Load(path string) (*Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read models catalog %s: %w", path, err)
	}
	var doc fileDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse models catalog: %w", err)
	}
	c := NewEmpty()
	c.path = path
	if err := c.replace(doc); err != nil {
		return nil, err
	}
	return c, nil
}

// ResolvePath picks the catalog file from env or common defaults.
func ResolvePath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if v := os.Getenv("MODELS_CATALOG_PATH"); v != "" {
		return v
	}
	for _, p := range []string{
		"deploy/models.yaml",
		"/etc/agentmesh/models.yaml",
		"models.yaml",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "deploy/models.yaml"
}

func (c *Catalog) replace(doc fileDoc) error {
	if len(doc.Providers) == 0 {
		return fmt.Errorf("catalog: at least one provider required")
	}
	if len(doc.Models) == 0 {
		return fmt.Errorf("catalog: at least one model required")
	}
	byProv := make(map[string]Provider, len(doc.Providers))
	for _, p := range doc.Providers {
		if p.ID == "" {
			return fmt.Errorf("catalog: provider missing id")
		}
		style := p.APIStyle
		if style == "" {
			style = APIStyleOpenAI
		}
		if style != APIStyleOpenAI && style != APIStyleAnthropic {
			return fmt.Errorf("catalog: provider %s has invalid api_style %q", p.ID, p.APIStyle)
		}
		p.APIStyle = style
		byProv[p.ID] = p
	}
	byID := make(map[string]Model, len(doc.Models))
	models := make([]Model, 0, len(doc.Models))
	for _, m := range doc.Models {
		if m.ID == "" || m.Provider == "" || m.APIModel == "" || m.Capability == "" {
			return fmt.Errorf("catalog: model %q requires id, provider, api_model, capability", m.ID)
		}
		prov, ok := byProv[m.Provider]
		if !ok {
			return fmt.Errorf("catalog: model %s references unknown provider %s", m.ID, m.Provider)
		}
		if m.DefaultBaseURL == "" {
			m.DefaultBaseURL = prov.DefaultBaseURL
		}
		if m.Label == "" {
			m.Label = m.ID
		}
		byID[m.ID] = m
		models = append(models, m)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providers = append([]Provider(nil), doc.Providers...)
	for i := range c.providers {
		c.providers[i] = byProv[c.providers[i].ID]
	}
	c.models = models
	c.byID = byID
	c.byProv = byProv
	return nil
}

func (c *Catalog) Path() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.path
}

func (c *Catalog) Models() []Model {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Model, len(c.models))
	copy(out, c.models)
	return out
}

func (c *Catalog) Providers() []Provider {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Provider, len(c.providers))
	copy(out, c.providers)
	return out
}

func (c *Catalog) ProviderIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]string, 0, len(c.providers))
	for _, p := range c.providers {
		out = append(out, p.ID)
	}
	return out
}

func (c *Catalog) ByID(id string) (Model, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	m, ok := c.byID[id]
	return m, ok
}

func (c *Catalog) ProviderByID(id string) (Provider, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.byProv[id]
	return p, ok
}

func (c *Catalog) HasProvider(id string) bool {
	_, ok := c.ProviderByID(id)
	return ok
}
