package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/apitoken"
)

const apiTokenContextKey contextKey = "api_token"

func apiPrincipal(r *http.Request) *apitoken.Principal {
	principal, _ := r.Context().Value(apiTokenContextKey).(*apitoken.Principal)
	return principal
}

func (s *Server) authenticateAPIToken(w http.ResponseWriter, r *http.Request, next http.Handler) {
	values := r.Header.Values("Authorization")
	parts := strings.Fields(r.Header.Get("Authorization"))
	if s.deps.APITokens == nil || len(values) != 1 || len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		s.apiTokenFailure(w, r, apitoken.ErrUnauthorized)
		return
	}
	principal, err := s.deps.APITokens.Authenticate(r.Context(), parts[1])
	if err != nil {
		s.apiTokenFailure(w, r, err)
		return
	}
	if !principal.AllowsRequest(r.Method, r.URL.Path) {
		s.writeError(w, r, http.StatusForbidden, "API_TOKEN_SCOPE_REQUIRED", "This API token does not permit this action.")
		return
	}
	if err = s.requests.CheckBearer(r, principal.Item.ID, isStreamingBackupUpload(r)); err != nil {
		s.writeRequestAuthError(w, r, err)
		return
	}
	ctx := context.WithValue(r.Context(), apiTokenContextKey, &principal)
	ctx = context.WithValue(ctx, userContextKey, principal.User)
	next.ServeHTTP(w, r.WithContext(ctx))
}

func (s *Server) apiTokenFailure(w http.ResponseWriter, r *http.Request, err error) {
	status, code := http.StatusServiceUnavailable, "API_TOKEN_UNAVAILABLE"
	switch {
	case errors.Is(err, apitoken.ErrUnauthorized):
		status, code = http.StatusUnauthorized, "API_TOKEN_INVALID"
	case errors.Is(err, apitoken.ErrInvalid):
		status, code = http.StatusUnprocessableEntity, "API_TOKEN_INPUT_INVALID"
	case errors.Is(err, apitoken.ErrLimit):
		status, code = http.StatusConflict, "API_TOKEN_LIMIT"
	case errors.Is(err, apitoken.ErrNotFound):
		status, code = http.StatusNotFound, "API_TOKEN_NOT_FOUND"
	}
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", `Bearer realm="VanSail Carpool"`)
	}
	s.writeError(w, r, status, code, "API access could not be completed.")
}
