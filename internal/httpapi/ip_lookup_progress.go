package httpapi

import "net/http"

func (s *Server) ipLookupProgress(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.ipLookupMember(w, r) {
		return
	}
	result, err := s.deps.IPLookup.Progress(r.Context(), r.URL.Query().Get("ip"))
	if err != nil {
		s.ipLookupError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
