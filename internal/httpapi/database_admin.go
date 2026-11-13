package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/backup"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/databaseadmin"
	"github.com/go-chi/chi/v5"
	"io"
	"mime"
	"net/http"
	"strconv"
)

type DatabaseAdministrationHTTP struct {
	editor  *databaseadmin.Service
	backups *backup.Service
}

// NewDatabaseAdministrationHTTP constructs independently mountable handlers.

func NewDatabaseAdministrationHTTP(editor *databaseadmin.Service, backups *backup.Service) *DatabaseAdministrationHTTP {
	return &DatabaseAdministrationHTTP{editor: editor, backups: backups}
}

// Mount registers database, backup download, routes on an
// already-authenticated administrator router.

func (h *DatabaseAdministrationHTTP) Mount(router chi.Router) {
	router.Get("/database/tables", h.tables)
	router.Get("/database/tables/{table}/rows", h.rows)
	router.Post("/database/tables/{table}/query", h.queryRows)
	router.Post("/database/mutations/review", h.review)
	router.Post("/database/mutations", h.apply)
	router.Get("/backups/{id}/download", h.download)
}

func (h *DatabaseAdministrationHTTP) tables(w http.ResponseWriter, r *http.Request) {
	items, err := h.editor.Tables(r.Context())
	if err != nil {
		h.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *DatabaseAdministrationHTTP) rows(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil && r.URL.Query().Get("limit") != "" {
		h.writeError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "The page limit must be an integer.")
		return
	}
	page, err := h.editor.Records(r.Context(), chi.URLParam(r, "table"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		h.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *DatabaseAdministrationHTTP) queryRows(w http.ResponseWriter, r *http.Request) {
	var request databaseadmin.QueryRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "INVALID_DATABASE_QUERY", "The typed database query is invalid.")
		return
	}
	page, err := h.editor.QueryRecords(r.Context(), chi.URLParam(r, "table"), request)
	if err != nil {
		h.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func (h *DatabaseAdministrationHTTP) review(w http.ResponseWriter, r *http.Request) {
	var request databaseadmin.MutationRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "INVALID_DATABASE_MUTATION", "The typed database mutation is invalid.")
		return
	}
	review, err := h.editor.ReviewMutation(r.Context(), currentUser(r).ID, request)
	if err != nil {
		h.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, review)
}

func (h *DatabaseAdministrationHTTP) apply(w http.ResponseWriter, r *http.Request) {
	var request databaseadmin.MutationRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "INVALID_DATABASE_MUTATION", "The reviewed database mutation is invalid.")
		return
	}
	result, err := h.editor.ApplyMutation(r.Context(), currentUser(r).ID, request)
	if err != nil {
		h.failure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *DatabaseAdministrationHTTP) download(w http.ResponseWriter, r *http.Request) {
	download, err := h.backups.OpenDownload(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		h.failure(w, r, err)
		return
	}
	defer func() { _ = download.File.Close() }()
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": download.Name})
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", disposition)
	w.Header().Set("Content-Length", strconv.FormatInt(download.Size, 10))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, download.File)
}
