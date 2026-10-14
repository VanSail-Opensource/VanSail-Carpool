package httpapi

import (
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func (s *Server) adminBackups(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Store.ListBackupRuns(r.Context(), 100)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	response := make([]backupRunResponse, 0, len(items))
	for _, item := range items {
		response = append(response, mapBackupRun(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": response})
}

func (s *Server) adminCreateBackup(w http.ResponseWriter, r *http.Request) {
	run, err := s.deps.Admin.RunBackup(r.Context(), currentUser(r).ID)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapBackupRun(run))
}

func (s *Server) adminRunMaintenance(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	receipt, err := s.deps.Admin.QueueMaintenance(r.Context(), currentUser(r).ID, key)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) adminDeleteBackup(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Admin.DeleteBackup(r.Context(), currentUser(r).ID, chiURLParam(r, "id")); err != nil {
		s.adminFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type backupRunResponse struct {
	ID          string     `json:"id"`
	Path        string     `json:"path"`
	SizeBytes   string     `json:"sizeBytes"`
	Status      string     `json:"status"`
	Error       string     `json:"error"`
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt"`
}

func mapBackupRun(run model.BackupRun) backupRunResponse {
	return backupRunResponse{ID: run.ID, Path: filepath.Base(run.Path), SizeBytes: strconv.FormatInt(run.SizeBytes, 10),
		Status: run.Status, Error: run.Error, CreatedAt: run.CreatedAt, CompletedAt: run.CompletedAt}
}

func (s *Server) adminJobs(w http.ResponseWriter, r *http.Request) {
	query, ok := s.parseAdminInventoryQuery(w, r, jobPageStatuses)
	if !ok {
		return
	}
	items, nextCursor, err := s.deps.Store.ListAdminOutboxJobsPage(r.Context(), query.Cursor, query.Search, query.Status, query.Limit)
	if err != nil {
		s.writeAdminPageFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page": map[string]any{"nextCursor": nextCursor}})
}

func (s *Server) adminRetryJob(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	receipt, err := s.deps.Admin.QueueJobRetry(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), key)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) adminDeleteJob(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Store.DeleteOutboxJob(r.Context(), chiURLParam(r, "id")); err != nil {
		s.adminFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminAuditEvents(w http.ResponseWriter, r *http.Request) {
	items, err := s.deps.Store.ListAuditEvents(r.Context(), 200)
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
