package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func insertManualHourlyEnrollment(t *testing.T, store *Store, owner string, generation int64, group, status string) {
	t.Helper()
	id := fmt.Sprintf("admin-hourly-%s-%s-%d", group, status, generation)
	_, err := store.DB().ExecContext(context.Background(), `INSERT INTO hourly_enrollments(id,owner_id,account_generation,
		group_id,activation_key,request_fingerprint,status,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$8)`, id, owner, generation, group,
		"admin-hourly-"+id, strings.Repeat("a", 64), status, time.Now().UTC())
	if err != nil {
		t.Fatalf("insert %s enrollment: %v", status, err)
	}
}

func TestListAdminHourlyEnrollmentsReturnsEveryStatusForCurrentGeneration(t *testing.T) {
	t.Parallel()
	store, snapshot, input := hourlyBillingFixture(t)
	ctx := context.Background()
	active := activateHourlyBillingFixture(t, store, input, snapshot).Enrollment
	addHourlyBillingOffer(t, store, hourlyBillingAdmin(t, store), "billing-hourly-b", 250)
	addHourlyBillingOffer(t, store, hourlyBillingAdmin(t, store), "billing-hourly-c", 350)
	insertManualHourlyEnrollment(t, store, input.OwnerID, input.AccountGeneration, "billing-hourly-b", "disabled")
	insertManualHourlyEnrollment(t, store, input.OwnerID, input.AccountGeneration, "billing-hourly-c", "closed")
	insertManualHourlyEnrollment(t, store, input.OwnerID, input.AccountGeneration+1, "billing-hourly-b", "disabled")

	rows, err := store.ListAdminHourlyEnrollments(ctx, input.OwnerID, input.AccountGeneration)
	if err != nil {
		t.Fatalf("ListAdminHourlyEnrollments(): %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %+v, want three current-generation enrollments", rows)
	}
	byGroup := make(map[string]AdminHourlyEnrollment, len(rows))
	for _, row := range rows {
		byGroup[row.GroupID] = row
	}
	current, ok := byGroup[active.GroupID]
	if !ok || current.EnrollmentID != active.ID || current.Status != "active" || !current.OfferEnabled ||
		current.HourlyPriceTXBMinor != 100 || current.Name != "billing-hourly-a" ||
		current.ActivatedAt == nil || current.PaidThrough == nil {
		t.Fatalf("active row = %+v", current)
	}
	if row := byGroup["billing-hourly-b"]; row.Status != "disabled" || row.Name != "billing-hourly-b" || row.HourlyPriceTXBMinor != 250 {
		t.Fatalf("disabled row = %+v", row)
	}
	if row := byGroup["billing-hourly-c"]; row.Status != "closed" || row.Name != "billing-hourly-c" {
		t.Fatalf("closed row = %+v", row)
	}
}

func TestDisableAdminHourlyEnrollmentStopsWithoutCooldownOrPaymentChange(t *testing.T) {
	t.Parallel()
	store, snapshot, input := hourlyBillingFixture(t)
	ctx := context.Background()
	active := activateHourlyBillingFixture(t, store, input, snapshot).Enrollment
	beforeBalance := hourlyBillingBalance(t, store, input.OwnerID)

	stale := snapshot
	stale.Version = strings.Repeat("0", 64)
	if _, err := store.DisableAdminHourlyEnrollment(ctx, AdminHourlyDisableInput{OwnerID: input.OwnerID,
		AccountGeneration: input.AccountGeneration, EnrollmentID: active.ID}, stale, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale policy stop = %v", err)
	}
	other := createTestUser(t, store, 89_001)
	if _, err := store.DisableAdminHourlyEnrollment(ctx, AdminHourlyDisableInput{OwnerID: other.ID,
		AccountGeneration: input.AccountGeneration, EnrollmentID: active.ID}, snapshot, time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign owner stop = %v", err)
	}

	disabled, err := store.DisableAdminHourlyEnrollment(ctx, AdminHourlyDisableInput{OwnerID: input.OwnerID,
		AccountGeneration: input.AccountGeneration, EnrollmentID: active.ID}, snapshot, time.Now())
	if err != nil {
		t.Fatalf("DisableAdminHourlyEnrollment(): %v", err)
	}
	if disabled.Status != "disabled" || disabled.CooldownUntil != nil || disabled.DisabledAt == nil {
		t.Fatalf("administrator stop = %+v", disabled)
	}
	if disabled.PaidThrough == nil || active.PaidThrough == nil || !disabled.PaidThrough.Equal(*active.PaidThrough) {
		t.Fatalf("paid hour was not retained: before=%v after=%v", active.PaidThrough, disabled.PaidThrough)
	}
	if hourlyBillingBalance(t, store, input.OwnerID) != beforeBalance {
		t.Fatal("administrator stop moved TXB")
	}
	if got := countHourlySyncJobs(t, store, input.OwnerID); got == 0 {
		t.Fatal("administrator stop did not queue a user sync")
	}
	afterDisable := countHourlySyncJobs(t, store, input.OwnerID)
	again, err := store.DisableAdminHourlyEnrollment(ctx, AdminHourlyDisableInput{OwnerID: input.OwnerID,
		AccountGeneration: input.AccountGeneration, EnrollmentID: active.ID}, snapshot, time.Now())
	if err != nil || again.Status != "disabled" {
		t.Fatalf("idempotent stop = %+v, %v", again, err)
	}
	if got := countHourlySyncJobs(t, store, input.OwnerID); got != afterDisable {
		t.Fatalf("replay enqueued another sync: %d", got)
	}
}

func countHourlySyncJobs(t *testing.T, store *Store, owner string) int {
	t.Helper()
	var count int
	if err := store.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM outbox_jobs
		WHERE kind='xnode_sync_user' AND payload::jsonb->>'userId'=$1`, owner).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}
