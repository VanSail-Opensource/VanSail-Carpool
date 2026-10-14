package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/validation"
	"github.com/go-chi/chi/v5"
)

const maxJSONBodyBytes = 1 << 20

func (s *Server) validateRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := validation.Request(r); err != nil {
			s.writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST_INPUT", "The request contains invalid input.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read JSON: %w", err)
	}
	if err := validation.JSONDocument(body); err != nil {
		return fmt.Errorf("validate JSON: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("request body contains multiple JSON values")
	}
	return nil
}

func chiURLParam(r *http.Request, name string) string {
	return strings.TrimSpace(chi.URLParam(r, name))
}
