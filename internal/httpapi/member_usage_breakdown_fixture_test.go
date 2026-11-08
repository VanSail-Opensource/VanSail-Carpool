package httpapi

import (
	"context"
	"crypto/sha256"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/accounts"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"github.com/go-chi/chi/v5"
)

// Reuse the existing signed member request helper without constructing hourly
// policy, commerce or upstream mocks for this read-only projection.
func newUsageBreakdownHTTPFixture(t *testing.T, admin bool) memberHourlyFixture {
	t.Helper()
	s, handler, user := testBrowserHTTP(t)
	ctx := context.Background()
	role := "user"
	if admin {
		role = "admin"
	}
	if _, err := s.deps.Store.DB().ExecContext(ctx, `UPDATE users SET role=$2,onboarding_state='complete',created_at=$3 WHERE id=$1`, user.ID, role, time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	user.Role, user.OnboardingState = role, "complete"
	s.deps.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	s.deps.Accounts = accounts.NewService(s.deps.Store, s.deps.Store, nil, nil, nil, nil, nil, time.Hour)
	s.deps.XNode = xnode.NewService(s.deps.Store, nil, nil, nil, nil)
	token, err := ids.Token(32)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256([]byte(token))
	if err = s.deps.Store.ReplaceSession(ctx, hash[:], user.ID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	router := handler.(chi.Router)
	router.Group(func(r chi.Router) {
		r.Use(s.requireSignedRequest, s.requireSession)
		s.mountXNodeMembers(r)
	})
	return memberHourlyFixture{server: s, router: router, user: user, token: token}
}

func seedUsageBreakdownHTTPRow(t *testing.T, f memberHourlyFixture, owner, node, upload string) {
	t.Helper()
	ctx, now := context.Background(), time.Now().UTC().Truncate(time.Minute)
	if _, err := f.server.deps.Store.DB().ExecContext(ctx, `UPDATE users SET created_at=$2 WHERE id=$1`, owner, now.Add(-24*time.Hour).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.server.deps.Store.DB().ExecContext(ctx, `INSERT INTO xnode_usage_minutes
 (owner_id,billing_mode,reference_id,proxy_user_id,period_id,node_id,minute,observed_from,observed_through,upload_bytes,download_bytes)
 VALUES($1,'combo','historical-reference',1,'historical-period',$2,$3,$3,$3,$4::numeric,0)`, owner, node, now, upload); err != nil {
		t.Fatal(err)
	}
}
