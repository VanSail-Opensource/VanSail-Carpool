package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

type pricingDownloadWriter struct {
	response http.ResponseWriter
	started  bool
}

func (w *pricingDownloadWriter) Write(p []byte) (int, error) {
	w.started = true
	return w.response.Write(p)
}

func (s *Server) pricingExport(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Scope database.PricingExportScope `json:"scope"`
		Token string                      `json:"authorizationToken"`
	}
	if err := decodeJSON(w, r, &input); err != nil || input.Scope.Validate() != nil {
		s.writeError(w, r, http.StatusBadRequest, "PRICING_INVALID_REQUEST", "Export scope is invalid.")
		return
	}
	if (input.Scope.IncludeFinancial || input.Scope.IncludeUsers || len(input.Scope.Users) > 0) && !s.consumePricingExportGrant(r, input.Scope, input.Token) {
		s.writeError(w, r, http.StatusForbidden, "PRICING_EXPORT_AUTHORIZATION_REQUIRED", "A current single-use authorization is required for user or financial data.")
		return
	}
	user := currentUser(r)
	now := time.Now().UTC()
	detail, err := json.Marshal(input.Scope)
	if err != nil {
		s.pricingError(w, r, err)
		return
	}
	if err := s.deps.Store.AppendAudit(r.Context(), &user.ID, "pricing.export.started", "pricing", "export", string(detail), now); err != nil {
		s.pricingError(w, r, err)
		return
	}
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(now.Add(5 * time.Minute)); err != nil {
		s.pricingError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="bapo-export.zip"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Trailer", "X-Pricing-Export-Status")
	output := &pricingDownloadWriter{response: w}
	err = s.deps.Store.StreamPricingExport(r.Context(), output, input.Scope)
	status := "complete"
	if err != nil {
		status = "incomplete"
	}
	w.Header().Set("X-Pricing-Export-Status", status)
	if err != nil && !output.started {
		w.Header().Del("Content-Disposition")
		s.pricingError(w, r, err)
	}
	audit, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if auditErr := s.deps.Store.AppendAudit(audit, &user.ID, "pricing.export."+status, "pricing", "export", status, time.Now().UTC()); auditErr != nil {
		s.deps.Logger.Warn("pricing export completion audit failed", "error", auditErr)
	}
	if err != nil {
		s.deps.Logger.Warn("pricing export interrupted", "error", err)
		if output.started {
			panic(http.ErrAbortHandler)
		} // net/http must reject a truncated ZIP, never report a successful download.
	}
}
