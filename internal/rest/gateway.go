package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	agentmeshv1 "github.com/agentmesh/agentmesh/gen/agentmesh/v1"
	"github.com/agentmesh/agentmesh/internal/auth"
	"github.com/agentmesh/agentmesh/internal/grpcserver"
	"github.com/agentmesh/agentmesh/internal/inference"
	"github.com/agentmesh/agentmesh/internal/keymint"
	"github.com/agentmesh/agentmesh/internal/models"
	"github.com/agentmesh/agentmesh/internal/policy"
	"github.com/agentmesh/agentmesh/internal/registry"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Gateway struct {
	grpcAddr string
	auth     *auth.Service
	policies *policy.Store
	registry *registry.Store
	keymint  *keymint.Facade
	infer    *inference.Client
	catalog  *models.Catalog
}

func NewGateway(grpcAddr string, authSvc *auth.Service, policies *policy.Store, reg *registry.Store, km *keymint.Facade, catalog *models.Catalog) *Gateway {
	return &Gateway{
		grpcAddr: grpcAddr,
		auth:     authSvc,
		policies: policies,
		registry: reg,
		keymint:  km,
		infer:    inference.NewClient(),
		catalog:  catalog,
	}
}

func (g *Gateway) Routes() chi.Router {
	r := chi.NewRouter()
	r.Use(corsMiddleware)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/ready", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	r.Route("/v1", func(v1 chi.Router) {
		v1.Post("/auth/register", g.registerAuth)
		v1.Post("/auth/login", g.loginAuth)

		v1.Group(func(protected chi.Router) {
			protected.Use(g.authMiddleware)
			protected.Post("/auth/logout", g.logoutAuth)
			protected.Get("/auth/me", g.meAuth)
			protected.Post("/orgs", g.createOrg)
			protected.Get("/orgs", g.listAuthOrgs)
			protected.Post("/orgs/{org}/keys", g.createOrgKey)
			protected.Get("/orgs/{org}/keys", g.listOrgKeys)
			protected.Delete("/orgs/{org}/keys/{key_id}", g.revokeOrgKey)

			protected.Post("/agents/register", g.withRole(auth.RoleOperator, g.registerAgent))
			protected.Get("/agents/discover", g.withRole(auth.RoleViewer, g.discoverAgents))
			protected.Post("/agents/health", g.withRole(auth.RoleOperator, g.reportHealth))
			protected.Post("/route", g.withRole(auth.RoleViewer, g.route))
			protected.Get("/circuit/{agent_id}", g.withRole(auth.RoleViewer, g.circuitState))
			protected.Get("/policy", g.withRole(auth.RoleViewer, g.getPolicy))
			protected.Put("/policy", g.withRole(auth.RoleAdmin, g.putPolicy))
			protected.Get("/models", g.withRole(auth.RoleViewer, g.listModels))
			protected.Get("/models/providers", g.withRole(auth.RoleViewer, g.listProviders))
			protected.Get("/secrets/providers", g.withRole(auth.RoleViewer, g.listSecrets))
			protected.Put("/secrets/providers/{provider}", g.withRole(auth.RoleOperator, g.putSecret))
			protected.Delete("/secrets/providers/{provider}", g.withRole(auth.RoleOperator, g.deleteSecret))
			protected.Get("/secrets/keymint", g.withRole(auth.RoleViewer, g.keymintStatus))
			protected.Post("/chat", g.withRole(auth.RoleViewer, g.chat))
			protected.Post("/chat/stream", g.withRole(auth.RoleViewer, g.chatStream))
		})
	})
	return r
}

func (g *Gateway) withRole(min auth.Role, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFrom(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errBody("UNAUTHORIZED", "not authenticated"))
			return
		}
		if err := auth.RequireRole(p, min); err != nil {
			writeJSON(w, http.StatusForbidden, errBody("FORBIDDEN", "insufficient role"))
			return
		}
		h(w, r)
	}
}

func (g *Gateway) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := g.auth.Authenticate(r.Context(),
			r.Header.Get("X-Org-ID"),
			r.Header.Get("X-Workspace-ID"),
			r.Header.Get("Authorization"))
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "UNAUTHORIZED", "message": err.Error()}})
			return
		}
		ctx := auth.WithPrincipal(r.Context(), p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (g *Gateway) dial(ctx context.Context) (agentmeshv1.AgentMeshClient, *grpc.ClientConn, error) {
	conn, err := grpc.DialContext(ctx, g.grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return agentmeshv1.NewAgentMeshClient(conn), conn, nil
}

func orgFromCtx(r *http.Request) string {
	if p, ok := auth.PrincipalFrom(r.Context()); ok {
		return p.OrgID
	}
	return "default"
}

func (g *Gateway) registerAgent(w http.ResponseWriter, r *http.Request) {
	var body agentmeshv1.AgentRegistration
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_JSON", err.Error()))
		return
	}
	if body.OrgId == "" {
		body.OrgId = orgFromCtx(r)
	}
	client, conn, err := g.dial(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL", err.Error()))
		return
	}
	defer conn.Close()
	ack, err := client.RegisterAgent(r.Context(), &body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("REGISTER_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, ack)
}

func (g *Gateway) discoverAgents(w http.ResponseWriter, r *http.Request) {
	q := &agentmeshv1.DiscoveryQuery{
		OrgId:           orgFromCtx(r),
		Capability:      r.URL.Query().Get("capability"),
		RoutingStrategy: r.URL.Query().Get("routing_strategy"),
	}
	if mc := r.URL.Query().Get("max_cost"); mc != "" {
		if f, err := strconv.ParseFloat(mc, 64); err == nil {
			q.MaxCost = f
		}
	}
	if lim := r.URL.Query().Get("limit"); lim != "" {
		if n, err := strconv.Atoi(lim); err == nil && n > 0 {
			q.Limit = int32(n)
		}
	}
	if orgQ := strings.TrimSpace(r.URL.Query().Get("org_id")); orgQ != "" {
		q.OrgId = orgQ
	}
	iu := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("include_unhealthy")))
	q.IncludeUnhealthy = iu == "1" || iu == "true" || iu == "yes"
	client, conn, err := g.dial(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL", err.Error()))
		return
	}
	defer conn.Close()
	stream, err := client.DiscoverAgents(r.Context(), q)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("DISCOVER_FAILED", err.Error()))
		return
	}
	var endpoints []map[string]any
	for {
		ep, err := stream.Recv()
		if err != nil {
			break
		}
		endpoints = append(endpoints, map[string]any{
			"agent_id":             ep.GetAgentId(),
			"agent_type":           ep.GetAgentType(),
			"endpoint_url":         ep.GetEndpointUrl(),
			"capabilities":         ep.GetCapabilities(),
			"estimated_latency_ms": ep.GetEstimatedLatencyMs(),
			"success_rate":         ep.GetSuccessRate(),
			"cost_score":           ep.GetCostScore(),
			"rank_score":           ep.GetRankScore(),
			"org_id":               ep.GetOrgId(),
			"healthy":              ep.GetHealthy(),
			"persistent":           ep.GetPersistent(),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": endpoints})
}

func (g *Gateway) reportHealth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AgentID string `json:"agent_id"`
		Healthy bool   `json:"healthy"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_JSON", err.Error()))
		return
	}
	if body.AgentID == "" {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_ARGUMENT", "agent_id required"))
		return
	}
	org := orgFromCtx(r)
	if err := g.registry.UpdateHealth(r.Context(), org, body.AgentID, body.Healthy, body.Message); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("HEALTH_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"accepted": true})
}

func (g *Gateway) listOrgs(w http.ResponseWriter, r *http.Request) {
	ids, err := g.registry.ListOrgIDs(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("ORGS_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"orgs": ids})
}

func (g *Gateway) route(w http.ResponseWriter, r *http.Request) {
	var body agentmeshv1.RoutePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_JSON", err.Error()))
		return
	}
	if body.OrgId == "" {
		body.OrgId = orgFromCtx(r)
	}
	client, conn, err := g.dial(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL", err.Error()))
		return
	}
	defer conn.Close()
	resp, err := client.RouteRequest(r.Context(), &body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("ROUTE_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (g *Gateway) circuitState(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agent_id")
	client, conn, err := g.dial(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL", err.Error()))
		return
	}
	defer conn.Close()
	st, err := client.GetCircuitState(r.Context(), &agentmeshv1.CircuitQuery{
		OrgId:              orgFromCtx(r),
		DestinationAgentId: agentID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("CIRCUIT_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (g *Gateway) getPolicy(w http.ResponseWriter, r *http.Request) {
	org := orgFromCtx(r)
	p, err := g.policies.Get(r.Context(), org)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("POLICY_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (g *Gateway) putPolicy(w http.ResponseWriter, r *http.Request) {
	org := orgFromCtx(r)
	var body policy.OrgPolicy
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_JSON", err.Error()))
		return
	}
	body.OrgID = org
	if err := g.policies.Put(r.Context(), body); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("POLICY_PUT_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func errBody(code, msg string) map[string]any {
	return map[string]any{"error": map[string]string{"code": code, "message": msg}}
}

var _ = grpcserver.NowUnix
