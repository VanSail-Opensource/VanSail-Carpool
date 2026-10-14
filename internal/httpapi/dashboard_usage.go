package httpapi

import (
	"errors"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/catalog"
)

func (s *Server) dashboardNodeUsage(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if !s.requireOnboarded(w, r, user) {
		return
	}
	start, end, ok := dashboardUsageRange(r)
	if !ok {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_USAGE_RANGE", "Choose a valid UTC start and end date.")
		return
	}
	report, err := s.deps.Catalog.NodeUsage(r.Context(), user, start, end)
	if err != nil {
		if errors.Is(err, catalog.ErrDashboardNodeUsageRange) {
			s.writeError(w, r, http.StatusBadRequest, "INVALID_USAGE_RANGE", "Choose a usage range of up to 31 days.")
			return
		}
		s.writeError(w, r, http.StatusBadGateway, "NODE_USAGE_UNAVAILABLE", "Per-node usage is temporarily unavailable.")
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func dashboardUsageRange(r *http.Request) (time.Time, time.Time, bool) {
	start, startErr := time.Parse(time.DateOnly, r.URL.Query().Get("start"))
	end, endErr := time.Parse(time.DateOnly, r.URL.Query().Get("end"))
	if startErr != nil || endErr != nil || start.After(end) {
		return time.Time{}, time.Time{}, false
	}
	return start.UTC(), end.UTC(), true
}
