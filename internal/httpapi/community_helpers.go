package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
)

func (s *Server) requireIdempotencyKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	value := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if value == "" || len(value) > 128 {
		s.writeError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Send an Idempotency-Key header containing 1 to 128 characters.")
		return "", false
	}
	return value, true
}

func parseMinorString(value string, allowZero bool) (int64, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return 0, errors.New("minor amount must be a canonical decimal integer")
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return 0, errors.New("minor amount must contain decimal digits only")
		}
	}
	minor, err := strconv.ParseInt(value, 10, 64)
	if err != nil || minor < 0 || (!allowZero && minor == 0) {
		return 0, errors.New("minor amount is outside its allowed range")
	}
	return minor, nil
}

func txbMajorString(minor int64) string {
	sign := ""
	if minor < 0 {
		sign = "-"
		minor = -minor
	}
	return sign + strconv.FormatInt(minor/100, 10) + "." + leftPadTwo(minor%100)
}

func leftPadTwo(value int64) string {
	if value < 10 {
		return "0" + strconv.FormatInt(value, 10)
	}
	return strconv.FormatInt(value, 10)
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (s *Server) communityFailure(w http.ResponseWriter, r *http.Request, err error) {
	status, code, message := http.StatusInternalServerError, "COMMUNITY_OPERATION_FAILED", "The request could not be completed."
	switch {
	case errors.Is(err, activity.ErrGroupBoostRequired):
		status, code, message = http.StatusForbidden, "GROUP_BOOST_REQUIRED", "Boost the group at least once to receive activity rewards."
	case errors.Is(err, activity.ErrGroupBoostUnavailable):
		status, code, message = http.StatusServiceUnavailable, "GROUP_BOOST_UNAVAILABLE", "Group boost verification is temporarily unavailable. Please try again."
	case errors.Is(err, database.ErrNotFound):
		status, code, message = http.StatusNotFound, "NOT_FOUND", "The requested record was not found."
	case errors.Is(err, database.ErrInsufficientBalance):
		status, code, message = http.StatusConflict, "INSUFFICIENT_BALANCE", "Your TXB balance is too low for this action."
	case errors.Is(err, database.ErrConflict):
		status, code, message = http.StatusConflict, "CONFLICT", "The request conflicts with the current state. Refresh and retry."
	case errors.Is(err, activity.ErrInvalidInput), errors.Is(err, coupons.ErrInvalidInput):
		status, code, message = http.StatusUnprocessableEntity, "INVALID_REQUEST", "One or more request fields are invalid."
	}
	s.writeError(w, r, status, code, message)
}
