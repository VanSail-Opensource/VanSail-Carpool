package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
)

func TestActivitySettingsHTTPKeepsOneAuditedBundleAcrossFailureAndStaleActor(t *testing.T) {
	t.Parallel()
	s, _, actor := testBrowserHTTP(t)
	ctx := context.Background()
	s.deps.Settings = admin.NewSettingsService(s.deps.Store, nil)
	first := `{"timezone":"UTC","dailyRewardMinTxb":"1.00","dailyRewardMaxTxb":"2.00","groupMessageThreshold":7,"groupMessageRewardTxb":"3.00"}`
	next := `{"timezone":"Asia/Shanghai","dailyRewardMinTxb":"4.00","dailyRewardMaxTxb":"5.00","groupMessageThreshold":9,"groupMessageRewardTxb":"6.00"}`
	call := func(body string, status int) {
		t.Helper()
		request := httptest.NewRequest(http.MethodPut, "/api/v1/admin/activity/settings", strings.NewReader(body))
		request = request.WithContext(context.WithValue(request.Context(), userContextKey, actor))
		response := httptest.NewRecorder()
		s.adminUpdateActivitySettings(response, request)
		if response.Code != status || strings.Contains(response.Body.String(), "private database detail") {
			t.Fatalf("settings response %d, want %d: %s", response.Code, status, response.Body.String())
		}
	}
	call(first, http.StatusOK)
	initial, err := s.deps.Settings.ActivityConfig(ctx)
	if err != nil || initial.Timezone != "UTC" || initial.RewardMinMinor != 100 || initial.RewardMaxMinor != 200 || initial.GroupMessageThreshold != 7 || initial.GroupMessageRewardMinor != 300 {
		t.Fatal("HTTP settings omitted part of the committed bundle", initial, err)
	}
	if _, err = s.deps.Store.DB().ExecContext(ctx, `CREATE FUNCTION reject_activity_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.action='activity.settings.update' THEN RAISE EXCEPTION 'private database detail' USING ERRCODE='23514'; END IF; RETURN NEW; END $$`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.deps.Store.DB().ExecContext(ctx, `CREATE TRIGGER reject_activity_audit BEFORE INSERT ON audit_events FOR EACH ROW EXECUTE FUNCTION reject_activity_audit()`); err != nil {
		t.Fatal(err)
	}
	call(next, http.StatusInternalServerError)
	unchanged, err := s.deps.Settings.ActivityConfig(ctx)
	if err != nil || unchanged != initial {
		t.Fatal("failed audit left a partial settings update", unchanged, err)
	}
	if _, err = s.deps.Store.DB().ExecContext(ctx, `DROP TRIGGER reject_activity_audit ON audit_events`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.deps.Store.DB().ExecContext(ctx, `UPDATE users SET role='user' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	// The handler retains the authenticated request snapshot while the current
	// actor changes. Authorization must be rechecked in the write transaction.
	call(next, http.StatusConflict)
	unchanged, err = s.deps.Settings.ActivityConfig(ctx)
	if err != nil || unchanged != initial {
		t.Fatal("stale actor changed settings", unchanged, err)
	}
	if _, err = s.deps.Store.DB().ExecContext(ctx, `UPDATE users SET role='admin' WHERE id=$1`, actor.ID); err != nil {
		t.Fatal(err)
	}
	call(next, http.StatusOK)
	current, err := s.deps.Settings.ActivityConfig(ctx)
	if err != nil || current.Timezone != "Asia/Shanghai" || current.RewardMinMinor != 400 || current.RewardMaxMinor != 500 || current.GroupMessageThreshold != 9 || current.GroupMessageRewardMinor != 600 {
		t.Fatal("recovered update omitted part of the bundle", current, err)
	}
	var audits int
	if err = s.deps.Store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_events WHERE actor_user_id=$1 AND action='activity.settings.update' AND target_type='setting' AND target_id='activity'`, actor.ID).Scan(&audits); err != nil || audits != 2 {
		t.Fatal("failed or rejected updates retained an audit", audits, err)
	}
}
