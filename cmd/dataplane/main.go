package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	agentmeshv1 "github.com/agentmesh/agentmesh/gen/agentmesh/v1"
	"github.com/agentmesh/agentmesh/internal/analytics"
	"github.com/agentmesh/agentmesh/internal/circuit"
	"github.com/agentmesh/agentmesh/internal/config"
	"github.com/agentmesh/agentmesh/internal/dataplane"
	"github.com/agentmesh/agentmesh/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	cfg := config.Load()
	log, _ := zap.NewProduction()
	defer log.Sync()

	ctx := context.Background()
	shutdown, err := telemetry.InitTracer(ctx, cfg.TraceLens, "agentmesh-dataplane")
	if err != nil {
		log.Warn("tracer init", zap.Error(err))
	}
	defer func() { _ = shutdown(context.Background()) }()

	var ch *analytics.Store
	if s, err := analytics.NewStore(ctx, cfg.ClickHouse); err == nil {
		ch = s
		defer ch.Close()
	}

	var circ dataplane.CircuitRecorder = dataplane.NoopCircuit{}
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Warn("redis ping failed, dataplane circuit recording disabled", zap.Error(err))
		_ = rdb.Close()
	} else {
		br := circuit.New(rdb, "agentmesh", circuit.Config{
			FailureThreshold: cfg.Circuit.FailureThreshold,
			WindowSeconds:    cfg.Circuit.WindowSeconds,
			HalfOpenSeconds:  cfg.Circuit.HalfOpenSeconds,
		})
		circ = dataplane.BreakerAdapter{B: br}
		defer rdb.Close()
	}

	conn, err := grpc.Dial(cfg.DataPlane.ControlPlane, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("dial control plane", zap.Error(err))
	}
	defer conn.Close()
	cp := agentmeshv1.NewAgentMeshClient(conn)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/metrics", promhttp.Handler())

	mux.HandleFunc("/v1/route", func(w http.ResponseWriter, r *http.Request) {
		handleAdvisory(w, r, cp, ch)
	})

	mux.HandleFunc("/v1/proxy/", func(w http.ResponseWriter, r *http.Request) {
		handleProxy(w, r, cp, cfg, ch, circ, log)
	})

	addr := fmt.Sprintf(":%d", cfg.DataPlane.HTTPPort)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Info("dataplane listening",
			zap.String("addr", addr),
			zap.String("mode", cfg.DataPlane.Mode),
			zap.Bool("failover", cfg.DataPlane.Failover),
			zap.Int("max_retries", cfg.DataPlane.MaxRetries),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("serve", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shCtx)
}

func handleAdvisory(w http.ResponseWriter, r *http.Request, cp agentmeshv1.AgentMeshClient, ch *analytics.Store) {
	start := time.Now()
	org := headerOrg(r)
	var payload agentmeshv1.RoutePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if payload.OrgId == "" {
		payload.OrgId = org
	}
	ctx, span := telemetry.StartRouteSpan(r.Context(), org, payload.SourceAgentId, "", payload.RoutingStrategy)
	defer span.End()

	resp, err := cp.RouteRequest(ctx, &payload)
	if err != nil {
		telemetry.ObserveRoute(org, "advisory", "error", time.Since(start))
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	telemetry.ObserveRoute(org, "advisory", "ok", time.Since(start))
	recordCH(ch, org, resp, payload, time.Since(start), true, 0)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func handleProxy(w http.ResponseWriter, r *http.Request, cp agentmeshv1.AgentMeshClient, cfg config.Config, ch *analytics.Store, circ dataplane.CircuitRecorder, log *zap.Logger) {
	start := time.Now()
	org := headerOrg(r)
	var payload agentmeshv1.RoutePayload
	body, _ := io.ReadAll(r.Body)
	_ = json.Unmarshal(body, &payload)
	if payload.OrgId == "" {
		payload.OrgId = org
	}
	ctx, span := telemetry.StartRouteSpan(r.Context(), org, payload.SourceAgentId, "", payload.RoutingStrategy)
	defer span.End()

	resp, err := cp.RouteRequest(ctx, &payload)
	if err != nil {
		telemetry.ObserveRoute(org, "proxy", "error", time.Since(start))
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	candidates := dataplane.CandidatesFromRoute(resp)
	pathSuffix := strings.TrimPrefix(r.URL.Path, "/v1/proxy")
	maxRetries := cfg.DataPlane.MaxRetries
	if resp.GetMaxRetries() > 0 {
		maxRetries = int(resp.GetMaxRetries())
	}

	result, err := dataplane.ProxyWithFailover(ctx, r.Method, body, r.Header.Get("Content-Type"), candidates, dataplane.ProxyConfig{
		Timeout:    cfg.DataPlane.ProxyTimeout,
		MaxRetries: maxRetries,
		Failover:   cfg.DataPlane.Failover,
		PathSuffix: pathSuffix,
		Headers:    resp.GetInjectedHeaders(),
		OrgID:      org,
		Circuit:    circ,
	})
	if err != nil {
		telemetry.ObserveRoute(org, "proxy", "error", time.Since(start))
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	retries := uint8(0)
	if result.Attempts > 0 {
		retries = uint8(result.Attempts - 1)
	}
	ok := result.Status > 0 && result.Status < 400
	if result.Status >= 500 || result.Status == 0 {
		telemetry.ObserveRoute(org, "proxy", "error", time.Since(start))
		recordCH(ch, org, resp, payload, time.Since(start), false, retries)
		log.Warn("proxy upstream failed",
			zap.String("org", org),
			zap.Strings("failed_agents", result.FailedAgents),
			zap.Int("attempts", result.Attempts),
			zap.Bool("failover", cfg.DataPlane.Failover),
		)
		for k, vals := range result.Header {
			for _, v := range vals {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(result.Status)
		_, _ = w.Write(result.Body)
		return
	}

	// Prefer actual upstream destination after failover for analytics.
	if result.DestinationID != "" {
		resp.DestinationAgentId = result.DestinationID
	}
	for k, vals := range result.Header {
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	if result.Attempts > 1 {
		w.Header().Set("X-Agentmesh-Failover-Attempts", fmt.Sprintf("%d", result.Attempts))
		w.Header().Set("X-Agentmesh-Destination", result.DestinationID)
	}
	w.WriteHeader(result.Status)
	_, _ = w.Write(result.Body)
	telemetry.ObserveRoute(org, "proxy", "ok", time.Since(start))
	recordCH(ch, org, resp, payload, time.Since(start), ok, retries)
}

func headerOrg(r *http.Request) string {
	if o := r.Header.Get("X-Org-ID"); o != "" {
		return o
	}
	if o := r.Header.Get("X-Workspace-ID"); o != "" {
		return o
	}
	return "default"
}

func recordCH(ch *analytics.Store, org string, resp *agentmeshv1.RouteResponse, req agentmeshv1.RoutePayload, d time.Duration, ok bool, retries uint8) {
	if ch == nil {
		return
	}
	_ = ch.InsertRoute(context.Background(), analytics.RouteEvent{
		OrgID:              org,
		TraceID:            resp.TraceContext,
		SourceAgentID:      req.SourceAgentId,
		DestinationAgentID: resp.DestinationAgentId,
		RoutingStrategy:    req.RoutingStrategy,
		LatencyMs:          uint32(d.Milliseconds()),
		Success:            ok,
		RetryCount:         retries,
	})
}
