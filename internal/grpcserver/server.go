package grpcserver

import (
	"context"
	"fmt"
	"time"

	agentmeshv1 "github.com/agentmesh/agentmesh/gen/agentmesh/v1"
	"github.com/agentmesh/agentmesh/internal/circuit"
	"github.com/agentmesh/agentmesh/internal/config"
	"github.com/agentmesh/agentmesh/internal/policy"
	"github.com/agentmesh/agentmesh/internal/ratelimit"
	"github.com/agentmesh/agentmesh/internal/registry"
	"github.com/agentmesh/agentmesh/internal/router"
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
	if maxCost == 0 && pol.Routing.DefaultMaxCost > 0 {
		maxCost = pol.Routing.DefaultMaxCost
	}
	agents, err := s.reg.ListByOrg(ctx, req.GetOrgId())
	if err != nil {
		return status.Errorf(codes.Internal, "list: %v", err)
	}
	ranked := s.engine.Rank(router.RankInput{
		OrgID:      req.GetOrgId(),
		Capability: req.GetCapability(),
		MaxCost:    maxCost,
		Strategy:   router.Strategy(strategy),
		Agents:     agents,
	})
	limit := int(req.GetLimit())
	if limit <= 0 {
		if req.GetCapability() == "" {
			limit = 256
		} else {
			limit = 50
		}
	}
	for i, r := range ranked {
		if i >= limit {
			break
		}
		ep := &agentmeshv1.AgentEndpoint{
			AgentId:            r.Agent.AgentID,
			AgentType:          r.Agent.AgentType,
			EndpointUrl:        r.Agent.EndpointURL,
			Capabilities:       r.Agent.Capabilities,
			EstimatedLatencyMs: r.Agent.LatencyEWMA,
			SuccessRate:        r.Agent.SuccessRate,
			CostScore:          r.Agent.CostPerReq,
			RankScore:          r.Score,
			OrgId:              r.Agent.OrgID,
		}
		if err := stream.Send(ep); err != nil {
			return err
		}
	}
	return nil
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
	ranked := s.engine.Rank(router.RankInput{
		OrgID:      req.GetOrgId(),
		Capability: req.GetCapability(),
		Strategy:   router.Strategy(strategy),
		Agents:     agents,
	})
	ranked = router.FilterByFallbackTypes(ranked, req.GetFallbackAgentTypes())
	if len(ranked) == 0 {
		return nil, status.Error(codes.NotFound, "no route")
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
	for k, v := range req.GetHeaders() {
		headers[k] = v
	}
	var chain []*agentmeshv1.AgentEndpoint
	for i, r := range ranked {
		if i > 4 {
			break
		}
		chain = append(chain, &agentmeshv1.AgentEndpoint{
			AgentId:     r.Agent.AgentID,
			AgentType:   r.Agent.AgentType,
			EndpointUrl: r.Agent.EndpointURL,
		})
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
