package grpcserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	agentmeshv1 "github.com/agentmesh/agentmesh/gen/agentmesh/v1"
	"github.com/agentmesh/agentmesh/internal/circuit"
	"github.com/agentmesh/agentmesh/internal/config"
	"github.com/agentmesh/agentmesh/internal/policy"
	"github.com/agentmesh/agentmesh/internal/ratelimit"
	"github.com/agentmesh/agentmesh/internal/registry"
	"github.com/agentmesh/agentmesh/internal/router"
	"github.com/agentmesh/agentmesh/internal/telemetry"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	agentmeshv1.UnimplementedAgentMeshServer
	reg      *registry.Store
	policies *policy.Store
	engine   *router.Engine
	breaker  *circuit.Breaker
	limiter  *ratelimit.Limiter
	cfg      config.Config
}

func New(reg *registry.Store, policies *policy.Store, engine *router.Engine, breaker *circuit.Breaker, limiter *ratelimit.Limiter, cfg config.Config) *Server {
	return &Server{reg: reg, policies: policies, engine: engine, breaker: breaker, limiter: limiter, cfg: cfg}
}

func (s *Server) RegisterAgent(ctx context.Context, req *agentmeshv1.AgentRegistration) (*agentmeshv1.RegistrationAck, error) {
	if req.GetOrgId() == "" || req.GetAgentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "org_id and agent_id required")
	}
	pol, err := s.policies.Get(ctx, req.GetOrgId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "policy: %v", err)
	}
	if ok, _ := s.limiter.Allow(ctx, req.GetOrgId(), "register", pol.RateLimit.RegisterPerMinute, time.Minute); !ok {
		return nil, status.Error(codes.ResourceExhausted, "register rate limit exceeded")
	}
	rec := registry.AgentRecord{
		OrgID:        req.GetOrgId(),
		AgentID:      req.GetAgentId(),
		AgentType:    req.GetAgentType(),
		Capabilities: append([]string(nil), req.GetCapabilities()...),
		EndpointURL:  req.GetEndpointUrl(),
		Healthy:      req.GetHealth().GetHealthy(),
		Metadata:     req.GetMetadata(),
	}
	if cp := req.GetCostProfile(); cp != nil {
		rec.CostPerReq = cp.GetCostPerRequestUsd()
		rec.CostPer1K = cp.GetCostPer_1KTokensUsd()
	}
	ttl, err := s.reg.Register(ctx, rec)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register: %v", err)
	}
	return &agentmeshv1.RegistrationAck{
		AgentId:         rec.AgentID,
		LeaseTtlSeconds: ttl,
		Message:         "registered",
	}, nil
}

func (s *Server) DiscoverAgents(req *agentmeshv1.DiscoveryQuery, stream agentmeshv1.AgentMesh_DiscoverAgentsServer) error {
	ctx := stream.Context()
	strategy := req.GetRoutingStrategy()
	pol, _ := s.policies.Get(ctx, req.GetOrgId())
	if strategy == "" {
		strategy = pol.Routing.DefaultStrategy
	}
	maxCost := req.GetMaxCost()
	if maxCost == 0 {
		maxCost = pol.Routing.EffectiveMaxCost()
	}
	maxLatency := pol.Routing.MaxLatencyMs
	agents, err := s.reg.ListByOrg(ctx, req.GetOrgId())
	if err != nil {
		return status.Errorf(codes.Internal, "list: %v", err)
	}
	limit := int(req.GetLimit())
	if limit <= 0 {
		if req.GetCapability() == "" {
			limit = 256
		} else {
			limit = 50
		}
	}

	if req.GetIncludeUnhealthy() {
		// Catalog listing with optional SLO filters (cost/latency) and soft preferences.
		var catalog []registry.AgentRecord
		for _, a := range agents {
			if req.GetCapability() != "" && !registry.HasCapability(a, req.GetCapability()) {
				continue
			}
			if !router.MeetsSLOs(a, maxCost, maxLatency) {
				continue
			}
			catalog = append(catalog, a)
		}
		ranked := make([]router.Ranked, len(catalog))
		for i, a := range catalog {
			ranked[i] = router.Ranked{Agent: a, Score: 0}
		}
		ranked = router.PreferSLO(ranked, pol.Routing.PreferFree, pol.Routing.PreferLocal)
		for i, r := range ranked {
			if i >= limit {
				break
			}
			if err := stream.Send(toEndpoint(r.Agent, r.Score)); err != nil {
				return err
			}
		}
		return nil
	}

	ranked := s.engine.Rank(router.RankInput{
		OrgID:        req.GetOrgId(),
		Capability:   req.GetCapability(),
		MaxCost:      maxCost,
		MaxLatencyMs: maxLatency,
		PreferFree:   pol.Routing.PreferFree,
		PreferLocal:  pol.Routing.PreferLocal,
		Strategy:     router.Strategy(strategy),
		Agents:       agents,
	})
	for i, r := range ranked {
		if i >= limit {
			break
		}
		ep := toEndpoint(r.Agent, r.Score)
		if err := stream.Send(ep); err != nil {
			return err
		}
	}
	return nil
}

func toEndpoint(a registry.AgentRecord, rankScore float64) *agentmeshv1.AgentEndpoint {
	return &agentmeshv1.AgentEndpoint{
		AgentId:            a.AgentID,
		AgentType:          a.AgentType,
		EndpointUrl:        a.EndpointURL,
		Capabilities:       a.Capabilities,
		EstimatedLatencyMs: a.LatencyEWMA,
		SuccessRate:        a.SuccessRate,
		CostScore:          a.CostPerReq,
		RankScore:          rankScore,
		OrgId:              a.OrgID,
		Healthy:            a.Healthy,
		Persistent:         registry.IsPersistentRegistration(a),
	}
}

func (s *Server) RouteRequest(ctx context.Context, req *agentmeshv1.RoutePayload) (*agentmeshv1.RouteResponse, error) {
	pol, err := s.policies.Get(ctx, req.GetOrgId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "policy: %v", err)
	}
	if ok, _ := s.limiter.Allow(ctx, req.GetOrgId(), "route", pol.RateLimit.RoutesPerMinute, time.Minute); !ok {
		return nil, status.Error(codes.ResourceExhausted, "org route rate limit exceeded")
	}
	if req.GetSourceAgentId() != "" {
		key := "route:" + req.GetSourceAgentId()
		if ok, _ := s.limiter.Allow(ctx, req.GetOrgId(), key, pol.RateLimit.PerAgentRoutesPerMinute, time.Minute); !ok {
			return nil, status.Error(codes.ResourceExhausted, "agent route rate limit exceeded")
		}
	}
	strategy := req.GetRoutingStrategy()
	if strategy == "" {
		strategy = pol.Routing.DefaultStrategy
	}
	agents, err := s.reg.ListByOrg(ctx, req.GetOrgId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list: %v", err)
	}

	pinned := false
	var ranked []router.Ranked

	maxCost := pol.Routing.EffectiveMaxCost()
	maxLatency := pol.Routing.MaxLatencyMs

	if pinID := strings.TrimSpace(req.GetDestinationAgentId()); pinID != "" {
		rec := router.FindAgent(agents, pinID)
		if rec == nil {
			return nil, status.Errorf(codes.NotFound, "pinned agent %s not found", pinID)
		}
		if !req.GetForcePin() {
			if !rec.Healthy {
				return nil, status.Errorf(codes.FailedPrecondition, "pinned agent %s is unhealthy", pinID)
			}
			if req.GetCapability() != "" && !registry.HasCapability(*rec, req.GetCapability()) {
				return nil, status.Errorf(codes.InvalidArgument, "pinned agent %s lacks capability %s", pinID, req.GetCapability())
			}
			if !router.MeetsSLOs(*rec, maxCost, maxLatency) {
				telemetry.ObserveSLOReject(req.GetOrgId(), sloRejectReason(maxCost, maxLatency, *rec))
				return nil, status.Errorf(codes.FailedPrecondition, "pinned agent %s exceeds org cost/latency SLO", pinID)
			}
		}
		cbCfg := policy.ResolveCircuit(pol, rec.AgentID, rec.AgentType)
		ok, _, cbErr := s.breaker.AllowRequestWithConfig(ctx, req.GetOrgId(), rec.AgentID, cbCfg)
		if cbErr == nil && !ok {
			if !req.GetPinFailover() {
				return nil, status.Errorf(codes.Unavailable, "circuit open for pinned agent %s", pinID)
			}
			// Fall through to normal ranking.
		} else {
			ranked = []router.Ranked{{Agent: *rec, Score: 1e9}}
			pinned = true
			strategy = "pin"
		}
	}

	if !pinned {
		var stats router.FilterStats
		ranked = s.engine.Rank(router.RankInput{
			OrgID:        req.GetOrgId(),
			Capability:   req.GetCapability(),
			MaxCost:      maxCost,
			MaxLatencyMs: maxLatency,
			PreferFree:   pol.Routing.PreferFree,
			PreferLocal:  pol.Routing.PreferLocal,
			Strategy:     router.Strategy(strategy),
			Agents:       agents,
			Stats:        &stats,
		})
		ranked = router.FilterByFallbackTypes(ranked, req.GetFallbackAgentTypes())
		ranked = router.PreferAgentIDs(ranked, req.GetPreferAgentIds())
		if len(ranked) == 0 {
			if stats.RejectedCost > 0 || stats.RejectedLatency > 0 {
				reason := "slo_filter"
				if stats.RejectedCost > 0 && stats.RejectedLatency == 0 {
					reason = "max_cost"
				} else if stats.RejectedLatency > 0 && stats.RejectedCost == 0 {
					reason = "max_latency"
				}
				telemetry.ObserveSLOReject(req.GetOrgId(), reason)
				return nil, status.Error(codes.FailedPrecondition, "no route: all candidates exceed org cost/latency SLO")
			}
			return nil, status.Error(codes.NotFound, "no route")
		}
	}

	var chosen router.Ranked
	for _, r := range ranked {
		cbCfg := policy.ResolveCircuit(pol, r.Agent.AgentID, r.Agent.AgentType)
		ok, _, err := s.breaker.AllowRequestWithConfig(ctx, req.GetOrgId(), r.Agent.AgentID, cbCfg)
		if err != nil || ok {
			chosen = r
			break
		}
	}
	if chosen.Agent.AgentID == "" {
		return nil, status.Error(codes.Unavailable, "circuit open for all candidates")
	}
	traceID := uuid.NewString()
	headers := map[string]string{
		"x-agentmesh-trace-id":         traceID,
		"x-agentmesh-source-agent":     req.GetSourceAgentId(),
		"x-agentmesh-dest-agent":       chosen.Agent.AgentID,
		"x-agentmesh-routing-strategy": strategy,
	}
	if pinned {
		headers["x-agentmesh-pin"] = "true"
	}
	for k, v := range req.GetHeaders() {
		headers[k] = v
	}
	var chain []*agentmeshv1.AgentEndpoint
	for i, r := range ranked {
		if i > 4 {
			break
		}
		chain = append(chain, toEndpoint(r.Agent, r.Score))
	}
	timeout := pol.Routing.DefaultTimeoutMs
	retries := pol.Routing.DefaultMaxRetries
	return &agentmeshv1.RouteResponse{
		DestinationAgentId:   chosen.Agent.AgentID,
		DestinationAgentType: chosen.Agent.AgentType,
		EndpointUrl:          chosen.Agent.EndpointURL,
		InjectedHeaders:      headers,
		FallbackChain:        chain,
		TimeoutMs:            timeout,
		MaxRetries:           retries,
		TraceContext:         traceID,
	}, nil
}

func (s *Server) ReportHealth(ctx context.Context, req *agentmeshv1.HealthStatus) (*agentmeshv1.HealthAck, error) {
	org := ""
	if p, ok := ctx.Value("org_id").(string); ok {
		org = p
	}
	if org == "" {
		org = "default"
	}
	err := s.reg.UpdateHealth(ctx, org, req.GetAgentId(), req.GetHealthy(), req.GetMessage())
	if err != nil {
		return &agentmeshv1.HealthAck{Accepted: false}, nil
	}
	return &agentmeshv1.HealthAck{Accepted: true}, nil
}

func (s *Server) GetCircuitState(ctx context.Context, req *agentmeshv1.CircuitQuery) (*agentmeshv1.CircuitState, error) {
	pol, _ := s.policies.Get(ctx, req.GetOrgId())
	agentType := ""
	if rec, _ := s.reg.Get(ctx, req.GetOrgId(), req.GetDestinationAgentId()); rec != nil {
		agentType = rec.AgentType
	}
	cbCfg := policy.ResolveCircuit(pol, req.GetDestinationAgentId(), agentType)
	st, err := s.breaker.GetStateWithConfig(ctx, req.GetOrgId(), req.GetDestinationAgentId(), cbCfg)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "circuit: %v", err)
	}
	return &agentmeshv1.CircuitState{
		State:          st.State,
		FailureCount:   st.FailureCount,
		OpenedAtUnix:   st.OpenedAtUnix,
		HalfOpenAtUnix: st.HalfOpenAtUnix,
		Threshold:      st.Threshold,
		WindowSeconds:  st.WindowSeconds,
	}, nil
}

func NowUnix() int64 { return time.Now().Unix() }

func FormatErr(e error) error {
	return fmt.Errorf("%w", e)
}

func sloRejectReason(maxCost, maxLatency float64, a registry.AgentRecord) string {
	costFail := maxCost > 0 && router.CostOf(a) > maxCost
	latFail := maxLatency > 0 && router.LatencyOf(a) > maxLatency
	switch {
	case costFail && !latFail:
		return "max_cost"
	case latFail && !costFail:
		return "max_latency"
	default:
		return "slo_filter"
	}
}
