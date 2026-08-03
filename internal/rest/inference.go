package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	agentmeshv1 "github.com/agentmesh/agentmesh/gen/agentmesh/v1"
	"github.com/agentmesh/agentmesh/internal/inference"
	"github.com/agentmesh/agentmesh/internal/secrets"
	"github.com/go-chi/chi/v5"
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

func (g *Gateway) listSecrets(w http.ResponseWriter, r *http.Request) {
	views, err := g.secrets.ListViews(r.Context(), orgFromCtx(r), g.catalog)
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
	err := g.secrets.Put(r.Context(), orgFromCtx(r), secrets.ProviderSecret{
		Provider: provider,
		APIKey:   body.APIKey,
		BaseURL:  body.BaseURL,
		APIModel: body.APIModel,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("SECRET_PUT_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

func (g *Gateway) deleteSecret(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if !g.catalog.HasProvider(provider) {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_PROVIDER", "provider not in models.yaml catalog"))
		return
	}
	if err := g.secrets.Delete(r.Context(), orgFromCtx(r), provider); err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("SECRET_DELETE_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type chatBody struct {
	Model         string              `json:"model"`
	Messages      []inference.Message `json:"messages"`
	Question      string              `json:"question"`
	System        string              `json:"system"`
	RouteViaMesh  *bool               `json:"route_via_mesh"`
	SourceAgentID string              `json:"source_agent_id"`
}

func (g *Gateway) chat(w http.ResponseWriter, r *http.Request) {
	var body chatBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_JSON", err.Error()))
		return
	}
	m, ok := g.catalog.ByID(body.Model)
	if !ok {
		writeJSON(w, http.StatusBadRequest, errBody("UNKNOWN_MODEL", "model not in models.yaml catalog"))
		return
	}
	msgs := body.Messages
	if len(msgs) == 0 {
		if strings.TrimSpace(body.Question) == "" {
			writeJSON(w, http.StatusBadRequest, errBody("EMPTY_PROMPT", "question or messages required"))
			return
		}
		if body.System != "" {
			msgs = append(msgs, inference.Message{Role: "system", Content: body.System})
		}
		msgs = append(msgs, inference.Message{Role: "user", Content: body.Question})
	}

	sec, err := g.secrets.Get(r.Context(), orgFromCtx(r), m.Provider)
	if errors.Is(err, secrets.ErrNotFound) {
		writeJSON(w, http.StatusBadRequest, errBody("SECRET_MISSING",
			"store an API key for provider "+m.Provider+" under Models → Provider keys"))
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("SECRET_FAILED", err.Error()))
		return
	}

	routeVia := true
	if body.RouteViaMesh != nil {
		routeVia = *body.RouteViaMesh
	}

	var routeInfo *inference.RouteInfo
	if routeVia {
		client, conn, err := g.dial(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL", err.Error()))
			return
		}
		src := body.SourceAgentID
		if src == "" {
			src = "console-chat"
		}
		resp, err := client.RouteRequest(r.Context(), &agentmeshv1.RoutePayload{
			OrgId:           orgFromCtx(r),
			Capability:      m.Capability,
			SourceAgentId:   src,
			RoutingStrategy: "capability-match",
		})
		conn.Close()
		if err == nil && resp != nil {
			routeInfo = &inference.RouteInfo{
				DestinationAgentID:   resp.GetDestinationAgentId(),
				DestinationAgentType: resp.GetDestinationAgentType(),
				EndpointURL:          resp.GetEndpointUrl(),
				TraceID:              resp.GetInjectedHeaders()["x-agentmesh-trace-id"],
			}
			if routeInfo.TraceID == "" {
				routeInfo.TraceID = resp.GetTraceContext()
			}
		}
	}

	result, err := g.infer.Chat(r.Context(), g.catalog, m, sec, msgs, routeInfo)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errBody("INFERENCE_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, result)
}
