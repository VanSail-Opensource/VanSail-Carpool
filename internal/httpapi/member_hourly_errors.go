package httpapi

import (
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func hourlyPolicyVersion(value string) bool {
	if len(value) != 64 || strings.ToLower(value) != value {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func hourlyDecimal(raw string, positive bool) (int64, bool) {
	value, err := strconv.ParseInt(raw, 10, 64)
	return value, err == nil && value >= 0 && (!positive || value > 0) && strconv.FormatInt(value, 10) == raw
}

func (s *Server) requireHourlyKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	key, ok := s.requireIdempotencyKey(w, r)
	if ok && len(key) > 96 {
		s.hourlyInputInvalid(w, r)
		return "", false
	}
	return key, ok
}

func (s *Server) hourlyInputInvalid(w http.ResponseWriter, r *http.Request) {
	s.writeError(w, r, http.StatusBadRequest, "HOURLY_INPUT_INVALID", "Provide valid hourly fields and decimal-string units.")
}

func (s *Server) hourlyMemberFailure(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusServiceUnavailable, "HOURLY_UNAVAILABLE", "Hourly access is temporarily unavailable."
	switch {
	case errors.Is(err, database.ErrNotFound):
		status, code, message = http.StatusNotFound, "HOURLY_ENROLLMENT_NOT_FOUND", "The hourly enrollment could not be found."
	case errors.Is(err, database.ErrInsufficientBalance):
		status, code, message = http.StatusConflict, "INSUFFICIENT_BALANCE", "Your balance does not meet the hourly activation requirement."
	case errors.Is(err, database.ErrHourlyCooldown):
		status, code, message = http.StatusConflict, "HOURLY_COOLDOWN", "This squad is in its disable cooldown."
	case errors.Is(err, database.ErrHourlyLegacyConflict):
		status, code, message = http.StatusConflict, "HOURLY_LEGACY_ACTIVE", "A fixed-term purchase blocks hourly activation."
	case errors.Is(err, database.ErrStockUnavailable):
		status, code, message = http.StatusConflict, "SQUAD_STOCK_UNAVAILABLE", "This squad has no available places."
	case errors.Is(err, database.ErrHourlyCapacity):
		status, code, message = http.StatusConflict, "HOURLY_CAPACITY", "Hourly account capacity is currently occupied."
	case errors.Is(err, database.ErrActivationCodeRequired):
		status, code, message = http.StatusUnprocessableEntity, "SQUAD_ACTIVATION_REQUIRED", "An activation code is required for this squad."
	case errors.Is(err, database.ErrActivationCodeInvalid), errors.Is(err, database.ErrActivationCodeExtra):
		status, code, message = http.StatusUnprocessableEntity, "SQUAD_ACTIVATION_INVALID", "The squad activation code is invalid."
	case errors.Is(err, database.ErrConflict), errors.Is(err, database.ErrAccountUnavailable):
		status, code, message = http.StatusConflict, "HOURLY_TERMS_CHANGED", "Reload and review the current hourly terms."
	}
	s.writeError(w, r, status, code, message)
}
