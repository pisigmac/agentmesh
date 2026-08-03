package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	agentmeshv1 "github.com/agentmesh/agentmesh/gen/agentmesh/v1"
	"github.com/agentmesh/agentmesh/internal/analytics"
	"github.com/agentmesh/agentmesh/internal/config"
	"github.com/agentmesh/agentmesh/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	// Advisory routing
	mux.HandleFunc("/v1/route", func(w http.ResponseWriter, r *http.Request) {
		handleAdvisory(w, r, cp, cfg, ch, false)
	})

	// Full proxy: POST /v1/proxy/route then forwards to chosen endpoint
	mux.HandleFunc("/v1/proxy/", func(w http.ResponseWriter, r *http.Request) {
		handleProxy(w, r, cp, cfg, ch)
	})

	addr := fmt.Sprintf(":%d", cfg.DataPlane.HTTPPort)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		log.Info("dataplane listening", zap.String("addr", addr), zap.String("mode", cfg.DataPlane.Mode))
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

func handleAdvisory(w http.ResponseWriter, r *http.Request, cp agentmeshv1.AgentMeshClient, cfg config.Config, ch *analytics.Store, proxy bool) {
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
	if proxy {
		target, _ := url.Parse(resp.EndpointUrl)
		for k, v := range resp.InjectedHeaders {
			r.Header.Set(k, v)
		}
		proxyTo(w, r, target, cfg)
		return
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func handleProxy(w http.ResponseWriter, r *http.Request, cp agentmeshv1.AgentMeshClient, cfg config.Config, ch *analytics.Store) {
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
	target, err := url.Parse(resp.EndpointUrl)
	if err != nil {
		http.Error(w, "bad endpoint", http.StatusBadGateway)
		return
	}
	pathSuffix := strings.TrimPrefix(r.URL.Path, "/v1/proxy")
	target.Path = strings.TrimSuffix(target.Path, "/") + pathSuffix
	r.Body = io.NopCloser(bytes.NewReader(body))
	for k, v := range resp.InjectedHeaders {
		r.Header.Set(k, v)
	}
	var retries uint8
	for attempt := 0; attempt <= cfg.DataPlane.MaxRetries; attempt++ {
		retries = uint8(attempt)
		rec := httptestRecorder{header: make(http.Header)}
		proxyTo(&rec, r, target, cfg)
		if rec.status < 500 {
			for k, vals := range rec.header {
				for _, v := range vals {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(rec.status)
			_, _ = w.Write(rec.body)
			telemetry.ObserveRoute(org, "proxy", "ok", time.Since(start))
			recordCH(ch, org, resp, payload, time.Since(start), rec.status < 400, retries)
			return
		}
	}
	telemetry.ObserveRoute(org, "proxy", "error", time.Since(start))
	recordCH(ch, org, resp, payload, time.Since(start), false, retries)
	http.Error(w, "upstream failed after retries", http.StatusBadGateway)
}

type httptestRecorder struct {
	status int
	body   []byte
	header http.Header
}

func (r *httptestRecorder) Header() http.Header { return r.header }
func (r *httptestRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	if r.status == 0 {
		r.status = 200
	}
	return len(b), nil
}
func (r *httptestRecorder) WriteHeader(statusCode int) { r.status = statusCode }

func proxyTo(w http.ResponseWriter, r *http.Request, target *url.URL, cfg config.Config) {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		ResponseHeaderTimeout: cfg.DataPlane.ProxyTimeout,
	}
	r.URL.Host = target.Host
	r.URL.Scheme = target.Scheme
	r.Host = target.Host
	proxy.ServeHTTP(w, r)
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
