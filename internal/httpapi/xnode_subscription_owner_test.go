package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

func TestNativeSubscriptionHTTPRejectsEarlierAuthenticatedAuthority(t *testing.T) {
	for _, state := range []string{"revived", "purged", "blocked"} {
		t.Run(state, func(t *testing.T) {
			server, _, user := testBrowserHTTP(t)
			statement := `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`
			if state == "purged" {
				statement = `UPDATE users SET account_status='purged' WHERE id=$1`
			} else if state == "blocked" {
				statement = `UPDATE users SET bot_blocked=1 WHERE id=$1`
			}
			if _, err := server.deps.Store.DB().ExecContext(context.Background(), statement, user.ID); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/api/v1/xnode/subscription", nil)
			request = request.WithContext(context.WithValue(request.Context(), userContextKey, user))
			response := httptest.NewRecorder()
			// A nil native service also proves the stale request is rejected before
			// loading the revived account's identity or credential material.
			server.deps.XNode = nil
			server.xnodeOwnedSubscription(response, request)
			if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "XNODE_CONFLICT") ||
				strings.Contains(response.Body.String(), `"url"`) || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("stale authenticated authority reached native projection: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestNativeSubscriptionHTTPPublicationDiscardsChangedOwner(t *testing.T) {
	server, _, user := testBrowserHTTP(t)
	ctx := context.Background()
	secret := xnode.SubscriptionMetadata{URL: "https://panel.example/subscription/private-new-generation"}
	value, err := server.readAuthenticatedNativeSubscription(ctx, user, func() (xnode.SubscriptionMetadata, error) {
		if _, err := server.deps.Store.DB().ExecContext(ctx, `UPDATE users SET account_generation=account_generation+1 WHERE id=$1`, user.ID); err != nil {
			return xnode.SubscriptionMetadata{}, err
		}
		return secret, nil
	})
	if err == nil || !reflect.DeepEqual(value, xnode.SubscriptionMetadata{}) {
		t.Fatalf("changed owner published private subscription: %+v, %v", value, err)
	}
	current, err := server.deps.Store.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	value, err = server.readAuthenticatedNativeSubscription(ctx, current, func() (xnode.SubscriptionMetadata, error) { return secret, nil })
	if err != nil || value.URL != secret.URL {
		t.Fatalf("fresh authenticated authority lost its subscription: %+v, %v", value, err)
	}
}
