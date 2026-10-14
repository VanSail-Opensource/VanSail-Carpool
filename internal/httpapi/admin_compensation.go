package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/compensation"
)

type compensationConfigRequest struct {
	Enabled          bool `json:"enabled"`
	ThresholdMinutes *int `json:"thresholdMinutes"`
	MultiplierBPS    *int `json:"multiplierBps"`
	Revision         int  `json:"revision"`
}

type compensationReviewRequest struct {
	Revision         int    `json:"revision"`
	ExtensionMinutes int    `json:"extensionMinutes"`
	Reason           string `json:"reason"`
}

func (s *Server) adminCompensationConfig(w http.ResponseWriter, r *http.Request) {
	config, err := s.deps.Compensation.Config(r.Context())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) adminUpdateCompensationConfig(w http.ResponseWriter, r *http.Request) {
	var request compensationConfigRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_COMPENSATION", "A complete compensation configuration is required.")
		return
	}
	config, err := s.deps.Compensation.UpdateConfig(r.Context(), currentUser(r).ID, compensation.ConfigUpdate{
		Enabled: request.Enabled, ThresholdMinutes: request.ThresholdMinutes,
		MultiplierBPS: request.MultiplierBPS, Revision: request.Revision,
	}, time.Now().UTC())
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) adminCompensationEvents(w http.ResponseWriter, r *http.Request) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			s.writeError(w, r, http.StatusBadRequest, "INVALID_PAGE_SIZE", "Page size must be between 1 and 100.")
			return
		}
		limit = parsed
	}
	page, err := s.deps.Compensation.Events(r.Context(), r.URL.Query().Get("status"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		s.writeAdminPageFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) adminApproveCompensationEvent(w http.ResponseWriter, r *http.Request) {
	s.adminReviewCompensationEvent(w, r, true)
}

func (s *Server) adminDismissCompensationEvent(w http.ResponseWriter, r *http.Request) {
	s.adminReviewCompensationEvent(w, r, false)
}

func (s *Server) adminReviewCompensationEvent(w http.ResponseWriter, r *http.Request, approve bool) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request compensationReviewRequest
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "INVALID_COMPENSATION_REVIEW", "Revision, extension, and reason are required.")
		return
	}
	input := compensation.ReviewInput{EventID: chiURLParam(r, "id"), ActorUserID: currentUser(r).ID,
		IdempotencyKey: key, Reason: request.Reason, Revision: request.Revision, ExtensionMinutes: request.ExtensionMinutes}
	var event compensation.Event
	var err error
	if approve {
		event, err = s.deps.Compensation.Approve(r.Context(), input, time.Now().UTC())
	} else {
		event, err = s.deps.Compensation.Dismiss(r.Context(), input, time.Now().UTC())
	}
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, event)
}
