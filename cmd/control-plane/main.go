package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	agentmeshv1 "github.com/agentmesh/agentmesh/gen/agentmesh/v1"
	"github.com/agentmesh/agentmesh/internal/analytics"
	"github.com/agentmesh/agentmesh/internal/auth"
	"github.com/agentmesh/agentmesh/internal/billing"
	"github.com/agentmesh/agentmesh/internal/circuit"
	"github.com/agentmesh/agentmesh/internal/config"
	"github.com/agentmesh/agentmesh/internal/grpcserver"
	"github.com/agentmesh/agentmesh/internal/policy"
	"github.com/agentmesh/agentmesh/internal/ratelimit"
	"github.com/agentmesh/agentmesh/internal/keymint"
	"github.com/agentmesh/agentmesh/internal/models"
	"github.com/agentmesh/agentmesh/internal/registry"
	"github.com/agentmesh/agentmesh/internal/rest"
	"github.com/agentmesh/agentmesh/internal/router"
	"github.com/agentmesh/agentmesh/internal/score"
	"github.com/agentmesh/agentmesh/internal/score/guardloop"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()
	log, _ := zap.NewProduction()
	defer log.Sync()

	ctx := context.Background()
	reg, err := registry.NewStore(cfg.Etcd.Endpoints, cfg.Etcd.Prefix, cfg.Etcd.AgentLeaseTTL)
	if err != nil {
		log.Fatal("etcd", zap.Error(err))
	}
	defer reg.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Warn("redis ping failed, circuit breaker may fail", zap.Error(err))
	}

	gl := guardloop.NewClient(cfg.GuardLoop)
	scoreName := score.ResolveProviderName(cfg.Score.Provider, gl.Enabled())
	var scoreProvider score.Provider
	switch scoreName {
	case "guardloop":
		scoreProvider = gl
	case "noop":
		scoreProvider = score.Noop{}
	default:
		scoreProvider = score.Fixture{}
		scoreName = "fixture"
	}
	log.Info("score provider", zap.String("provider", scoreName), zap.Bool("guardloop_enabled", gl.Enabled()))
	engine := router.NewEngine(scoreProvider)
	br := circuit.New(rdb, "agentmesh", circuit.Config{
		FailureThreshold: cfg.Circuit.FailureThreshold,
		WindowSeconds:    cfg.Circuit.WindowSeconds,
		HalfOpenSeconds:  cfg.Circuit.HalfOpenSeconds,
	})

	var chStore *analytics.Store
	if s, err := analytics.NewStore(ctx, cfg.ClickHouse); err != nil {
		log.Warn("clickhouse unavailable", zap.Error(err))
	} else {
		chStore = s
		defer chStore.Close()
	}
	meter := &billing.Meter{
		CH: chStore,
		Stripe: billing.StripeHooks{
			Enabled:        cfg.Stripe.Enabled,
			SecretKey:      cfg.Stripe.SecretKey,
			WebhookSecret:  cfg.Stripe.WebhookSecret,
			MeterEventName: cfg.Stripe.MeterEventName,
			OrgCustomers:   billing.ParseOrgCustomers(cfg.Stripe.OrgCustomers),
		},
	}
	if cfg.Stripe.Enabled {
		log.Info("stripe metering enabled", zap.String("meter", cfg.Stripe.MeterEventName))
	}

	policies, err := policy.NewStore(cfg.Etcd.Endpoints, cfg.Etcd.Prefix, cfg)
	if err != nil {
		log.Fatal("policy store", zap.Error(err))
	}
	defer policies.Close()
	policies.Watch(ctx)

	limiter := ratelimit.New(rdb, "agentmesh")

	authStore, err := auth.NewStore(cfg.Etcd.Endpoints, cfg.Etcd.Prefix)
	if err != nil {
		log.Fatal("auth store", zap.Error(err))
	}
	defer authStore.Close()
	issuer, err := auth.NewTokenIssuer(cfg.Auth.JWTSecret, cfg.Auth.TokenTTLHours)
	if err != nil {
		log.Fatal("auth jwt", zap.Error(err))
	}
	authSvc := &auth.Service{
		Store:          authStore,
		Tokens:         issuer,
		AllowRegister:  cfg.Auth.AllowRegister,
		DevAPIKeyPlain: cfg.Auth.DevAPIKey,
	}
	if err := authSvc.Bootstrap(ctx, cfg.Auth.BootstrapEmail, cfg.Auth.BootstrapPassword); err != nil {
		log.Warn("auth bootstrap", zap.Error(err))
	} else {
		log.Info("auth ready", zap.Bool("allow_register", cfg.Auth.AllowRegister))
	}

	bindStore, err := keymint.NewBindStore(cfg.Etcd.Endpoints, cfg.Etcd.Prefix, cfg.Secrets.MasterKey)
	if err != nil {
		log.Fatal("keymint bind store", zap.Error(err))
	}
	defer bindStore.Close()
	kmClient := keymint.NewClient(cfg.Keymint.BaseURL, cfg.Keymint.APIKey, cfg.Keymint.TeamID)
	kmFacade := &keymint.Facade{Client: kmClient, Binds: bindStore}
	if kmClient.Configured() {
		log.Info("keymint configured", zap.String("base_url", cfg.Keymint.BaseURL))
	} else {
		log.Warn("keymint not configured; Models/Chat require KEYMINT_BASE_URL, KEYMINT_API_KEY, KEYMINT_TEAM_ID")
	}

	catalogPath := models.ResolvePath(cfg.Models.CatalogPath)
	catalog, err := models.Load(catalogPath)
	if err != nil {
		log.Fatal("models catalog", zap.String("path", catalogPath), zap.Error(err))
	}
	log.Info("models catalog loaded", zap.String("path", catalog.Path()), zap.Int("models", len(catalog.Models())), zap.Int("providers", len(catalog.Providers())))

	srv := grpcserver.New(reg, policies, engine, br, limiter, cfg, meter)

	grpcServer := grpc.NewServer()
	agentmeshv1.RegisterAgentMeshServer(grpcServer, srv)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.ControlPlane.GRPCPort))
	if err != nil {
		log.Fatal("listen grpc", zap.Error(err))
	}
	go func() {
		log.Info("control plane gRPC", zap.Int("port", cfg.ControlPlane.GRPCPort))
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal("grpc serve", zap.Error(err))
		}
	}()

	gw := rest.NewGateway(fmt.Sprintf("localhost:%d", cfg.ControlPlane.GRPCPort), authSvc, policies, reg, kmFacade, catalog)
	httpSrv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.ControlPlane.HTTPPort),
		Handler: mountHTTP(gw.Routes()),
	}
	go func() {
		log.Info("control plane HTTP", zap.Int("port", cfg.ControlPlane.HTTPPort))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("http", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shCtx)
	grpcServer.GracefulStop()
}

func mountHTTP(r http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", r)
	return mux
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
