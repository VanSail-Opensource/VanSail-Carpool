package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
)

type bulkExtensionRequest struct {
	ComboIDs        []string `json:"comboIds"`
	AddonSquadUUIDs []string `json:"addonSquadUuids"`
	DurationMinutes *int     `json:"durationMinutes"`
	Days            *int     `json:"days"`
	Reason          string   `json:"reason"`
}

func (request bulkExtensionRequest) domain() (admin.BulkExtension, error) {
	if (request.DurationMinutes == nil) == (request.Days == nil) {
		return admin.BulkExtension{}, errors.New("exactly one extension duration is required")
	}
	durationMinutes := 0
	if request.DurationMinutes != nil {
		durationMinutes = *request.DurationMinutes
	} else {
		if *request.Days < 1 || *request.Days > 3650 {
			return admin.BulkExtension{}, errors.New("extension days must be between 1 and 3650")
		}
		durationMinutes = *request.Days * 24 * 60
	}
	return admin.BulkExtension{ComboIDs: request.ComboIDs, AddonSquadUUIDs: request.AddonSquadUUIDs,
		DurationMinutes: durationMinutes, Reason: request.Reason}, nil
}

func (s *Server) adminPreviewBulkExtension(w http.ResponseWriter, r *http.Request) {
	var request bulkExtensionRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_BULK_EXTENSION", "Filters and an extension duration are required.")
		return
	}
	domain, err := request.domain()
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	preview, err := s.deps.AdminUsers.PreviewBulkExtension(r.Context(), domain)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (s *Server) adminCreateBulkExtension(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request bulkExtensionRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_BULK_EXTENSION", "Filters, duration, and reason are required.")
		return
	}
	domain, err := request.domain()
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	receipt, err := s.deps.AdminUsers.CreateBulkExtension(r.Context(), currentUser(r).ID, key, domain)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}
