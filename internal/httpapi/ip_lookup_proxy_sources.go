package httpapi

import (
	"net/http"

	"github.com/txyyddss/Remna-User-Panel/internal/iplookup"
)

func (s *Server) adminProxySources(w http.ResponseWriter, r *http.Request) {
	if s.deps.IPLookup == nil || s.deps.IPLookup.Enhanced == nil {
		s.ipLookupError(w, r, &iplookup.CodeError{Code: "IP_LOOKUP_UNAVAILABLE"})
		return
	}
	result, err := s.deps.IPLookup.Enhanced.SourcesStatus(r.Context())
	if err != nil {
		s.ipLookupError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) adminTestProxySource(w http.ResponseWriter, r *http.Request) {
	if s.deps.IPLookup == nil || s.deps.IPLookup.Enhanced == nil {
		s.ipLookupError(w, r, &iplookup.CodeError{Code: "IP_LOOKUP_UNAVAILABLE"})
		return
	}
	result, err := s.deps.IPLookup.Enhanced.TestSource(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		s.ipLookupError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusAccepted, result)
}

func (s *Server) adminProxySourceTest(w http.ResponseWriter, r *http.Request) {
	if s.deps.IPLookup == nil || s.deps.IPLookup.Enhanced == nil {
		s.ipLookupError(w, r, &iplookup.CodeError{Code: "IP_LOOKUP_UNAVAILABLE"})
		return
	}
	result, err := s.deps.IPLookup.Enhanced.SourceTest(chiURLParam(r, "testId"))
	if err != nil {
		s.ipLookupError(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}
