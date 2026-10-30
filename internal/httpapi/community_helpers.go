package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/coupons"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func (s *Server) requireIdempotencyKey(w http.ResponseWriter, r *http.Request) (string, bool) {
	value := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if value == "" || len(value) > 128 {
		s.writeError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Send an Idempotency-Key header containing 1 to 128 characters.")
		return "", false
	}
	return value, true
}

func optionalOrGeneratedIdempotencyKey(w http.ResponseWriter, r *http.Request) (string, error) {
	value := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(value) > 128 {
		return "", errors.New("idempotency key is too long")
	}
	if value != "" {
		return value, nil
	}
	generated, err := ids.New()
	if err != nil {
		return "", err
	}
	w.Header().Set("Idempotency-Key", generated)
	w.Header().Set("Idempotency-Key-Generated", "true")
	return generated, nil
}

func boundedMultipartFile(w http.ResponseWriter, r *http.Request, fieldName string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return nil, errors.New("invalid upload bound")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+maxMultipartOverhead)
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, err
	}
	var result []byte
	found := false
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return nil, nextErr
		}
		if part.FormName() != fieldName || part.FileName() == "" || found {
			_ = part.Close()
			return nil, errors.New("multipart upload must contain exactly one file field")
		}
		content, readErr := io.ReadAll(io.LimitReader(part, maxBytes+1))
		closeErr := part.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if int64(len(content)) > maxBytes {
			return nil, errors.New("uploaded file exceeds bound")
		}
		result = content
		found = true
	}
	if !found {
		return nil, errors.New("multipart file is missing")
	}
	return result, nil
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

func parseSignedDecimalInt64(value string) (int64, error) {
	if value == "" || strings.TrimSpace(value) != value || value == "-" {
		return 0, errors.New("signed amount must be a canonical decimal integer")
	}
	digits := value
	if strings.HasPrefix(digits, "-") {
		digits = strings.TrimPrefix(digits, "-")
	}
	for _, character := range digits {
		if character < '0' || character > '9' {
			return 0, errors.New("signed amount must contain decimal digits only")
		}
	}
	return strconv.ParseInt(value, 10, 64)
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
