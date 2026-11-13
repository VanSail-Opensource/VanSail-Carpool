package database

import (
	"fmt"
	"testing"
	"time"
)

type adminTimestampFixture struct {
	store              *Store
	owner              string
	recordIDs, userIDs []string
}

func newAdminTimestampFixture(t *testing.T) adminTimestampFixture {
	t.Helper()
	store := newTestStore(t)
	base := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	times := []time.Time{base.Add(time.Second), base.Add(time.Nanosecond), base, base.Add(500 * time.Millisecond), base.Add(500 * time.Millisecond), base.Add(-time.Nanosecond)}
	var users, ids []string
	for index := range times {
		user := createTestUser(t, store, 95_001+int64(index))
		users = append(users, user.ID)
		ids = append(ids, fmt.Sprintf("00000000-0000-4000-8000-%012d", index+1))
	}
	combo := saveTestCombo(t, store, "exact-page", 100, 30)
	for index, at := range times {
		id, userID := ids[index], users[index]
		usageBreakdownExec(t, store, `UPDATE users SET created_at=$2,telegram_first_name='Exact page' WHERE id=$1`, userID, stamp(at))
		usageBreakdownExec(t, store, `INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,valid_from,valid_until,status,created_at,updated_at)
 VALUES($1,$2,$3,100,$4,$5,'active',$4,$4)`, id, userID, combo.ID, stamp(at), stamp(at.Add(24*time.Hour)))
		usageBreakdownExec(t, store, `INSERT INTO payment_orders(id,user_id,provider,method_id,status,txb_minor,payable_amount,payable_currency,rate_snapshot,expires_at,created_at,updated_at)
 VALUES($1,$2,'ezpay','exact-page','refunded',100,'1','USD','1',$3,$3,$3)`, id, users[0], stamp(at))
		usageBreakdownExec(t, store, `INSERT INTO refunds(id,payment_order_id,txb_minor,reason,status,created_at)
 VALUES($1,$1,100,'exact-page','completed',$2)`, id, stamp(at))
		usageBreakdownExec(t, store, `INSERT INTO outbox_jobs(id,kind,payload,status,available_at,created_at,updated_at)
 VALUES($1,'exact_page','{}','failed',$2,$2,$2)`, id, stamp(at))
		usageBreakdownExec(t, store, `INSERT INTO provider_operations(id,actor_user_id,owner_user_id,kind,idempotency_key,request_fingerprint,status,created_at,updated_at)
 VALUES($1,$2,$2,'admin_entitlement_edit',$1,'timestamp-page-fingerprint','pending_review',$3,$3)`, id, users[0], stamp(at))
		usageBreakdownExec(t, store, `INSERT INTO audit_events(id,action,target_type,target_id,created_at)
 VALUES($1,'timestamp_page','user',$2,$3)`, id, users[0], stamp(at))
		usageBreakdownExec(t, store, `INSERT INTO backup_runs(id,path,status,created_at)
 VALUES($1,'/tmp/exact-page.zip','complete',$2)`, id, stamp(at))
	}
	fixture := adminTimestampFixture{store: store, owner: users[0]}
	// Descending instants, then descending ID for the equal 500ms records.
	for _, index := range []int{0, 4, 3, 1, 2, 5} {
		fixture.recordIDs = append(fixture.recordIDs, ids[index])
		fixture.userIDs = append(fixture.userIDs, users[index])
	}
	return fixture
}
