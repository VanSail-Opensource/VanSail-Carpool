package httpapi

import (
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnodemigration"
	"net/http"
)

func (s *Server) xnodeMigrationStatus(w http.ResponseWriter, r *http.Request) {
	value, err := s.deps.XNodeMigration.Status(r.Context())
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) xnodeMigrationPreview(w http.ResponseWriter, r *http.Request) {
	var input xnodemigration.PreviewInput
	if err := decodeJSON(w, r, &input); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	value, err := s.deps.XNodeMigration.Preview(r.Context(), input)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) xnodeMigrationImport(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Digest string `json:"digest"`
	}
	if err := decodeJSON(w, r, &input); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	s.queueXNodeMigration(w, r, xnodemigration.OperationRequest{Action: "import", Digest: input.Digest})
}
func (s *Server) xnodeMigrationCutover(w http.ResponseWriter, r *http.Request) {
	s.queueXNodeMigration(w, r, xnodemigration.OperationRequest{Action: "cutover", NodeID: chiURLParam(r, "id")})
}
func (s *Server) xnodeMigrationRollback(w http.ResponseWriter, r *http.Request) {
	s.queueXNodeMigration(w, r, xnodemigration.OperationRequest{Action: "rollback", NodeID: chiURLParam(r, "id")})
}
func (s *Server) queueXNodeMigration(w http.ResponseWriter, r *http.Request, request xnodemigration.OperationRequest) {
	receipt, err := s.deps.XNodeMigration.Queue(r.Context(), currentUser(r).ID, r.Header.Get("Idempotency-Key"), request)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusAccepted, receipt)
}
