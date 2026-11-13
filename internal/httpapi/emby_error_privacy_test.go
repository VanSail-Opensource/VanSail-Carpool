package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/emby"
)

func TestEmbyAccountProjectionRedactsHistoricalPrivateErrors(t *testing.T) {
	for _, status := range []string{emby.StatusQueued, emby.StatusProvisioning, emby.StatusPendingReview, emby.StatusFailed, emby.StatusActive} {
		account := emby.Account{ID: "account", Status: status, Retryable: status == emby.StatusQueued, LastError: "GET https://private.example/secret?api_key=private-credential response private-body"}
		projection := mapEmbyAccount(account)
		raw, err := json.Marshal(projection)
		if err != nil || projection.ErrorMessage == "" || strings.Contains(string(raw), "private") || strings.Contains(string(raw), "api_key") {
			t.Fatalf("historical private error exposed for %s: %s, %v", status, raw, err)
		}
		account.LastError = ""
		if mapEmbyAccount(account).ErrorMessage != "" {
			t.Fatal("error-free account gained a failure")
		}
	}
}
