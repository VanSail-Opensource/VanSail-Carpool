package httpapi

import (
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
)

func (s *Server) agentQPSReport(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if token == "" {
		s.writeError(w, r, http.StatusUnauthorized, "AGENT_UNAUTHORIZED", "A node API key is required.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, abuse.MaxReportBytes)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		s.writeError(w, r, http.StatusRequestEntityTooLarge, "REPORT_TOO_LARGE", "The report exceeds the allowed size.")
		return
	}
	counts, err := s.deps.Abuse.Ingest(r.Context(), token, string(raw), time.Now().UTC())
	if err != nil {
		s.writeError(w, r, http.StatusUnauthorized, "AGENT_UNAUTHORIZED", "The node API key or report is invalid.")
		return
	}
	writeJSON(w, http.StatusOK, counts)
}
