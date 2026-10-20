package httpapi

import "net/http"

func (s *Server) xnodeNodeLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := s.deps.XNodeMaster.Logs(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, logs)
}
