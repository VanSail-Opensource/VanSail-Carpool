package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	jobpayload "github.com/VanSail-Opensource/VanSail-Carpool/internal/outbox"
)

func TestSettlePaymentCreditsExactlyOnce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 10004)
	now := time.Date(2026, time.August, 7, 13, 0, 0, 0, time.UTC)
	if _, err := store.SavePaymentProfile(ctx, PaymentProfileInput{
		ID: "ezpay-main", Provider: "ezpay", ProviderName: "TX 收款一号", EnabledChannels: []string{"alipay"},
		Endpoint: "https://pay.example", MerchantID: "merchant", CredentialCiphertext: "ciphertext", Enabled: true,
	}); err != nil {
		t.Fatalf("SavePaymentProfile(): %v", err)
	}
	order := createTestPaymentOrder(t, store, user.ID, "ezpay", 2_500, now)
	if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET method_id='ezpay:ezpay-main:alipay',provider_rail='alipay' WHERE id=?`, order.ID); err != nil {
		t.Fatalf("seed payment channel: %v", err)
	}

	paid, applied, err := store.SettlePayment(ctx, "ezpay", "event-1", "payload-a", order.ID, "trade-1", "charge-1", now)
	if err != nil {
		t.Fatalf("SettlePayment(first): %v", err)
	}
	if !applied || paid.Status != "paid" {
		t.Fatalf("first settlement = (status %q, applied %t), want (paid, true)", paid.Status, applied)
	}

	replayed, applied, err := store.SettlePayment(ctx, "ezpay", "event-1", "payload-a", order.ID, "trade-1", "charge-1", now.Add(time.Minute))
	if err != nil {
		t.Fatalf("SettlePayment(replay): %v", err)
	}
	if applied || replayed.Status != "paid" {
		t.Fatalf("replay = (status %q, applied %t), want (paid, false)", replayed.Status, applied)
	}

	if replayed, applied, err = store.SettlePayment(ctx, "ezpay", "event-1", "different-payload", order.ID, "trade-1", "charge-1", now.Add(2*time.Minute)); err != nil || applied || replayed.Status != "paid" {
		t.Fatalf("metadata-varied replay = (status %q, applied %t, error %v), want (paid, false, nil)", replayed.Status, applied, err)
	}
	otherOrder := createTestPaymentOrder(t, store, user.ID, "ezpay", 2_500, now.Add(3*time.Minute))
	if _, _, err := store.SettlePayment(ctx, "ezpay", "event-1", "payload-a", otherOrder.ID, "trade-1", "charge-1", now.Add(4*time.Minute)); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-order replay error = %v, want ErrConflict", err)
	}
	balance, err := store.Balance(ctx, user.ID)
	if err != nil {
		t.Fatalf("Balance(): %v", err)
	}
	if balance.Minor != "2500" {
		t.Fatalf("balance = %s, want 2500", balance.Minor)
	}
	entries, err := store.ListLedger(ctx, user.ID, 20)
	if err != nil {
		t.Fatalf("ListLedger(): %v", err)
	}
	if got := countLedgerKind(entries, "payment_credit"); got != 1 {
		t.Fatalf("payment credit count = %d, want 1", got)
	}
	var announcementJSON string
	if err := store.DB().QueryRowContext(ctx, `SELECT payload FROM outbox_jobs WHERE kind=?`,
		jobpayload.PaymentSuccessAnnouncementKind).Scan(&announcementJSON); err != nil {
		t.Fatalf("load payment announcement job: %v", err)
	}
	var announcement jobpayload.PaymentSuccessAnnouncement
	if err := json.Unmarshal([]byte(announcementJSON), &announcement); err != nil {
		t.Fatalf("decode payment announcement job: %v", err)
	}
	if announcement.OrderID != order.ID || announcement.Provider != "ezpay" || announcement.ProviderName != "TX 收款一号" || announcement.Channel != "alipay" ||
		announcement.TXBMinor != 2_500 || announcement.PayableAmount != "10.00" || announcement.PayableCurrency != "CNY" ||
		announcement.Username != "@telegram10004" {
		t.Fatalf("payment announcement snapshot = %+v", announcement)
	}
	var announcementCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE kind=?`,
		jobpayload.PaymentSuccessAnnouncementKind).Scan(&announcementCount); err != nil || announcementCount != 1 {
		t.Fatalf("payment announcement jobs = %d, error %v", announcementCount, err)
	}
	assertRowCount(t, store, "webhook_events", 0)
	assertRowCount(t, store, "payment_callback_tombstones", 1)
}

func TestSettlePaymentOverflowRollsBackWebhookAndCredit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 10009)
	now := time.Date(2026, time.August, 8, 10, 0, 0, 0, time.UTC)
	order := createTestPaymentOrder(t, store, user.ID, "ezpay", 1, now)
	if _, err := store.AdjustBalance(ctx, user.ID, math.MaxInt64, "payment-overflow-max", "test", now); err != nil {
		t.Fatalf("AdjustBalance(max): %v", err)
	}
	if _, _, err := store.SettlePayment(ctx, "ezpay", "overflow-event", "overflow-payload", order.ID, "overflow-trade", "", now.Add(time.Second)); err == nil {
		t.Fatal("SettlePayment(overflow) unexpectedly succeeded")
	}
	current, err := store.PaymentOrderByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("PaymentOrderByID(): %v", err)
	}
	if current.Status != "pending" {
		t.Fatalf("order status = %q, want pending", current.Status)
	}
	assertRowCount(t, store, "webhook_events", 0)
	balance, err := store.Balance(ctx, user.ID)
	if err != nil {
		t.Fatalf("Balance(): %v", err)
	}
	if balance.Minor != "9223372036854775807" {
		t.Fatalf("balance = %s, want MaxInt64", balance.Minor)
	}
	entries, err := store.ListLedger(ctx, user.ID, 10)
	if err != nil {
		t.Fatalf("ListLedger(): %v", err)
	}
	if got := countLedgerKind(entries, "payment_credit"); got != 0 {
		t.Fatalf("payment credit count = %d, want 0", got)
	}
	assertRowCount(t, store, "outbox_jobs", 0)
}

func TestPaymentTerminalTransitionsMinimizeProviderData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		transition       string
		receivingAddress bool
		wantQR           bool
		wantStatus       string
	}{
		{name: "failed", transition: "fail", wantStatus: "failed"},
		{name: "paid", transition: "settle", wantStatus: "paid"},
		{name: "refunded", transition: "refund", wantStatus: "refunded"},
		{name: "expired with separate address", transition: "expire", receivingAddress: true, wantStatus: "expired"},
		{name: "expired legacy QR address", transition: "expire", wantQR: true, wantStatus: "expired"},
		{name: "stale with separate address", transition: "stale", receivingAddress: true, wantStatus: "expired"},
		{name: "stale legacy QR address", transition: "stale", wantQR: true, wantStatus: "expired"},
		{name: "insert-time stale with separate address", transition: "insert-stale", receivingAddress: true, wantStatus: "expired"},
		{name: "insert-time stale legacy QR address", transition: "insert-stale", wantQR: true, wantStatus: "expired"},
		{name: "cancelled with separate address", transition: "cancel", receivingAddress: true, wantStatus: "cancelled"},
		{name: "cancelled legacy QR address", transition: "cancel", wantQR: true, wantStatus: "cancelled"},
	}

	for index, test := range tests {
		index, test := index, test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			store := newTestStore(t)
			user := createTestUser(t, store, int64(33000+index))
			now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
			status := "pending"
			if test.transition == "fail" {
				status = "creating"
			}
			order, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{
				UserID: user.ID, Provider: "ezpay", Status: status, TXBMinor: 100,
				PayableAmount: "1.00", PayableCurrency: "CNY", RateSnapshot: "1",
				ExpiresAt: now.Add(time.Hour),
			})
			if err != nil {
				t.Fatalf("CreatePaymentOrder(): %v", err)
			}
			var initialPayload string
			if err := store.DB().QueryRowContext(ctx, `SELECT provider_payload FROM payment_orders WHERE id=?`, order.ID).Scan(&initialPayload); err != nil {
				t.Fatalf("read new provider payload: %v", err)
			}
			if initialPayload != "{}" {
				t.Fatalf("new provider payload = %q, want {}", initialPayload)
			}

			var address any
			if test.receivingAddress {
				address = "TSeparateReceiveAddress"
			}
			if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET payment_url=?,qr_payload=?,receiving_address=?,provider_payload=? WHERE id=?`,
				"https://pay.example/secret", "TLegacyQRAddress", address, `{"provider":"secret"}`, order.ID); err != nil {
				t.Fatalf("seed provider data: %v", err)
			}

			switch test.transition {
			case "fail":
				err = store.FailPaymentOrder(ctx, order.ID)
			case "settle":
				_, _, err = store.SettlePayment(ctx, "ezpay", "event-1", "hash", order.ID, "trade-1", "charge-1", now)
			case "refund":
				if _, _, err = store.SettlePayment(ctx, "ezpay", "event-1", "hash", order.ID, "trade-1", "charge-1", now); err == nil {
					_, err = store.DB().ExecContext(ctx, `UPDATE payment_orders SET payment_url=?,qr_payload=?,provider_payload=? WHERE id=?`,
						"https://pay.example/refund-secret", "TRefundQR", `{"provider":"refund-secret"}`, order.ID)
				}
				if err == nil {
					_, err = store.RefundPayment(ctx, nil, order.ID, "test refund", now.Add(time.Minute))
				}
			case "expire":
				err = store.ExpirePaymentOrder(ctx, order.ID, "ezpay", now)
			case "stale":
				if _, err = store.DB().ExecContext(ctx, `UPDATE payment_orders SET expires_at=? WHERE id=?`, stamp(now.Add(-time.Second)), order.ID); err == nil {
					err = store.ExpireStalePaymentOrders(ctx, now)
				}
			case "insert-stale":
				if _, err = store.DB().ExecContext(ctx, `UPDATE payment_orders SET expires_at=? WHERE id=?`, stamp(now.Add(-time.Second)), order.ID); err == nil {
					_, err = store.CreatePaymentOrder(ctx, model.PaymentOrder{
						UserID: user.ID, Provider: "stars", Status: "pending", TXBMinor: 100,
						PayableAmount: "1", PayableCurrency: "XTR", RateSnapshot: "1", ExpiresAt: now.Add(time.Hour),
					})
				}
			case "cancel":
				_, _, err = store.CancelPaymentOrder(ctx, order.ID, user.ID, "test cancellation", now)
			default:
				t.Fatalf("unknown transition %q", test.transition)
			}
			if err != nil {
				t.Fatalf("%s transition: %v", test.transition, err)
			}

			current, err := store.PaymentOrderByID(ctx, order.ID)
			if err != nil {
				t.Fatalf("PaymentOrderByID(): %v", err)
			}
			if current.Status != test.wantStatus {
				t.Fatalf("status = %q, want %q", current.Status, test.wantStatus)
			}
			if current.PaymentURL != nil {
				t.Fatalf("terminal payment URL = %q, want nil", *current.PaymentURL)
			}
			if gotQR := current.QRPayload != nil; gotQR != test.wantQR {
				t.Fatalf("terminal QR presence = %t, want %t", gotQR, test.wantQR)
			}
			var providerPayload string
			if err := store.DB().QueryRowContext(ctx, `SELECT provider_payload FROM payment_orders WHERE id=?`, order.ID).Scan(&providerPayload); err != nil {
				t.Fatalf("read terminal provider payload: %v", err)
			}
			if providerPayload != "{}" {
				t.Fatalf("terminal provider payload = %q, want {}", providerPayload)
			}
		})
	}
}

func TestPaymentPayloadMinimizationMigration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		status           string
		cancelled        bool
		receivingAddress bool
		wantPaymentURL   bool
		wantQR           bool
	}{
		{name: "live pending", status: "pending", wantPaymentURL: true, wantQR: true},
		{name: "paid", status: "paid"},
		{name: "failed", status: "failed"},
		{name: "refunded", status: "refunded"},
		{name: "expired with separate address", status: "expired", receivingAddress: true},
		{name: "expired legacy QR address", status: "expired", wantQR: true},
		{name: "cancelled with separate address", status: "pending", cancelled: true, receivingAddress: true},
		{name: "cancelled legacy QR address", status: "pending", cancelled: true, wantQR: true},
	}

	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 34000)
	now := time.Date(2099, time.August, 10, 13, 0, 0, 0, time.UTC)
	orderIDs := make([]string, len(tests))
	for index, test := range tests {
		order, err := store.CreatePaymentOrder(ctx, model.PaymentOrder{
			UserID: user.ID, Provider: "ezpay", Status: "pending", TXBMinor: 100,
			PayableAmount: "1.00", PayableCurrency: "CNY", RateSnapshot: "1", ExpiresAt: now.Add(time.Hour),
		})
		if err != nil {
			t.Fatalf("CreatePaymentOrder(%s): %v", test.name, err)
		}
		orderIDs[index] = order.ID
		var cancelledAt any
		if test.cancelled {
			cancelledAt = stamp(now)
		}
		var address any
		if test.receivingAddress {
			address = "TSeparateReceiveAddress"
		}
		if _, err := store.DB().ExecContext(ctx, `UPDATE payment_orders SET status=?,cancelled_at=?,payment_url=?,qr_payload=?,receiving_address=?,provider_payload=? WHERE id=?`,
			test.status, cancelledAt, "https://pay.example/legacy-secret", "TLegacyQRAddress", address, `{"provider":"legacy-secret"}`, order.ID); err != nil {
			t.Fatalf("seed migration row %s: %v", test.name, err)
		}
	}

	script, err := migrations.ReadFile("migrations/011_minimize_payment_payloads.sql")
	if err != nil {
		t.Fatalf("read payment minimization migration: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx, string(script)); err != nil {
		t.Fatalf("apply payment minimization migration: %v", err)
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var providerPayload string
			var paymentURL, qrPayload sql.NullString
			if err := store.DB().QueryRowContext(ctx, `SELECT provider_payload,payment_url,qr_payload FROM payment_orders WHERE id=?`, orderIDs[index]).
				Scan(&providerPayload, &paymentURL, &qrPayload); err != nil {
				t.Fatalf("read migrated payment order: %v", err)
			}
			if providerPayload != "{}" {
				t.Fatalf("provider payload = %q, want {}", providerPayload)
			}
			if paymentURL.Valid != test.wantPaymentURL {
				t.Fatalf("payment URL presence = %t, want %t", paymentURL.Valid, test.wantPaymentURL)
			}
			if qrPayload.Valid != test.wantQR {
				t.Fatalf("QR payload presence = %t, want %t", qrPayload.Valid, test.wantQR)
			}
		})
	}
}
