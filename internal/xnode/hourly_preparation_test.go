package xnode_test

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCancelledUnpreparedEnrollmentConfirmsAbsenceWithoutAnOnlineAgent(t *testing.T) {
	f := controlFixture(t, false)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := f.store.DB().ExecContext(ctx, `INSERT INTO hourly_enrollments(id,owner_id,account_generation,group_id,activation_key,request_fingerprint,status,created_at,updated_at) VALUES('cancelled-unprepared',$1,$2,$3,'cancel-key',$4,'closed',clock_timestamp(),clock_timestamp())`, f.user.ID, f.user.AccountGeneration, f.group.ID, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	if err := f.service.SyncHourlyEnrollment(ctx, "cancelled-unprepared"); err != nil {
		t.Fatal("certain never-issued preparation waited on offline native state", err)
	}
	var count int
	if err := f.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xnode_identities WHERE service_name='member-hourly-cancelled-unprepared'`).Scan(&count); err != nil || count != 0 {
		t.Fatal("absence confirmation created native credentials", count, err)
	}
}
