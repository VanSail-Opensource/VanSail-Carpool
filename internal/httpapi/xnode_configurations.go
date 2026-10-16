package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func (s *Server) xnodeConfigurations(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.XNode.Configurations(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) xnodeSaveConfiguration(w http.ResponseWriter, r *http.Request) {
	var item xnode.Configuration
	if err := decodeJSON(w, r, &item); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	if id := chiURLParam(r, "id"); id != "" {
		item.ID = id
	}
	item, err := s.deps.XNode.SaveConfiguration(r.Context(), item)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) xnodeDeleteConfiguration(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.XNode.DeleteConfiguration(r.Context(), chiURLParam(r, "id")); err != nil {
		s.xnodeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (s *Server) xnodeValidateConfiguration(w http.ResponseWriter, r *http.Request) {
	var item struct {
		Document   json.RawMessage `json:"document"`
		SnippetIDs []string        `json:"snippetIds"`
	}
	if err := decodeJSON(w, r, &item); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	result, err := s.deps.XNode.PreviewConfiguration(r.Context(), item.Document, item.SnippetIDs)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, result)
}
func (s *Server) xnodeSnippets(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.XNode.Snippets(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (s *Server) xnodeSaveSnippet(w http.ResponseWriter, r *http.Request) {
	var item xnode.Snippet
	if err := decodeJSON(w, r, &item); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	if id := chiURLParam(r, "id"); id != "" {
		item.ID = id
	}
	item, err := s.deps.XNode.SaveSnippet(r.Context(), item)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (s *Server) xnodeDeleteSnippet(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.XNode.DeleteSnippet(r.Context(), chiURLParam(r, "id")); err != nil {
		s.xnodeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
