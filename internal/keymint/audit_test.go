package keymint

import (
	"fmt"
	"testing"
)

func TestFilterAuditForBindings(t *testing.T) {
	binds := []Binding{
		{Provider: "openai", ProviderKeyID: "pk-1", VirtualKeyID: "vk-1"},
		{Provider: "anthropic", ProviderKeyID: "pk-2", VirtualKeyID: "vk-2"},
	}
	logs := []AuditLog{
		{ID: "1", Action: "key_created", ProviderKeyID: "pk-1", VirtualKeyID: "vk-1", CreatedAt: "2026-01-01T00:00:00Z"},
		{ID: "2", Action: "key_rotated", ProviderKeyID: "pk-other", VirtualKeyID: "vk-other", CreatedAt: "2026-01-02T00:00:00Z"},
		{ID: "3", Action: "key_revoked", ProviderKeyID: "", VirtualKeyID: "vk-2", CreatedAt: "2026-01-03T00:00:00Z"},
		{ID: "4", Action: "key_created", ProviderKeyID: "pk-2", VirtualKeyID: "vk-2", CreatedAt: "2026-01-04T00:00:00Z"},
	}
	got := FilterAuditForBindings(logs, binds, 50)
	if len(got) != 3 {
		t.Fatalf("want 3 events, got %d", len(got))
	}
	if got[0].Provider != "openai" || got[0].Action != "key_created" {
		t.Fatalf("first=%+v", got[0])
	}
	if got[1].Provider != "anthropic" || got[1].ID != "3" {
		t.Fatalf("second=%+v", got[1])
	}
	if got[2].Provider != "anthropic" || got[2].ID != "4" {
		t.Fatalf("third=%+v", got[2])
	}
}

func TestFilterAuditForBindingsLimit(t *testing.T) {
	binds := []Binding{{Provider: "openai", ProviderKeyID: "pk-1", VirtualKeyID: "vk-1"}}
	var logs []AuditLog
	for i := 0; i < 10; i++ {
		logs = append(logs, AuditLog{
			ID: fmt.Sprintf("id-%d", i), Action: "x", ProviderKeyID: "pk-1", VirtualKeyID: "vk-1",
		})
	}
	got := FilterAuditForBindings(logs, binds, 3)
	if len(got) != 3 {
		t.Fatalf("want 3, got %d", len(got))
	}
}
