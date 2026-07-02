package proxy

import (
	"errors"
	"net/http"

	"github.com/hdprajwal/codexpass/internal/codex"
)

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	req, err := DecodeChatRequest(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "could not parse request body: "+err.Error())
		return
	}
	if err := ValidateChatRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	up, err := toUpstream(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", err.Error())
		return
	}
	up.Model = s.models.Resolve(up.Model)

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

// handleChatStream streams a chat/completions response as SSE chunks.
func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request, req ChatRequest, up UpstreamRequest) {
	sse, ok := newSSEWriter(w)
	if !ok {
		writeError(w, http.StatusInternalServerError, "server_error", "streaming unsupported")
		return
	}
	st := newStreamState(req.Model, up.IncludeUsage)
	err := s.upstream.Stream(r.Context(), up, func(e StreamEvent) error {
		chunks, err := st.onEvent(e)
		if err != nil {
			return err
		}
		for _, c := range chunks {
			if err := sse.send(c); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		_ = sse.send(map[string]any{"error": map[string]any{"message": err.Error(), "type": "upstream_error"}})
		return
	}
	for _, c := range st.finalChunks() {
		_ = sse.send(c)
	}
	_ = sse.done()
}
