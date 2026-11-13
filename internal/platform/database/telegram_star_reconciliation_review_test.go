package database

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

func TestTelegramStarRefundReviewAndCursorSurviveRecoveryInspection(t *testing.T) {
	store, ctx := newTestStore(t), context.Background()
	now := time.Now().UTC()
	proof := TelegramStarRefundReview{BotID: 123, OrderID: "historically-purged-order", ChargeID: "charge", PayableAmount: "25",
		Currency: "XTR", TelegramID: 42, ProofHash: strings.Repeat("a", 64), Reason: "PAYMENT_REFUND_ORDER_NOT_FOUND"}
	for range 2 {
		if err := store.RecordTelegramStarRefundReview(ctx, proof, now); err != nil {
			t.Fatal(err)
		}
	}
	events, err := store.ListAuditEvents(ctx, 100)
	if err != nil {
		t.Fatalf("review audit = %+v, %v", events, err)
	}
	var matched int
	var retained TelegramStarRefundReview
	for _, event := range events {
		if event.Action != "billing.stars.refund_review_required" || event.TargetID != proof.OrderID {
			continue
		}
		matched++
		if json.Unmarshal([]byte(event.Detail), &retained) != nil || retained != proof {
			t.Fatalf("retained refund proof = %s", event.Detail)
		}
	}
	if matched != 1 {
		t.Fatalf("deduplicated refund reviews = %d", matched)
	}
	if err := store.EnsureTelegramStarReconciliation(ctx, 123, now); err != nil {
		t.Fatal(err)
	}
	before, err := postgres.InspectRecovery(ctx, store.DB(), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := store.TelegramStarReconciliationCursor(ctx, 123)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AdvanceTelegramStarReconciliation(ctx, cursor, 100, strings.Repeat("b", 64), now.Add(time.Second), now); err != nil {
		t.Fatal(err)
	}
	after, err := postgres.InspectRecovery(ctx, store.DB(), make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	find := func(report postgres.Report) postgres.TableDigest {
		for _, table := range report.Tables {
			if table.Name == "telegram_star_reconciliation" {
				return table
			}
		}
		t.Fatal("Stars cursor is missing from native recovery contract")
		return postgres.TableDigest{}
	}
	old, current := find(before), find(after)
	if old.Rows != 1 || current.Rows != 1 || old.Hash == current.Hash {
		t.Fatalf("cursor recovery digest = %+v -> %+v", old, current)
	}
}
