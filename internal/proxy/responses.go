package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

var passthroughHTTPClient = http.DefaultClient

// responsesFeatureHeaders is deliberately limited to headers that opt a
// request into OpenAI API features. Client authentication and account headers
// must always come from the borrowed Codex credential below.
var responsesFeatureHeaders = []string{
	"OpenAI-Beta",
}

// handleResponsesPassthrough forwards a Responses-native request upstream with
// the borrowed credential after applying local safety defaults.
func (s *Server) handleResponsesPassthrough(w http.ResponseWriter, r *http.Request) {
	body, requestedModel, resolvedModel, err := normalizeResponsesBody(r.Body, s.models)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	if client, ok := clientFromContext(r.Context()); ok && !modelAllowed(client, requestedModel, resolvedModel) {
		writeError(w, http.StatusForbidden, "permission_error", "client is not allowed to use this model")
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
	for _, name := range responsesFeatureHeaders {
		for _, value := range r.Header.Values(name) {
			upReq.Header.Add(name, value)
		}
	}
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

func normalizeResponsesBody(r io.Reader, models ModelRegistry) ([]byte, string, string, error) {
	var payload map[string]json.RawMessage
	dec := json.NewDecoder(r)
	if err := dec.Decode(&payload); err != nil {
		return nil, "", "", err
	}
	if _, ok := payload["store"]; !ok {
		payload["store"] = json.RawMessage("false")
	}
	var requestedModel, resolvedModel string
	if rawModel, ok := payload["model"]; ok {
		var model string
		if err := json.Unmarshal(rawModel, &model); err == nil {
			requestedModel = model
			resolvedModel = models.Resolve(model)
			payload["model"], err = json.Marshal(resolvedModel)
			if err != nil {
				return nil, "", "", err
			}
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", "", err
	}
	return body, requestedModel, resolvedModel, nil
}
