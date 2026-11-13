package httpapi

import (
	"context"
	"encoding/json"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"net/http"
)

// Command results expose only the bounded, credential-free result selected by
// the node worker. Other provider operations are outside this administrator API.
func (s *Server) xnodeCommandResult(w http.ResponseWriter, r *http.Request) {
	operation, err := s.deps.Store.ProviderOperationByID(r.Context(), chiURLParam(r, "id"))
	if err != nil {
		s.adminFailure(w, r, err)
		return
	}
	if operation.Receipt.Kind != xnode.OperationKind {
		s.xnodeError(w, r, xnode.ErrNotFound)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusOK, map[string]any{"receipt": operation.Receipt, "result": xnode.BrowserMetricNumbers(json.RawMessage(operation.ResultJSON))})
}

func (s *Server) xnodeQueueCommand(w http.ResponseWriter, r *http.Request) {
	var input xnode.CommandRequest
	if err := decodeJSON(w, r, &input); err != nil {
		s.xnodeError(w, r, xnode.ErrInvalid)
		return
	}
	receipt, err := s.deps.XNode.QueueCommand(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), r.Header.Get("Idempotency-Key"), input)
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) xnodeCertificateOperation(w http.ResponseWriter, r *http.Request) {
	operation, err := s.deps.XNode.LatestCertificateOperation(r.Context(), chiURLParam(r, "id"), r.URL.Query().Get("profileId"))
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	operation.Result = xnode.BrowserMetricNumbers(operation.Result)
	writeJSON(w, http.StatusOK, operation)
}
func (s *Server) xnodeDeleteNode(w http.ResponseWriter, r *http.Request) {
	s.xnodeDeleteAdminResource(w, r, s.deps.XNode.DeleteNodeAs)
}
func (s *Server) xnodeDeleteGroup(w http.ResponseWriter, r *http.Request) {
	s.xnodeDeleteAdminResource(w, r, s.deps.XNode.DeleteGroupAs)
}
func (s *Server) xnodeDeleteEndpoint(w http.ResponseWriter, r *http.Request) {
	s.xnodeDeleteAdminResource(w, r, s.deps.XNode.DeleteEndpointAs)
}
func (s *Server) xnodeDeleteAdminResource(w http.ResponseWriter, r *http.Request, remove func(context.Context, string, string) error) {
	if err := remove(r.Context(), currentUser(r).ID, chiURLParam(r, "id")); err != nil {
		s.xnodeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) xnodeQueueRemoval(w http.ResponseWriter, r *http.Request) {
	receipt, err := s.deps.XNode.QueueCommand(r.Context(), currentUser(r).ID, chiURLParam(r, "id"), r.Header.Get("Idempotency-Key"), xnode.CommandRequest{Name: "node.remove"})
	if err != nil {
		s.xnodeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}
