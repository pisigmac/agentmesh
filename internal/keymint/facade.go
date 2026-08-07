package keymint

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/agentmesh/agentmesh/internal/models"
)

// SecretView is a safe listing (no tokens).
type SecretView struct {
	Provider   string    `json:"provider"`
	Label      string    `json:"label,omitempty"`
	Configured bool      `json:"configured"`
	KeyHint    string    `json:"key_hint,omitempty"`
	BaseURL    string    `json:"base_url,omitempty"`
	APIModel   string    `json:"api_model,omitempty"`
	UpdatedAt  time.Time `json:"updated_at,omitempty"`
	Backend    string    `json:"backend,omitempty"`
}

// Facade manages provider secrets via Keymint + local binding store.
type Facade struct {
	Client *Client
	Binds  *BindStore
}

func (f *Facade) Configured() bool {
	return f != nil && f.Client != nil && f.Client.Configured() && f.Binds != nil
}

func (f *Facade) ListViews(ctx context.Context, orgID string, catalog *models.Catalog) ([]SecretView, error) {
	binds, err := f.Binds.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	byProv := map[string]Binding{}
	for _, b := range binds {
		byProv[b.Provider] = b
	}
	var out []SecretView
	for _, p := range catalog.Providers() {
		v := SecretView{
			Provider: p.ID,
			Label:    p.Label,
			Backend:  "keymint",
		}
		if b, ok := byProv[p.ID]; ok {
			v.Configured = true
			v.KeyHint = b.KeyHint
			v.BaseURL = b.BaseURL
			v.APIModel = b.APIModel
			v.UpdatedAt = b.UpdatedAt
		}
		out = append(out, v)
	}
	return out, nil
}

func (f *Facade) Put(ctx context.Context, orgID, provider, apiKey, baseURL, apiModel string, catalog *models.Catalog) error {
	if !f.Configured() {
		return fmt.Errorf("keymint not configured (set KEYMINT_BASE_URL, KEYMINT_API_KEY, KEYMINT_TEAM_ID)")
	}
	if orgID == "" || provider == "" {
		return fmt.Errorf("org_id and provider required")
	}
	prov, ok := catalog.ProviderByID(provider)
	if !ok {
		return fmt.Errorf("unknown provider %s", provider)
	}
	if strings.TrimSpace(apiKey) == "" {
		// Allow updating base_url/api_model only when binding exists
		existing, err := f.Binds.Get(ctx, orgID, provider)
		if err != nil {
			return fmt.Errorf("api_key required for new Keymint secret")
		}
		if baseURL != "" {
			existing.BaseURL = baseURL
		}
		if apiModel != "" {
			existing.APIModel = apiModel
		}
		return f.Binds.Put(ctx, orgID, existing)
	}

	authType := "bearer"
	if prov.APIStyle == models.APIStyleAnthropic {
		authType = "api-key"
	}
	kmBase := baseURL
	if kmBase == "" {
		kmBase = prov.DefaultBaseURL
	}
	if kmBase == "" {
		kmBase = "https://api.openai.com/v1"
	}

	kmProv, err := f.Client.EnsureProvider(ctx, ProviderName(provider), kmBase, authType)
	if err != nil {
		return fmt.Errorf("ensure keymint provider: %w", err)
	}

	// Replace prior key if present
	if old, err := f.Binds.Get(ctx, orgID, provider); err == nil && old.ProviderKeyID != "" {
		_ = f.Client.DeleteProviderKey(ctx, old.ProviderKeyID)
		_ = f.Binds.Delete(ctx, orgID, provider)
	}

	meta := map[string]any{
		"agentmesh_org":      orgID,
		"agentmesh_provider": provider,
	}
	pk, err := f.Client.CreateProviderKey(ctx, kmProv.ID, apiKey, meta)
	if err != nil {
		return fmt.Errorf("create provider key: %w", err)
	}
	vk, token, err := f.Client.CreateVirtualKey(ctx, VirtualKeyName(orgID, provider), pk.ID)
	if err != nil {
		_ = f.Client.DeleteProviderKey(ctx, pk.ID)
		return fmt.Errorf("create virtual key: %w", err)
	}
	return f.Binds.Put(ctx, orgID, Binding{
		Provider:          provider,
		KeymintProviderID: kmProv.ID,
		ProviderKeyID:     pk.ID,
		VirtualKeyID:      vk.ID,
		VirtualKeyToken:   token,
		BaseURL:           baseURL,
		APIModel:          apiModel,
		KeyHint:           Hint(apiKey),
	})
}

func (f *Facade) Delete(ctx context.Context, orgID, provider string) error {
	if !f.Configured() {
		return fmt.Errorf("keymint not configured")
	}
	b, err := f.Binds.Get(ctx, orgID, provider)
	if err != nil {
		return err
	}
	if b.ProviderKeyID != "" {
		_ = f.Client.DeleteProviderKey(ctx, b.ProviderKeyID)
	}
	return f.Binds.Delete(ctx, orgID, provider)
}

func (f *Facade) GetBinding(ctx context.Context, orgID, provider string) (Binding, error) {
	return f.Binds.Get(ctx, orgID, provider)
}

// AuditEvent is an org-scoped Keymint lifecycle event for the console.
type AuditEvent struct {
	ID            string         `json:"id"`
	Action        string         `json:"action"`
	Provider      string         `json:"provider,omitempty"`
	ProviderKeyID string         `json:"provider_key_id,omitempty"`
	VirtualKeyID  string         `json:"virtual_key_id,omitempty"`
	Details       map[string]any `json:"details,omitempty"`
	CreatedAt     string         `json:"created_at"`
}

// FilterAuditForBindings keeps Keymint logs that match org bindings and attaches provider labels.
func FilterAuditForBindings(logs []AuditLog, binds []Binding, limit int) []AuditEvent {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	byPK := map[string]string{}
	byVK := map[string]string{}
	for _, b := range binds {
		if b.ProviderKeyID != "" {
			byPK[b.ProviderKeyID] = b.Provider
		}
		if b.VirtualKeyID != "" {
			byVK[b.VirtualKeyID] = b.Provider
		}
	}
	var out []AuditEvent
	for _, log := range logs {
		provider := ""
		if p, ok := byPK[log.ProviderKeyID]; ok {
			provider = p
		} else if p, ok := byVK[log.VirtualKeyID]; ok {
			provider = p
		} else {
			continue
		}
		out = append(out, AuditEvent{
			ID:            log.ID,
			Action:        log.Action,
			Provider:      provider,
			ProviderKeyID: log.ProviderKeyID,
			VirtualKeyID:  log.VirtualKeyID,
			Details:       log.Details,
			CreatedAt:     log.CreatedAt,
		})
		if len(out) >= limit {
			break
		}
	}
	return out
}

// ListAudit returns org-scoped Keymint audit events (no tokens/plaintext).
func (f *Facade) ListAudit(ctx context.Context, orgID string, limit int) ([]AuditEvent, error) {
	if !f.Configured() {
		return nil, fmt.Errorf("keymint not configured")
	}
	if orgID == "" {
		return nil, fmt.Errorf("org_id required")
	}
	binds, err := f.Binds.List(ctx, orgID)
	if err != nil {
		return nil, err
	}
	logs, err := f.Client.ListAuditLogs(ctx, 100, 0)
	if err != nil {
		return nil, err
	}
	return FilterAuditForBindings(logs, binds, limit), nil
}
