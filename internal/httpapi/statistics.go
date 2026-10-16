package httpapi

import (
	"net/http"
	"time"
)

func (s *Server) statisticsSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.deps.Statistics.Snapshot(r.Context())
	if err != nil {
		s.writeError(w, r, http.StatusServiceUnavailable, "STATISTICS_UNAVAILABLE", "Statistics are not available yet.")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) statisticsNodes(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.deps.Statistics.NodeSnapshot(r.Context(), time.Now().UTC())
	if err != nil {
		s.deps.Logger.Warn("load live node statistics", "request_id", middlewareRequestID(r), "error", err)
		s.writeError(w, r, http.StatusBadGateway, "NODE_STATISTICS_UNAVAILABLE", "Node statistics are temporarily unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}
