package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/notifications"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestHourlyFundingWarningsDedupeWindowAndBalance(t *testing.T) {
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	e := makeDueHourlyBillingFixture(t, s, input, policy, hourlyBillingNow(t, s).Add(-4*time.Hour))
	now := hourlyBillingNow(t, s)
	paidThrough := now.Add(30 * time.Minute)
	if _, err := s.DB().Exec(`UPDATE hourly_enrollments SET paid_through=$1 WHERE id=$2`, paidThrough, e.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=50 WHERE user_id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	if count, err := s.EnqueueHourlyFundingWarnings(ctx, now); err != nil || count != 1 {
		t.Fatalf("first funding warning = %d, %v", count, err)
	}
	key := "hourly-funding-warning:" + e.ID + ":" + strconv.FormatInt(paidThrough.Unix(), 10)
	assertEventKind(t, s, key, jobpayload.UserEventHourlyFundingWarning)
	payload := notificationPayload(t, s, key)
	if payload.Facts[notifications.FactSquad] != input.GroupID || payload.Facts[notifications.FactHourlyPrice] != "100" ||
		payload.Facts[notifications.FactBalance] != "50" ||
		payload.Facts[notifications.FactTime] != paidThrough.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("funding warning facts = %+v", payload.Facts)
	}
	if count, err := s.EnqueueHourlyFundingWarnings(ctx, now.Add(time.Minute)); err != nil || count != 0 {
		t.Fatalf("replayed funding warning = %d, %v", count, err)
	}
	assertNotificationCounts(t, s, 1, 1)
	// A balance that still covers the next hour must not warn.
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=100 WHERE user_id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	paidThrough = paidThrough.Add(15 * time.Minute)
	if _, err := s.DB().Exec(`UPDATE hourly_enrollments SET paid_through=$1 WHERE id=$2`, paidThrough, e.ID); err != nil {
		t.Fatal(err)
	}
	if count, err := s.EnqueueHourlyFundingWarnings(ctx, now); err != nil || count != 0 {
		t.Fatalf("funded next hour warned = %d, %v", count, err)
	}
	// Boundaries outside (now, now+1h] are left to settlement.
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=50 WHERE user_id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	paidThrough = now.Add(2 * time.Hour)
	if _, err := s.DB().Exec(`UPDATE hourly_enrollments SET paid_through=$1 WHERE id=$2`, paidThrough, e.ID); err != nil {
		t.Fatal(err)
	}
	if count, err := s.EnqueueHourlyFundingWarnings(ctx, now); err != nil || count != 0 {
		t.Fatalf("out-of-window funding warning = %d, %v", count, err)
	}
	assertNotificationCounts(t, s, 1, 1)
}

func TestHourlySettlementSuspensionEmitsFundingNotification(t *testing.T) {
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	e := makeDueHourlyBillingFixture(t, s, input, policy, hourlyBillingNow(t, s).Add(-4*time.Hour))
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=0 WHERE user_id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SettleHourlyOwnerAvailable(ctx, input.OwnerID, input.AccountGeneration, policy,
		hourlyBillingDelivery(t, s, e.ID), time.Time{}); err != nil {
		t.Fatal(err)
	}
	suspended, err := s.HourlyEnrollment(ctx, e.ID)
	if err != nil || suspended.Status != model.HourlyFundingSuspended {
		t.Fatalf("suspended enrollment = %+v, %v", suspended, err)
	}
	key := "hourly-funding-suspended:" + e.ID + ":" + strconv.FormatInt(e.PaidThrough.Unix(), 10)
	assertEventKind(t, s, key, jobpayload.UserEventHourlyFundingSuspended)
	payload := notificationPayload(t, s, key)
	if payload.Facts[notifications.FactSquad] != input.GroupID || payload.Facts[notifications.FactHourlyPrice] != "100" ||
		payload.Facts[notifications.FactBalance] != "0" {
		t.Fatalf("funding suspension facts = %+v", payload.Facts)
	}
	assertNotificationCounts(t, s, 1, 1)
}

func notificationPayload(t *testing.T, store *Store, key string) jobpayload.UserNotification {
	t.Helper()
	var encoded string
	if err := store.DB().QueryRow(`SELECT payload_json FROM user_notification_events WHERE event_key=$1`, key).Scan(&encoded); err != nil {
		t.Fatal(err)
	}
	var payload jobpayload.UserNotification
	if err := json.Unmarshal([]byte(encoded), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestHourlyFundingWarningsAdvancePastAlreadyNotifiedPage(t *testing.T) {
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	// Keep setup funded while constructing more live enrollments than one scan.
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=1000000000 WHERE user_id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	for index := 0; index <= hourlyFundingWarningLimit; index++ {
		if index == 128 {
			other := createTestUser(t, s, 89711)
			input.OwnerID, input.AccountGeneration = other.ID, other.AccountGeneration
			if _, err := s.AdjustBalance(ctx, other.ID, 1000000000, "warning-page-seed", "fixture", time.Now()); err != nil {
				t.Fatal(err)
			}
		}
		if index > 0 {
			input.GroupID = fmt.Sprintf("funding-warning-group-%03d", index)
			input.ActivationKey = fmt.Sprintf("funding-warning-activation-%03d", index)
			addHourlyBillingOffer(t, s, hourlyBillingAdmin(t, s), input.GroupID, 100)
		}
		makeDueHourlyBillingFixture(t, s, input, policy, hourlyBillingNow(t, s).Add(-4*time.Hour))
	}
	now := hourlyBillingNow(t, s)
	if _, err := s.DB().Exec(`UPDATE hourly_enrollments SET paid_through=$1`, now.Add(30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=50`); err != nil {
		t.Fatal(err)
	}
	for index, want := range []int{hourlyFundingWarningLimit, 1, 0} {
		count, err := s.EnqueueHourlyFundingWarnings(ctx, now)
		if err != nil || count != want {
			t.Fatalf("warning scan %d = %d, %v; want %d", index+1, count, err, want)
		}
	}
	assertNotificationCounts(t, s, hourlyFundingWarningLimit+1, hourlyFundingWarningLimit+1)
}

func TestHourlyFundingWarningRetainsFinalNanosecond(t *testing.T) {
	s, policy, input := hourlyBillingFixture(t)
	ctx := context.Background()
	e := makeDueHourlyBillingFixture(t, s, input, policy, hourlyBillingNow(t, s).Add(-4*time.Hour))
	boundary := hourlyBillingNow(t, s).Truncate(time.Microsecond).Add(time.Hour)
	if _, err := s.DB().Exec(`UPDATE hourly_enrollments SET paid_through=$1 WHERE id=$2`, boundary, e.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB().Exec(`UPDATE balances SET txb_minor=50 WHERE user_id=$1`, input.OwnerID); err != nil {
		t.Fatal(err)
	}
	if count, err := s.EnqueueHourlyFundingWarnings(ctx, boundary.Add(-time.Nanosecond)); err != nil || count != 1 {
		t.Fatal("future microsecond boundary rounded to expired", count, err)
	}
}
