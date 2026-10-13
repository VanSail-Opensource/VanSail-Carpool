package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/accounts"
	"github.com/txyyddss/Remna-User-Panel/internal/model"
	"github.com/txyyddss/Remna-User-Panel/internal/platform/ids"
	"github.com/txyyddss/Remna-User-Panel/internal/requestauth"
)

func TestBrowserMiniAppLoginRedirectsMembersOnly(t *testing.T) {
	server, handler, admin := testBrowserHTTP(t)
	server.deps.Accounts = accounts.NewService(server.deps.Store, nil, nil, nil, nil, nil, []int64{admin.TelegramID}, 7*24*time.Hour)
	member, _, err := server.deps.Store.UpsertTelegramUser(context.Background(), model.TelegramProfile{ID: 999, FirstName: "Member"}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		user model.User
		kind string
	}{{admin, "anonymous"}, {member, "member"}} {
		t.Run(test.kind, func(t *testing.T) {
			token, err := ids.Token(32)
			if err != nil {
				t.Fatal(err)
			}
			hash := sha256.Sum256([]byte(token))
			if err = server.deps.Store.ReplaceSession(context.Background(), hash[:], test.user.ID, time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/browser/session", nil)
			signBrowserTestRequest(t, server, request, token, "")
			cookies := request.Cookies()
			request.Header.Del("Cookie")
			for _, cookie := range cookies {
				if cookie.Name == browserSessionCookie {
					cookie.Name = sessionCookie
				} else if cookie.Name == browserRequestKeyCookie {
					cookie.Name = requestauth.ClientKeyCookie
				}
				request.AddCookie(cookie)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			var state struct {
				Kind          string `json:"kind"`
				Authenticated bool   `json:"authenticated"`
			}
			if err = json.Unmarshal(response.Body.Bytes(), &state); err != nil || response.Code != 200 || state.Kind != test.kind || state.Authenticated {
				t.Fatalf("Mini App fallback=%+v,status=%d,error=%v", state, response.Code, err)
			}
		})
	}
}
