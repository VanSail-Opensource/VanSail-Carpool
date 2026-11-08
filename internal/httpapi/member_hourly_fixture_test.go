package httpapi

import (
	"context"
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/hourlybilling"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/secret"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/requestauth"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/go-chi/chi/v5"
)

type memberHourlyFixture struct {
	server *Server
	router http.Handler
	user   model.User
	token  string
	policy model.HourlyPolicySnapshot
}

func newMemberHourlyFixture(t *testing.T) memberHourlyFixture {
	t.Helper()
	s, handler, user := testBrowserHTTP(t)
	ctx := context.Background()
	if _, err := s.deps.Store.DB().ExecContext(ctx, `UPDATE users SET onboarding_state='complete' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	user.OnboardingState = "complete"
	vault, err := secret.NewVault([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	s.deps.Settings = admin.NewSettingsService(s.deps.Store, vault)
	policy := model.DefaultHourlyPolicy()
	policy.SalesEnabled, policy.DailyAllowanceBytes, policy.ReducedRateBytesPerSecond, policy.ExtraGBPriceTXBMinor = true, 9007199254740993, 1024, 25
	snapshot, err := s.deps.Settings.SaveHourlyPolicy(ctx, user.ID, "", policy)
	if err != nil {
		t.Fatal(err)
	}
	s.deps.Hourly = hourlybilling.New(s.deps.Store, s.deps.Settings, nil, vault)
	s.deps.Accounts = accounts.NewService(s.deps.Store, nil, nil, nil, nil, nil, []int64{user.TelegramID}, time.Hour)
	if err := s.deps.Store.SaveXNodeGroup(ctx, xnode.Group{ID: "member-hourly", Name: "Hourly hidden legacy", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.deps.Store.SaveHourlyOffer(ctx, user.ID, model.HourlyOffer{GroupID: "member-hourly", Enabled: true, HourlyPriceTXBMinor: 100}, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.deps.Store.AdjustBalance(ctx, user.ID, 9007199254740993, "hourly-http-seed", "fixture", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.deps.Store.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	user.Role = "user"
	token, err := ids.Token(32)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(token))
	if err := s.deps.Store.ReplaceSession(ctx, hash[:], user.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	router := handler.(chi.Router)
	router.Group(func(r chi.Router) {
		r.Use(s.requireSignedRequest, s.requireSession)
		s.mountMemberOperations(r)
	})
	return memberHourlyFixture{s, router, user, token, snapshot}
}

func (f memberHourlyFixture) request(t *testing.T, method, path, body, key string, authenticated bool) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	if authenticated {
		client, err := f.server.requests.ClientKey(f.token)
		if err != nil {
			t.Fatal(err)
		}
		nonce, err := ids.Token(16)
		if err != nil {
			t.Fatal(err)
		}
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		signature, err := requestauth.Sign(client, method, requestauth.CanonicalTarget(r.URL), timestamp, nonce, []byte(body))
		if err != nil {
			t.Fatal(err)
		}
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: f.token})
		r.AddCookie(&http.Cookie{Name: requestauth.ClientKeyCookie, Value: client})
		r.Header.Set(requestauth.TimestampHeader, timestamp)
		r.Header.Set(requestauth.NonceHeader, nonce)
		r.Header.Set(requestauth.SignatureHeader, signature)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, r)
	return w
}
