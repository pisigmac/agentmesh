package dataplane

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	agentmeshv1 "github.com/agentmesh/agentmesh/gen/agentmesh/v1"
)

// CircuitRecorder records upstream outcomes for circuit breaking.
type CircuitRecorder interface {
	RecordFailure(ctx context.Context, orgID, destAgentID string) (int, error)
	RecordSuccess(ctx context.Context, orgID, destAgentID string) error
	AllowRequest(ctx context.Context, orgID, destAgentID string) (bool, error)
}

// NoopCircuit ignores circuit state (used when Redis unavailable).
type NoopCircuit struct{}

func (NoopCircuit) RecordFailure(context.Context, string, string) (int, error) { return 0, nil }
func (NoopCircuit) RecordSuccess(context.Context, string, string) error         { return nil }
func (NoopCircuit) AllowRequest(context.Context, string, string) (bool, error) {
	return true, nil
}

type Candidate struct {
	AgentID     string
	AgentType   string
	EndpointURL string
}

// CandidatesFromRoute builds primary + fallback_chain (deduped).
func CandidatesFromRoute(resp *agentmeshv1.RouteResponse) []Candidate {
	seen := map[string]struct{}{}
	var out []Candidate
	add := func(id, typ, ep string) {
		if ep == "" {
			return
		}
		key := id
		if key == "" {
			key = ep
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, Candidate{AgentID: id, AgentType: typ, EndpointURL: ep})
	}
	add(resp.GetDestinationAgentId(), resp.GetDestinationAgentType(), resp.GetEndpointUrl())
	for _, ep := range resp.GetFallbackChain() {
		add(ep.GetAgentId(), ep.GetAgentType(), ep.GetEndpointUrl())
	}
	return out
}

type ProxyConfig struct {
	Timeout    time.Duration
	MaxRetries int
	Failover   bool
	PathSuffix string // e.g. "/v1/chat" from /v1/proxy/v1/chat
	Headers    map[string]string
	OrgID      string
	Circuit    CircuitRecorder
	HTTPClient *http.Client // optional; unit tests
}

type ProxyResult struct {
	Status        int
	Body          []byte
	Header        http.Header
	DestinationID string
	Attempts      int
	FailedAgents  []string
}

// ProxyWithFailover forwards body to candidates.
// When Failover is true, walks fallback candidates on 5xx/timeout within MaxRetries+1 attempts.
// When false, retries the primary only.
func ProxyWithFailover(ctx context.Context, method string, body []byte, contentType string, candidates []Candidate, cfg ProxyConfig) (ProxyResult, error) {
	if cfg.Circuit == nil {
		cfg.Circuit = NoopCircuit{}
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	maxAttempts := cfg.MaxRetries + 1
	if len(candidates) == 0 {
		return ProxyResult{
			Status: http.StatusBadGateway,
			Body:   []byte("no upstream candidates"),
			Header: make(http.Header),
		}, nil
	}
	if !cfg.Failover {
		candidates = candidates[:1]
	}

	result := ProxyResult{Header: make(http.Header)}
	var lastStatus int
	var lastBody []byte
	var lastHdr http.Header

	for _, c := range candidates {
		if result.Attempts >= maxAttempts {
			break
		}
		ok, err := cfg.Circuit.AllowRequest(ctx, cfg.OrgID, c.AgentID)
		if err == nil && !ok {
			result.FailedAgents = append(result.FailedAgents, c.AgentID+":circuit_open")
			continue
		}

		for {
			if result.Attempts >= maxAttempts {
				break
			}
			result.Attempts++
			status, respBody, hdr, fwdErr := forwardOnce(ctx, method, body, contentType, c, cfg)
			lastStatus, lastBody, lastHdr = status, respBody, hdr
			if fwdErr == nil && status > 0 && status < 500 {
				_ = cfg.Circuit.RecordSuccess(ctx, cfg.OrgID, c.AgentID)
				result.Status = status
				result.Body = respBody
				result.Header = hdr
				result.DestinationID = c.AgentID
				return result, nil
			}
			_, _ = cfg.Circuit.RecordFailure(ctx, cfg.OrgID, c.AgentID)
			result.FailedAgents = append(result.FailedAgents, c.AgentID)
			if cfg.Failover {
				// move to next candidate
				break
			}
			// same primary retry loop continues until maxAttempts
		}
	}

	result.Status = lastStatus
	if result.Status == 0 {
		result.Status = http.StatusBadGateway
	}
	if len(lastBody) > 0 {
		result.Body = lastBody
	} else {
		result.Body = []byte("upstream failed after failover attempts")
	}
	if lastHdr != nil {
		result.Header = lastHdr
	}
	return result, nil
}

func forwardOnce(ctx context.Context, method string, body []byte, contentType string, c Candidate, cfg ProxyConfig) (int, []byte, http.Header, error) {
	target, err := url.Parse(c.EndpointURL)
	if err != nil {
		return 0, nil, nil, err
	}
	pathSuffix := cfg.PathSuffix
	if pathSuffix == "" {
		pathSuffix = "/"
	}
	target.Path = strings.TrimSuffix(target.Path, "/") + pathSuffix
	if target.Path == "" {
		target.Path = "/"
	}

	if cfg.HTTPClient != nil {
		req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
		if err != nil {
			return 0, nil, nil, err
		}
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		for k, v := range cfg.Headers {
			req.Header.Set(k, v)
		}
		resp, err := cfg.HTTPClient.Do(req)
		if err != nil {
			return 0, nil, nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, b, resp.Header.Clone(), nil
	}

	rec := &httptestRecorder{header: make(http.Header)}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		return 0, nil, nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	req.URL = target
	req.Host = target.Host
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		ResponseHeaderTimeout: cfg.Timeout,
		IdleConnTimeout:       90 * time.Second,
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("upstream error"))
	}
	proxy.ServeHTTP(rec, req)
	status := rec.status
	if status == 0 {
		status = http.StatusBadGateway
	}
	return status, rec.body, rec.header, nil
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
