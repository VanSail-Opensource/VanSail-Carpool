package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) adminAPITokens(w http.ResponseWriter, r *http.Request) {
	privateSubscriptionHeaders(w)
	items, err := s.deps.APITokens.List(r.Context(), currentUser(r))
	if err != nil {
		s.apiTokenFailure(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) adminCreateAPIToken(w http.ResponseWriter, r *http.Request) {
	var input apitoken.CreateInput
	if err := decodeJSON(w, r, &input); err != nil {
		s.apiTokenFailure(w, r, apitoken.ErrInvalid)
		return
	}
	issued, err := s.deps.APITokens.Create(r.Context(), currentUser(r), input)
	if err != nil {
		s.apiTokenFailure(w, r, err)
		return
	}
	privateSubscriptionHeaders(w)
	writeJSON(w, http.StatusCreated, issued)
}

func (s *Server) adminRevokeAPIToken(w http.ResponseWriter, r *http.Request) {
	privateSubscriptionHeaders(w)
	if err := s.deps.APITokens.Revoke(r.Context(), currentUser(r), chiURLParam(r, "id")); err != nil {
		s.apiTokenFailure(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adminOperationReceipt(w http.ResponseWriter, r *http.Request) {
	receipt, err := s.deps.Store.ProviderOperationForPrincipal(r.Context(), chiURLParam(r, "operationId"), currentUser(r).ID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			s.writeError(w, r, 404, "NOT_FOUND", "The operation was not found.")
		} else {
			s.adminFailure(w, r, err)
		}
		return
	}
	if principal := apiPrincipal(r); principal != nil && !principal.AllowsOperation(receipt.Kind) {
		s.writeError(w, r, http.StatusForbidden, "API_TOKEN_SCOPE_REQUIRED", "This API token does not permit this operation.")
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}
