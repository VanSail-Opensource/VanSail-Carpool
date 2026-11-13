package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type privateAdminStorageError struct{}

func (privateAdminStorageError) Error() string {
	return "private SQL detail containing a secret credential"
}
func (privateAdminStorageError) SQLState() string { return "08006" }

func TestAdminErrorsRedactPrivateDetailsAndKeepRetryableFailures(t *testing.T) {
	for _, item := range []struct {
		name   string
		err    error
		status int
	}{
		{"untyped integration failure", errors.New("private endpoint https://private.example?credential=secret"), http.StatusUnprocessableEntity},
		{"storage failure", fmt.Errorf("private SQL operation: %w", privateAdminStorageError{}), http.StatusInternalServerError},
		{"deadline", fmt.Errorf("private connection: %w", context.DeadlineExceeded), http.StatusServiceUnavailable},
	} {
		t.Run(item.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			(&Server{}).adminFailure(response, httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings", nil), item.err)
			var failure apiError
			if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
				t.Fatal(err)
			}
			if response.Code != item.status || strings.Contains(response.Body.String(), "private") || strings.Contains(response.Body.String(), "secret") {
				t.Fatalf("unsafe administrator error %d: %+v", response.Code, failure)
			}
		})
	}
}
