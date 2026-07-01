package proxy

import (
	"errors"
	"net/http"

	"github.com/hdprajwal/codex2key/internal/codex"
)

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	req, err := DecodeChatRequest(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "could not parse request body: "+err.Error())
		return
	}
	up, err := toUpstream(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}

	if req.Stream {
		s.handleChatStream(w, r, req, up)
		return
	}

	res, err := s.upstream.Complete(r.Context(), up)
	if err != nil {
		writeUpstreamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toChatCompletion(req.Model, res))
}

// writeUpstreamError maps upstream/borrow errors to OpenAI-shaped responses.
func writeUpstreamError(w http.ResponseWriter, err error) {
	if errors.Is(err, codex.ErrNoAuthFile) || errors.Is(err, codex.ErrRefreshExpired) ||
		errors.Is(err, codex.ErrNoTokens) || errors.Is(err, codex.ErrNoRefreshToken) {
		writeError(w, http.StatusUnauthorized, "authentication_error", err.Error())
		return
	}
	writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
}

// handleChatStream is a temporary stub replaced in the streaming task.
func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request, req ChatRequest, up UpstreamRequest) {
	writeError(w, http.StatusNotImplemented, "not_implemented", "streaming not yet implemented")
}
