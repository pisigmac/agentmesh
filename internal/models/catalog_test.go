package models

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	root, _ := filepath.Abs("../..")
	path := filepath.Join(root, "deploy", "models.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Skip("deploy/models.yaml not found")
	}
	c, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Models()) < 1 || len(c.Providers()) < 1 {
		t.Fatal("expected providers and models")
	}
	m, ok := c.ByID("gpt-4o-mini")
	if !ok || m.Provider != "openai" || m.Capability != "model_openai" {
		t.Fatalf("unexpected model %+v", m)
	}
	p, ok := c.ProviderByID("anthropic")
	if !ok || p.APIStyle != APIStyleAnthropic {
		t.Fatalf("unexpected provider %+v", p)
	}
	if m.DefaultBaseURL == "" {
		t.Fatal("expected default base url inherited from provider")
	}
}

func TestLoadRejectsUnknownProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	_ = os.WriteFile(path, []byte(`
providers:
  - id: openai
    api_style: openai
models:
  - id: x
    provider: missing
    api_model: x
    capability: c
`), 0o644)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error")
	}
}
