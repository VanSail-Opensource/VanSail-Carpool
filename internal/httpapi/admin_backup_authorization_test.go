package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/backup"
)

func TestBackupDeletionAuthorityFailureReturnsForbidden(t *testing.T) {
	t.Parallel()
	server := &Server{}
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/backups/backup", nil)
	response := httptest.NewRecorder()
	server.adminFailure(response, request, fmt.Errorf("delete backup: %w", backup.ErrBackupForbidden))
	var failure apiError
	if err := json.Unmarshal(response.Body.Bytes(), &failure); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusForbidden || failure.Code != "FORBIDDEN" || failure.Message != "Administrator access is required." {
		t.Fatalf("revoked administrator received %d: %+v", response.Code, failure)
	}
}
