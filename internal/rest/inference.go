package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	agentmeshv1 "github.com/agentmesh/agentmesh/gen/agentmesh/v1"
	"github.com/agentmesh/agentmesh/internal/inference"
	"github.com/agentmesh/agentmesh/internal/keymint"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (g *Gateway) listModels(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"models":    g.catalog.Models(),
		"catalog":   g.catalog.Path(),
		"providers": g.catalog.Providers(),
	})
}

func (g *Gateway) listProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"providers": g.catalog.Providers()})
}

func (g *Gateway) keymintStatus(w http.ResponseWriter, _ *http.Request) {
	configured := g.keymint != nil && g.keymint.Configured()
	base := ""
	if configured {
		base = g.keymint.Client.BaseURL()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"configured": configured,
		"base_url":   base,
		"backend":    "keymint",
	})
}

func (g *Gateway) listSecrets(w http.ResponseWriter, r *http.Request) {
	if g.keymint == nil || !g.keymint.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, errBody("KEYMINT_NOT_CONFIGURED",
			"set KEYMINT_BASE_URL, KEYMINT_API_KEY, and KEYMINT_TEAM_ID"))
		return
	}
	views, err := g.keymint.ListViews(r.Context(), orgFromCtx(r), g.catalog)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("SECRETS_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"secrets": views})
}

type putSecretBody struct {
	APIKey   string `json:"api_key"`
	BaseURL  string `json:"base_url"`
	APIModel string `json:"api_model"`
}

func (g *Gateway) putSecret(w http.ResponseWriter, r *http.Request) {
	if g.keymint == nil || !g.keymint.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, errBody("KEYMINT_NOT_CONFIGURED",
			"set KEYMINT_BASE_URL, KEYMINT_API_KEY, and KEYMINT_TEAM_ID"))
		return
	}
	provider := chi.URLParam(r, "provider")
	if !g.catalog.HasProvider(provider) {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_PROVIDER", "provider not in models.yaml catalog"))
		return
	}
	var body putSecretBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_JSON", err.Error()))
		return
	}
	err := g.keymint.Put(r.Context(), orgFromCtx(r), provider, body.APIKey, body.BaseURL, body.APIModel, g.catalog)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("SECRET_PUT_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved", "backend": "keymint"})
}

func (g *Gateway) deleteSecret(w http.ResponseWriter, r *http.Request) {
	if g.keymint == nil || !g.keymint.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, errBody("KEYMINT_NOT_CONFIGURED",
			"set KEYMINT_BASE_URL, KEYMINT_API_KEY, and KEYMINT_TEAM_ID"))
		return
	}
	provider := chi.URLParam(r, "provider")
	if !g.catalog.HasProvider(provider) {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_PROVIDER", "provider not in models.yaml catalog"))
		return
	}
	if err := g.keymint.Delete(r.Context(), orgFromCtx(r), provider); err != nil {
		if errors.Is(err, keymint.ErrBindingNotFound) {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no keymint binding for provider"))
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("SECRET_DELETE_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (g *Gateway) listSecretAudit(w http.ResponseWriter, r *http.Request) {
	if g.keymint == nil || !g.keymint.Configured() {
		writeJSON(w, http.StatusServiceUnavailable, errBody("KEYMINT_NOT_CONFIGURED",
			"set KEYMINT_BASE_URL, KEYMINT_API_KEY, and KEYMINT_TEAM_ID"))
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_LIMIT", "limit must be a positive integer"))
			return
		}
		limit = n
	}
	events, err := g.keymint.ListAudit(r.Context(), orgFromCtx(r), limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errBody("AUDIT_FAILED", err.Error()))
		return
	}
	if events == nil {
		events = []keymint.AuditEvent{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":  events,
		"backend": "keymint",
	})
}

type chatBody struct {
	Model              string              `json:"model"`
	Messages           []inference.Message `json:"messages"`
	Question           string              `json:"question"`
	System             string              `json:"system"`
	RouteViaMesh       *bool               `json:"route_via_mesh"`
	SourceAgentID      string              `json:"source_agent_id"`
	DestinationAgentID string              `json:"destination_agent_id"`
	PreferAgentIDs     []string            `json:"prefer_agent_ids"`
	ForcePin           bool                `json:"force_pin"`
	PinFailover        bool                `json:"pin_failover"`
}

func (g *Gateway) prepareChat(r *http.Request, body *chatBody) (msgs []inference.Message, modelID string, apiModel string, bind keymint.Binding, route *inference.RouteInfo, errCode int, errPayload map[string]any) {
	if g.keymint == nil || !g.keymint.Configured() {
		return nil, "", "", keymint.Binding{}, nil, http.StatusServiceUnavailable, errBody("KEYMINT_NOT_CONFIGURED",
			"set KEYMINT_BASE_URL, KEYMINT_API_KEY, and KEYMINT_TEAM_ID")
	}
	m, ok := g.catalog.ByID(body.Model)
	if !ok {
		return nil, "", "", keymint.Binding{}, nil, http.StatusBadRequest, errBody("UNKNOWN_MODEL", "model not in models.yaml catalog")
	}
	msgs = body.Messages
	if len(msgs) == 0 {
		if strings.TrimSpace(body.Question) == "" {
			return nil, "", "", keymint.Binding{}, nil, http.StatusBadRequest, errBody("EMPTY_PROMPT", "question or messages required")
		}
		if body.System != "" {
			msgs = append(msgs, inference.Message{Role: "system", Content: body.System})
		}
		msgs = append(msgs, inference.Message{Role: "user", Content: body.Question})
	}

	bind, err := g.keymint.GetBinding(r.Context(), orgFromCtx(r), m.Provider)
	if errors.Is(err, keymint.ErrBindingNotFound) {
		return nil, "", "", keymint.Binding{}, nil, http.StatusBadRequest, errBody("SECRET_MISSING",
			"store an API key for provider "+m.Provider+" under Models → Provider keys (Keymint)")
	}
	if err != nil {
		return nil, "", "", keymint.Binding{}, nil, http.StatusInternalServerError, errBody("SECRET_FAILED", err.Error())
	}

	apiModel = m.APIModel
	if bind.APIModel != "" {
		apiModel = bind.APIModel
	}

	routeVia := true
	if body.RouteViaMesh != nil {
		routeVia = *body.RouteViaMesh
	}
	if routeVia {
		client, conn, dialErr := g.dial(r.Context())
		if dialErr == nil {
			src := body.SourceAgentID
			if src == "" {
				src = "console-chat"
			}
			resp, routeErr := client.RouteRequest(r.Context(), &agentmeshv1.RoutePayload{
				OrgId:              orgFromCtx(r),
				Capability:         m.Capability,
				SourceAgentId:      src,
				RoutingStrategy:    "capability-match",
				DestinationAgentId: body.DestinationAgentID,
				PreferAgentIds:     body.PreferAgentIDs,
				ForcePin:           body.ForcePin,
				PinFailover:        body.PinFailover,
			})
			conn.Close()
			if routeErr == nil && resp != nil {
				route = &inference.RouteInfo{
					DestinationAgentID:   resp.GetDestinationAgentId(),
					DestinationAgentType: resp.GetDestinationAgentType(),
					EndpointURL:          resp.GetEndpointUrl(),
					TraceID:              resp.GetInjectedHeaders()["x-agentmesh-trace-id"],
				}
				if route.TraceID == "" {
					route.TraceID = resp.GetTraceContext()
				}
			}
		}
	}
	return msgs, m.ID, apiModel, bind, route, 0, nil
}

func (g *Gateway) chat(w http.ResponseWriter, r *http.Request) {
	var body chatBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_JSON", err.Error()))
		return
	}
	msgs, _, apiModel, bind, route, code, payload := g.prepareChat(r, &body)
	if code != 0 {
		writeJSON(w, code, payload)
		return
	}
	m, _ := g.catalog.ByID(body.Model)
	result, err := g.infer.ChatViaKeymint(r.Context(), g.keymint.Client, bind.VirtualKeyToken, m, apiModel, msgs, route)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errBody("INFERENCE_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (g *Gateway) chatStream(w http.ResponseWriter, r *http.Request) {
	var body chatBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_JSON", err.Error()))
		return
	}
	msgs, modelID, apiModel, bind, route, code, payload := g.prepareChat(r, &body)
	if code != 0 {
		writeJSON(w, code, payload)
		return
	}
	m, _ := g.catalog.ByID(body.Model)

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errBody("STREAM_UNSUPPORTED", "response flushing not supported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	traceID := ""
	dest := ""
	routed := false
	if route != nil {
		traceID = route.TraceID
		dest = route.DestinationAgentID
		routed = true
	}
	if traceID == "" {
		traceID = uuid.NewString()
	}
	meta := map[string]any{
		"trace_id":             traceID,
		"model_id":             modelID,
		"provider":             m.Provider,
		"destination_agent_id": dest,
		"routed_via_mesh":      routed,
	}
	writeSSE(w, flusher, "meta", meta)

	usage, err := g.infer.ChatStreamViaKeymint(r.Context(), g.keymint.Client, bind.VirtualKeyToken, m, apiModel, msgs, func(text string) error {
		writeSSE(w, flusher, "token", map[string]string{"text": text})
		return nil
	})
	if err != nil {
		writeSSE(w, flusher, "error", map[string]string{"message": err.Error()})
		return
	}
	done := map[string]any{}
	if usage != nil {
		done["usage"] = usage
	}
	writeSSE(w, flusher, "done", done)
}

func writeSSE(w http.ResponseWriter, f http.Flusher, event string, data any) {
	raw, _ := json.Marshal(data)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
	f.Flush()
}
