package httpapi

import (
	"errors"
	"net/http"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func decodeEmbyPreferences(w http.ResponseWriter, r *http.Request, withPassword bool) (emby.Preferences, string, error) {
	type preferencesRequest struct {
		MaxParentalRating  *int32    `json:"maxParentalRating"`
		DisabledLibraryIDs *[]string `json:"disabledLibraryIds"`
	}
	if !withPassword {
		var request preferencesRequest
		if err := decodeJSON(w, r, &request); err != nil {
			return emby.Preferences{}, "", err
		}
		if request.DisabledLibraryIDs == nil {
			return emby.Preferences{}, "", emby.ErrInvalidSetup
		}
		return emby.Preferences{MaxParentalRating: request.MaxParentalRating,
			DisabledLibraryIDs: *request.DisabledLibraryIDs}, "", nil
	}
	request := struct {
		Password string `json:"password"`
		preferencesRequest
	}{}
	if err := decodeJSON(w, r, &request); err != nil {
		return emby.Preferences{}, "", err
	}
	if request.Password == "" || request.DisabledLibraryIDs == nil {
		return emby.Preferences{}, "", emby.ErrInvalidSetup
	}
	return emby.Preferences{MaxParentalRating: request.MaxParentalRating,
		DisabledLibraryIDs: *request.DisabledLibraryIDs}, request.Password, nil
}

func (s *Server) setupEmby(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	preferences, password, err := decodeEmbyPreferences(w, r, true)
	if err != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_EMBY_SETUP", "Choose a password and valid viewing preferences.")
		return
	}
	receipt, err := s.deps.EmbyOperations.QueueSetup(r.Context(), currentUser(r).ID, password, preferences, key)
	if err != nil {
		code := "EMBY_SETUP_FAILED"
		if errors.Is(err, database.ErrInsufficientBalance) {
			code = "INSUFFICIENT_BALANCE"
		}
		s.writeError(w, r, http.StatusConflict, code, "Emby setup could not be queued.")
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) updateEmbyPreferences(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	preferences, _, err := decodeEmbyPreferences(w, r, false)
	if err != nil {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_EMBY_PREFERENCES", "Choose valid Emby preferences.")
		return
	}
	receipt, err := s.deps.EmbyOperations.QueuePreferences(r.Context(), currentUser(r).ID, preferences, key)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, database.ErrConflict) {
			status = http.StatusConflict
		}
		s.writeError(w, r, status, "EMBY_UPDATE_FAILED", "Emby preferences could not be updated.")
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) changeEmbyPassword(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	var request struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(w, r, &request); err != nil || request.Password == "" {
		s.writeError(w, r, http.StatusUnprocessableEntity, "INVALID_EMBY_PASSWORD", "Enter a new password.")
		return
	}
	receipt, err := s.deps.EmbyOperations.QueuePassword(r.Context(), currentUser(r).ID, request.Password, key)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, database.ErrConflict) {
			status = http.StatusConflict
		}
		s.writeError(w, r, status, "EMBY_PASSWORD_FAILED", "The Emby password could not be changed.")
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (s *Server) retryAdminEmbyAccount(w http.ResponseWriter, r *http.Request) {
	key, ok := s.requireIdempotencyKey(w, r)
	if !ok {
		return
	}
	receipt, err := s.deps.EmbyOperations.QueueProvisionRetry(r.Context(), currentUser(r).ID,
		chiURLParam(r, "id"), key)
	if err != nil {
		s.writeError(w, r, http.StatusConflict, "EMBY_RETRY_FAILED", "This Emby setup cannot be retried.")
		return
	}
	writeJSON(w, http.StatusAccepted, receipt)
}
