package analytics

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/agentmesh/agentmesh/internal/config"
)

type Store struct {
	conn clickhouse.Conn
}

func NewStore(ctx context.Context, cfg config.ClickHouseConfig) (*Store, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.Addr},
		Auth: clickhouse.Auth{
			Database: cfg.Database,
			Username: cfg.User,
			Password: cfg.Password,
		},
	})
	if err != nil {
		return nil, err
	}
	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}
	s := &Store{conn: conn}
	if err := s.initSchema(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) initSchema(ctx context.Context) error {
	ddl := `
CREATE TABLE IF NOT EXISTS routes (
  org_id LowCardinality(String),
  trace_id String,
  source_agent_id String,
  destination_agent_id String,
  routing_strategy LowCardinality(String),
  latency_ms UInt32,
  token_cost_usd Float64,
  success UInt8,
  retry_count UInt8,
  recorded_at DateTime64(3)
) ENGINE = MergeTree()
PARTITION BY toStartOfDay(recorded_at)
ORDER BY (org_id, recorded_at, trace_id);
CREATE TABLE IF NOT EXISTS agent_metrics (
  org_id LowCardinality(String),
  agent_id String,
  agent_type LowCardinality(String),
  latency_ewma_ms Float64,
  success_rate Float64,
  updated_at DateTime64(3)
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY (org_id, agent_id);
CREATE TABLE IF NOT EXISTS billing_events (
  org_id LowCardinality(String),
  event_type LowCardinality(String),
  quantity Float64,
  stripe_event_id String,
  recorded_at DateTime64(3)
) ENGINE = MergeTree()
ORDER BY (org_id, recorded_at);
`
	return s.conn.Exec(ctx, ddl)
}

type RouteEvent struct {
	OrgID              string
	TraceID            string
	SourceAgentID      string
	DestinationAgentID string
	RoutingStrategy    string
	LatencyMs          uint32
	TokenCostUSD       float64
	Success            bool
	RetryCount         uint8
}

func (s *Store) InsertRoute(ctx context.Context, e RouteEvent) error {
	return s.conn.Exec(ctx, `INSERT INTO routes (
		org_id, trace_id, source_agent_id, destination_agent_id,
		routing_strategy, latency_ms, token_cost_usd, success, retry_count, recorded_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.OrgID, e.TraceID, e.SourceAgentID, e.DestinationAgentID,
		e.RoutingStrategy, e.LatencyMs, e.TokenCostUSD, boolToU8(e.Success), e.RetryCount, time.Now().UTC())
}

func boolToU8(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

func (s *Store) Close() error {
	return s.conn.Close()
}

func DSN(cfg config.ClickHouseConfig) string {
	return fmt.Sprintf("clickhouse://%s:%s@%s/%s", cfg.User, cfg.Password, cfg.Addr, cfg.Database)
}
