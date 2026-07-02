package proxy

import (
	"io"
	"net/http"
	"strings"
)

type FallbackConfig struct {
	Enabled bool
	BaseURL string
	APIKey  string
}

var fallbackHTTPClient = http.DefaultClient

func (s *Server) handleFallback(endpointPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.cfg.Fallback.Enabled {
			writeError(w, http.StatusNotImplemented, "not_implemented", "the Codex backend does not serve this endpoint")
			return
		}
		if client, ok := clientFromContext(r.Context()); ok && !client.AllowFallback {
			writeError(w, http.StatusForbidden, "permission_error", "client is not allowed to use fallback endpoints")
			return
		}
		base := strings.TrimRight(s.cfg.Fallback.BaseURL, "/")
		if base == "" || s.cfg.Fallback.APIKey == "" {
			writeError(w, http.StatusBadGateway, "upstream_error", "fallback backend is not fully configured")
			return
		}
		upReq, err := http.NewRequestWithContext(r.Context(), r.Method, base+endpointPath, r.Body)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
		upReq.Header.Set("Authorization", "Bearer "+s.cfg.Fallback.APIKey)
		upReq.Header.Set("Content-Type", r.Header.Get("Content-Type"))
		resp, err := fallbackHTTPClient.Do(upReq)
		if err != nil {
			writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
			return
		}
		defer resp.Body.Close()
		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}
