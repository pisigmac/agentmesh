package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env string

	ControlPlane         ControlPlaneConfig
	DataPlane            DataPlaneConfig
	Etcd                 EtcdConfig
	Redis                RedisConfig
	ClickHouse           ClickHouseConfig
	Auth                 AuthConfig
	Secrets              SecretsConfig
	Keymint              KeymintConfig
	Models               ModelsConfig
	GuardLoop            GuardLoopConfig
	Score                ScoreProviderConfig
	TraceLens            TraceLensConfig
	Circuit              CircuitConfig
	Stripe               StripeConfig
	AggregateDefaultOrg  bool // E21: when true, ListByOrg("default") unions all tenants (forced false if Env=production)
}

type StripeConfig struct {
	Enabled        bool
	SecretKey      string
	WebhookSecret  string
	MeterEventName string
	OrgCustomers   string // "org:cus_xxx,org2:cus_yyy"
}

type ControlPlaneConfig struct {
	GRPCPort int
	HTTPPort int
}

type DataPlaneConfig struct {
	Mode          string
	HTTPPort      int
	ControlPlane  string
	ProxyTimeout  time.Duration
	MaxRetries    int
	Failover      bool
	RouteCacheTTL time.Duration
	AuthEnabled   bool // require Bearer on /v1/route and /v1/proxy/*
}

type EtcdConfig struct {
	Endpoints       []string
	Prefix          string
	AgentLeaseTTL   int
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type ClickHouseConfig struct {
	Addr     string
	Database string
	User     string
	Password string
}

type AuthConfig struct {
	JWTSecret         string
	AllowRegister     bool
	TokenTTLHours     int
	BootstrapEmail    string
	BootstrapPassword string
	DevAPIKey         string
}

type SecretsConfig struct {
	// MasterKey encrypts Keymint virtual-key bindings at rest in etcd (AES-GCM).
	MasterKey string
}

type KeymintConfig struct {
	BaseURL string
	APIKey  string
	TeamID  string
}

type ModelsConfig struct {
	CatalogPath string
}

type GuardLoopConfig struct {
	Enabled  bool
	BaseURL  string
	APIKey   string
	Timeout  time.Duration
	CacheTTL time.Duration
}

// ScoreProvider selects the score-based routing backend: auto|noop|fixture|guardloop.
type ScoreProviderConfig struct {
	Provider string
}

type TraceLensConfig struct {
	Enabled  bool
	Endpoint string
	JWT      string
}

type CircuitConfig struct {
	FailureThreshold int
	WindowSeconds    int
	HalfOpenSeconds  int
}

func Load() Config {
	env := getenv("ENV", "development")
	aggregateDefault := getenvBool("AGGREGATE_DEFAULT_ORG", true)
	if strings.EqualFold(env, "production") {
		aggregateDefault = false
	}
	return Config{
		Env:                 env,
		AggregateDefaultOrg: aggregateDefault,
		ControlPlane: ControlPlaneConfig{
			GRPCPort: getenvInt("CONTROL_PLANE_GRPC_PORT", 9090),
			HTTPPort: getenvInt("CONTROL_PLANE_HTTP_PORT", 8080),
		},
		DataPlane: DataPlaneConfig{
			Mode:          getenv("DATAPLANE_MODE", "gateway"),
			HTTPPort:      getenvInt("DATAPLANE_HTTP_PORT", 8081),
			ControlPlane:  getenv("CONTROL_PLANE_ADDR", "localhost:9090"),
			ProxyTimeout:  time.Duration(getenvInt("DATAPLANE_PROXY_TIMEOUT_MS", 30000)) * time.Millisecond,
			MaxRetries:    getenvInt("DATAPLANE_MAX_RETRIES", 2),
			Failover:      getenvBool("DATAPLANE_FAILOVER", true),
			RouteCacheTTL: time.Duration(getenvInt("DATAPLANE_ROUTE_CACHE_TTL_MS", 500)) * time.Millisecond,
			AuthEnabled:   getenvBool("DATAPLANE_AUTH_ENABLED", true),
		},
		Etcd: EtcdConfig{
			Endpoints:     strings.Split(getenv("ETCD_ENDPOINTS", "localhost:2379"), ","),
			Prefix:        getenv("ETCD_PREFIX", "/agentmesh"),
			AgentLeaseTTL: getenvInt("AGENTMESH_AGENT_LEASE_TTL", 30),
		},
		Redis: RedisConfig{
			Addr:     getenv("REDIS_ADDR", "localhost:6379"),
			Password: getenv("REDIS_PASSWORD", ""),
			DB:       getenvInt("REDIS_DB", 0),
		},
		ClickHouse: ClickHouseConfig{
			Addr:     getenv("CLICKHOUSE_ADDR", "localhost:9000"),
			Database: getenv("CLICKHOUSE_DATABASE", "agentmesh"),
			User:     getenv("CLICKHOUSE_USER", "default"),
			Password: getenv("CLICKHOUSE_PASSWORD", ""),
		},
		Auth: AuthConfig{
			JWTSecret:         getenv("AUTH_JWT_SECRET", getenv("AGENTMESH_SECRETS_KEY", "agentmesh-dev-jwt-secret-change-me!!")),
			AllowRegister:     getenvBool("AUTH_ALLOW_REGISTER", true),
			TokenTTLHours:     getenvInt("AUTH_TOKEN_TTL_HOURS", 24),
			BootstrapEmail:    getenv("AGENTMESH_BOOTSTRAP_EMAIL", ""),
			BootstrapPassword: getenv("AGENTMESH_BOOTSTRAP_PASSWORD", ""),
			DevAPIKey:         getenv("AGENTMESH_DEV_API_KEY", "agentmesh-dev-key"),
		},
		Secrets: SecretsConfig{
			MasterKey: getenv("AGENTMESH_SECRETS_KEY", getenv("AGENTMESH_DEV_API_KEY", "agentmesh-dev-key")),
		},
		Keymint: KeymintConfig{
			BaseURL: getenv("KEYMINT_BASE_URL", ""),
			APIKey:  getenv("KEYMINT_API_KEY", ""),
			TeamID:  getenv("KEYMINT_TEAM_ID", ""),
		},
		Models: ModelsConfig{
			CatalogPath: getenv("MODELS_CATALOG_PATH", ""),
		},
		GuardLoop: GuardLoopConfig{
			Enabled:  getenvBool("GUARDLOOP_ENABLED", false),
			BaseURL:  getenv("GUARDLOOP_BASE_URL", ""),
			APIKey:   getenv("GUARDLOOP_API_KEY", ""),
			Timeout:  time.Duration(getenvInt("GUARDLOOP_TIMEOUT_MS", 5000)) * time.Millisecond,
			CacheTTL: time.Duration(getenvInt("GUARDLOOP_CACHE_TTL_SEC", 60)) * time.Second,
		},
		Score: ScoreProviderConfig{
			Provider: getenv("SCORE_PROVIDER", "auto"),
		},
		TraceLens: TraceLensConfig{
			Enabled:  getenvBool("TRACELENS_ENABLED", true),
			Endpoint: getenv("TRACELENS_OTLP_ENDPOINT", "http://localhost:8080/v1/traces"),
			JWT:      getenv("TRACELENS_JWT", ""),
		},
		Circuit: CircuitConfig{
			FailureThreshold: getenvInt("CIRCUIT_FAILURE_THRESHOLD", 5),
			WindowSeconds:    getenvInt("CIRCUIT_WINDOW_SECONDS", 60),
			HalfOpenSeconds:  getenvInt("CIRCUIT_HALF_OPEN_SECONDS", 30),
		},
		Stripe: StripeConfig{
			Enabled:        getenvBool("STRIPE_ENABLED", false),
			SecretKey:      getenv("STRIPE_SECRET_KEY", ""),
			WebhookSecret:  getenv("STRIPE_WEBHOOK_SECRET", ""),
			MeterEventName: getenv("STRIPE_METER_EVENT_NAME", "agentmesh_routes"),
			OrgCustomers:   getenv("STRIPE_ORG_CUSTOMERS", ""),
		},
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getenvInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getenvBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	}
	return def
}
