package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

var passthroughHTTPClient = http.DefaultClient

// handleResponsesPassthrough forwards a Responses-native request upstream with
// the borrowed credential after applying local safety defaults.
func (s *Server) handleResponsesPassthrough(w http.ResponseWriter, r *http.Request) {
	body, err := normalizeResponsesBody(r.Body, s.models)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	cred, err := s.borrow()
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	base := cred.BaseURL()
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	upReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, base+"/responses", bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	upReq.Header.Set("Content-Type", "application/json")
	upReq.Header.Set("Authorization", "Bearer "+cred.APIKey)
	if cred.AccountID != "" {
		upReq.Header.Set("ChatGPT-Account-ID", cred.AccountID)
	}
	resp, err := passthroughHTTPClient.Do(upReq)
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

func normalizeResponsesBody(r io.Reader, models ModelRegistry) ([]byte, error) {
	var payload map[string]any
	dec := json.NewDecoder(r)
	if err := dec.Decode(&payload); err != nil {
		return nil, err
	}
	if _, ok := payload["store"]; !ok {
		payload["store"] = false
	}
	if model, ok := payload["model"].(string); ok {
		payload["model"] = models.Resolve(model)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return body, nil
}
