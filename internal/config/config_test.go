package config

import (
	"os"
	"testing"
)

func TestAggregateDefaultOrgForcedOffInProduction(t *testing.T) {
	t.Setenv("ENV", "production")
	t.Setenv("AGGREGATE_DEFAULT_ORG", "true")
	cfg := Load()
	if cfg.AggregateDefaultOrg {
		t.Fatal("production must force AGGREGATE_DEFAULT_ORG=false")
	}
}

func TestAggregateDefaultOrgDevDefault(t *testing.T) {
	os.Unsetenv("AGGREGATE_DEFAULT_ORG")
	t.Setenv("ENV", "development")
	cfg := Load()
	if !cfg.AggregateDefaultOrg {
		t.Fatal("development default should aggregate for demos")
	}
}

func TestDataplaneAuthEnabledDefault(t *testing.T) {
	os.Unsetenv("DATAPLANE_AUTH_ENABLED")
	cfg := Load()
	if !cfg.DataPlane.AuthEnabled {
		t.Fatal("expected DATAPLANE_AUTH_ENABLED default true")
	}
}
