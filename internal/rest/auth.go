package rest

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/agentmesh/agentmesh/internal/auth"
	"github.com/go-chi/chi/v5"
)

func (g *Gateway) registerAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_JSON", err.Error()))
		return
	}
	res, err := g.auth.Register(r.Context(), body.Email, body.Password, false)
	if errors.Is(err, auth.ErrRegisterDisabled) {
		writeJSON(w, http.StatusForbidden, errBody("REGISTER_DISABLED", err.Error()))
		return
	}
	if errors.Is(err, auth.ErrUserExists) {
		writeJSON(w, http.StatusConflict, errBody("USER_EXISTS", err.Error()))
		return
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("REGISTER_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (g *Gateway) loginAuth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_JSON", err.Error()))
		return
	}
	res, err := g.auth.Login(r.Context(), body.Email, body.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, errBody("LOGIN_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (g *Gateway) logoutAuth(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errBody("UNAUTHORIZED", "not authenticated"))
		return
	}
	_ = g.auth.Logout(r.Context(), p.SessionJTI)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (g *Gateway) meAuth(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok || p.UserID == "" {
		writeJSON(w, http.StatusUnauthorized, errBody("UNAUTHORIZED", "JWT session required"))
		return
	}
	user, orgs, err := g.auth.Me(r.Context(), p.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("ME_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "orgs": orgs, "current_org": p.OrgID, "role": p.Role})
}

func (g *Gateway) createOrg(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok || p.UserID == "" {
		writeJSON(w, http.StatusUnauthorized, errBody("UNAUTHORIZED", "JWT session required"))
		return
	}
	var body struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_JSON", err.Error()))
		return
	}
	org, err := g.auth.CreateOrg(r.Context(), p.UserID, body.ID, body.Name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("ORG_CREATE_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, org)
}

func (g *Gateway) listAuthOrgs(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if ok && p.UserID != "" {
		_, orgs, err := g.auth.Me(r.Context(), p.UserID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("ORGS_FAILED", err.Error()))
			return
		}
		ids := make([]string, 0, len(orgs))
		for _, o := range orgs {
			ids = append(ids, o.ID)
		}
		// Also include registry orgs for operators browsing agents
		if regIDs, err := g.registry.ListOrgIDs(r.Context()); err == nil {
			seen := map[string]struct{}{}
			for _, id := range ids {
				seen[id] = struct{}{}
			}
			for _, id := range regIDs {
				if _, ok := seen[id]; !ok {
					ids = append(ids, id)
				}
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"orgs": ids, "memberships": orgs})
		return
	}
	// API key path: fall back to registry list
	g.listOrgs(w, r)
}

func (g *Gateway) createOrgKey(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errBody("UNAUTHORIZED", "not authenticated"))
		return
	}
	if err := auth.RequireRole(p, auth.RoleOperator); err != nil {
		writeJSON(w, http.StatusForbidden, errBody("FORBIDDEN", err.Error()))
		return
	}
	org := chi.URLParam(r, "org")
	if org == "" {
		org = p.OrgID
	}
	if org != p.OrgID {
		writeJSON(w, http.StatusForbidden, errBody("ORG_MISMATCH", "path org must match authenticated org"))
		return
	}
	var body struct {
		Name string    `json:"name"`
		Role auth.Role `json:"role"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	k, plain, err := g.auth.CreateAPIKey(r.Context(), org, body.Name, body.Role)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("KEY_CREATE_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"key": k, "api_key": plain})
}

func (g *Gateway) listOrgKeys(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errBody("UNAUTHORIZED", "not authenticated"))
		return
	}
	if err := auth.RequireRole(p, auth.RoleOperator); err != nil {
		writeJSON(w, http.StatusForbidden, errBody("FORBIDDEN", err.Error()))
		return
	}
	org := chi.URLParam(r, "org")
	if org == "" {
		org = p.OrgID
	}
	if org != p.OrgID {
		writeJSON(w, http.StatusForbidden, errBody("ORG_MISMATCH", "path org must match authenticated org"))
		return
	}
	keys, err := g.auth.Store.ListAPIKeys(r.Context(), org)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("KEYS_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": keys})
}

func (g *Gateway) revokeOrgKey(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, errBody("UNAUTHORIZED", "not authenticated"))
		return
	}
	if err := auth.RequireRole(p, auth.RoleOperator); err != nil {
		writeJSON(w, http.StatusForbidden, errBody("FORBIDDEN", err.Error()))
		return
	}
	org := chi.URLParam(r, "org")
	keyID := chi.URLParam(r, "key_id")
	if org != p.OrgID {
		writeJSON(w, http.StatusForbidden, errBody("ORG_MISMATCH", "path org must match authenticated org"))
		return
	}
	if err := g.auth.Store.RevokeAPIKey(r.Context(), org, keyID); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("KEY_REVOKE_FAILED", err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (g *Gateway) requireMinRole(min auth.Role, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFrom(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errBody("UNAUTHORIZED", "not authenticated"))
			return
		}
		if err := auth.RequireRole(p, min); err != nil {
			writeJSON(w, http.StatusForbidden, errBody("FORBIDDEN", "insufficient role"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
