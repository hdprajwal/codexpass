package proxy

import (
	"io"
	"net/http"
)

// handleResponsesPassthrough forwards a Responses-native request upstream with
// the borrowed credential, streaming the body back unchanged.
func (s *Server) handleResponsesPassthrough(w http.ResponseWriter, r *http.Request) {
	cred, err := s.borrow()
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	base := cred.BaseURL()
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	upReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, base+"/responses", r.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}
	upReq.Header.Set("Content-Type", "application/json")
	upReq.Header.Set("Authorization", "Bearer "+cred.APIKey)
	if cred.AccountID != "" {
		upReq.Header.Set("ChatGPT-Account-ID", cred.AccountID)
	}
	resp, err := http.DefaultClient.Do(upReq)
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
