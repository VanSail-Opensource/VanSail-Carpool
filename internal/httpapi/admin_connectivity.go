package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/connectivity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/google/uuid"
)

func (s *Server) connectivityAvailable(w http.ResponseWriter, r *http.Request) bool {
	if s.deps.Connectivity != nil {
		return true
	}
	s.writeError(w, r, http.StatusServiceUnavailable, "CONNECTIVITY_UNAVAILABLE", "Connectivity checks are unavailable.")
	return false
}

func (s *Server) connectivityError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, database.ErrInvalidCursor) || errors.Is(err, database.ErrConflict) {
		s.writeError(w, r, http.StatusBadRequest, "CONNECTIVITY_INVALID_HISTORY_QUERY", "The history query is invalid.")
		return
	}
	var coded *connectivity.CodeError
	status := http.StatusInternalServerError
	if errors.As(err, &coded) {
		status = http.StatusBadRequest
	}
	code := connectivity.ErrorCode(err)
	switch code {
	case "CONNECTIVITY_NOT_READY", "CONNECTIVITY_UNAVAILABLE", "CONNECTIVITY_STORAGE_UNAVAILABLE", "CONNECTIVITY_SETTINGS_UNAVAILABLE", "CONNECTIVITY_DEPENDENCIES_UNAVAILABLE", "CONNECTIVITY_INTERRUPTED", "CONNECTIVITY_PROBE_TIMEOUT":
		status = http.StatusServiceUnavailable
	}
	s.writeError(w, r, status, code, "The connectivity operation could not be completed.")
}

func (s *Server) adminConnectivitySnapshot(w http.ResponseWriter, r *http.Request) {
	if !s.connectivityAvailable(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	snapshot, err := s.deps.Connectivity.Snapshot(ctx)
	if err != nil {
		s.connectivityError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) adminConnectivityCheck(w http.ResponseWriter, r *http.Request) {
	if !s.connectivityAvailable(w, r) {
		return
	}
	run, err := s.deps.Connectivity.Start(r.Context(), "manual")
	if err != nil {
		s.connectivityError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) adminConnectivityResolve(w http.ResponseWriter, r *http.Request) {
	if !s.connectivityAvailable(w, r) {
		return
	}
	var request struct {
		Username string `json:"username"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		s.writeError(w, r, http.StatusBadRequest, "CONNECTIVITY_INVALID_USERNAME", "An exact Remnawave username is required.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	user, err := s.deps.Connectivity.Resolve(ctx, request.Username)
	if err != nil {
		s.connectivityError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) adminConnectivityHistory(w http.ResponseWriter, r *http.Request) {
	if !s.connectivityAvailable(w, r) {
		return
	}
	query := r.URL.Query()
	hostID := query.Get("hostUuid")
	if hostID != "" {
		parsed, err := uuid.Parse(hostID)
		if err != nil || parsed == uuid.Nil {
			s.connectivityError(w, r, database.ErrConflict)
			return
		}
		hostID = parsed.String()
	}
	limit := 50
	if raw := query.Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 200 {
			s.connectivityError(w, r, database.ErrConflict)
			return
		}
	}
	page, err := s.deps.Store.ConnectivityHistory(r.Context(), hostID, query.Get("cursor"), limit, time.Now().UTC())
	if err != nil {
		s.connectivityError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}
