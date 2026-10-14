package httpapi

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

var (
	entitlementPageStatuses = []string{"activating", "active", "queued", "expired", "cancelled", "failed"}
	paymentPageStatuses     = []string{"creating", "pending", "paid", "expired", "failed", "refunded", "cancelled"}
	refundPageStatuses      = []string{"completed"}
	jobPageStatuses         = []string{"pending", "processing", "done", "failed"}
)

type adminInventoryQuery struct {
	Cursor string
	Search string
	Status string
	Limit  int
}

func (s *Server) parseAdminInventoryQuery(w http.ResponseWriter, r *http.Request, statuses []string) (adminInventoryQuery, bool) {
	query := adminInventoryQuery{
		Cursor: r.URL.Query().Get("cursor"),
		Search: strings.TrimSpace(r.URL.Query().Get("search")),
		Status: strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status"))),
		Limit:  25,
	}
	if len(query.Search) > 100 {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_SEARCH", "Search must contain at most 100 characters.")
		return adminInventoryQuery{}, false
	}
	if query.Status != "" && !slices.Contains(statuses, query.Status) {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_STATUS", "The requested status filter is invalid.")
		return adminInventoryQuery{}, false
	}
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			s.writeError(w, r, http.StatusBadRequest, "INVALID_PAGE_SIZE", "Page size must be between 1 and 100.")
			return adminInventoryQuery{}, false
		}
		query.Limit = parsed
	}
	return query, true
}

func (s *Server) writeAdminPageFailure(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, database.ErrInvalidCursor) {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "The pagination cursor is invalid.")
		return
	}
	s.adminFailure(w, r, err)
}
