package proxy

import "net/http"

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.upstream.Models(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, "upstream_error", err.Error())
		return
	}
	models = s.models.Expose(models)
	type model struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		OwnedBy string `json:"owned_by"`
	}
	out := struct {
		Object string  `json:"object"`
		Data   []model `json:"data"`
	}{Object: "list"}
	for _, m := range models {
		out.Data = append(out.Data, model{ID: m.ID, Object: "model", OwnedBy: "openai"})
	}
	writeJSON(w, http.StatusOK, out)
}
