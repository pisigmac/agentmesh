package rest

import (
	"net/http"
	"net/url"
	"os"
	"strings"
)

// corsMiddleware allows browser dashboards to call the REST API cross-origin.
// Set AGENTMESH_CORS_ORIGINS to a comma-separated list (e.g. https://app.example.com).
// When unset, any http(s) origin on localhost or 127.0.0.1 is allowed (local dev).
func corsMiddleware(next http.Handler) http.Handler {
	explicit := parseCORSOrigins(os.Getenv("AGENTMESH_CORS_ORIGINS"))
	allowAll := explicit.allowAll
	allowed := explicit.exact

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && originAllowed(origin, allowed, allowAll, len(explicit.exact) > 0 || explicit.allowAll) {
			if allowAll {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Org-ID, X-Workspace-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type corsConfig struct {
	exact    map[string]struct{}
	allowAll bool
}

func parseCORSOrigins(raw string) corsConfig {
	cfg := corsConfig{exact: make(map[string]struct{})}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cfg
	}
	for _, part := range strings.Split(raw, ",") {
		o := strings.TrimSpace(part)
		if o == "" {
			continue
		}
		if o == "*" {
			cfg.allowAll = true
			continue
		}
		cfg.exact[o] = struct{}{}
	}
	return cfg
}

func originAllowed(origin string, allowed map[string]struct{}, allowAll, explicitConfigured bool) bool {
	if allowAll {
		return true
	}
	if _, ok := allowed[origin]; ok {
		return true
	}
	if !explicitConfigured {
		return isLocalDevOrigin(origin)
	}
	return false
}

func isLocalDevOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1"
}
