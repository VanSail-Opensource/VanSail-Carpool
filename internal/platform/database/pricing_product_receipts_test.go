package database

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/X-Node/pkg/compute"
)

func TestPricingProductReceiptsBindProductAndTimeWindow(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	day := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	user := createTestUser(t, store, 69_032)
	combo := saveTestCombo(t, store, "receipt-window", 100, 1)
	other := saveTestCombo(t, store, "other-receipts", 200, 1)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "seed", "seed", day.AddDate(0, 0, -32)); err != nil {
		t.Fatal(err)
	}
	for _, purchase := range []struct {
		key, combo string
		days       int
	}{
		{"old", combo.ID, -31}, {"in-window", combo.ID, -1},
		{"other-product", other.ID, -1}, {"after-window", combo.ID, 0},
	} {
		if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: purchase.combo, IdempotencyKey: purchase.key}, day.AddDate(0, 0, purchase.days).Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		id   string
		want float64
	}{{combo.ID, 100}, {other.ID, 200}, {"missing-product", 0}} {
		actual, status, err := store.pricingActual(ctx, "product", "combo:"+test.id, "gross_revenue_minor", day.AddDate(0, 0, -1), day)
		if err != nil || status != "COMPLETE" || actual == nil || *actual != test.want {
			t.Fatalf("product %s actual = %v, %s, %v; want %v", test.id, actual, status, err, test.want)
		}
	}
	if err := store.PricingProductForecasts(ctx, "combo", combo.ID, day); !errors.Is(err, ErrPricingComputePending) {
		t.Fatalf("prepare receipts forecast = %v", err)
	}
	job, err := store.ClaimComputeOutboxJob(ctx, time.Now().UTC().Add(time.Second))
	if err != nil || job == nil || job.Kind != PricingComputeJob {
		t.Fatalf("forecast compute job = %+v, %v", job, err)
	}
	var payload struct {
		ID string `json:"inputId"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	task, err := store.PricingComputeInput(ctx, payload.ID)
	if err != nil {
		t.Fatal(err)
	}
	var sum float64
	for _, value := range task.Samples {
		sum += value
	}
	if len(task.Samples) != 30 || sum != 100 {
		t.Fatalf("forecast samples = %v; want 30 days containing only the in-window receipt", task.Samples)
	}
	out, err := compute.Run(ctx, compute.Batch{Version: compute.Version, Tasks: []compute.Task{task}})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AcceptPricingCompute(ctx, task, out.Items[0], day); err != nil {
		t.Fatal(err)
	}
	if err := store.PricingProductForecasts(ctx, "combo", combo.ID, day); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := store.DB().QueryRowContext(ctx, `SELECT count(*) FROM pricing_forecast_vintages WHERE entity_type='product' AND entity_id=$1`, "combo:"+combo.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("receipt forecasts = %d, %v", count, err)
	}
}

func TestPricingReceiptsKeepExactLedgerWindowBoundaries(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	until := time.Date(2030, 2, 1, 0, 0, 0, 0, time.UTC)
	from := until.AddDate(0, 0, -30)
	user := createTestUser(t, s, 69_034)
	combo := saveTestCombo(t, s, "exact receipt window", 100, 1)
	if _, err := s.AdjustBalance(ctx, user.ID, 1000, "receipt-seed", "seed", from.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	for index, at := range []time.Time{from.Add(-time.Nanosecond), from, until.Add(-time.Nanosecond), until} {
		if _, err := s.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: []string{"before", "lower", "last", "upper"}[index]}, at); err != nil {
			t.Fatal(err)
		}
	}
	actual, status, err := s.pricingActual(ctx, "product", "combo:"+combo.ID, "gross_revenue_minor", from, until)
	if err != nil || status != "COMPLETE" || actual == nil || *actual != 200 {
		t.Fatal("receipt actual widened or shrank the exact window", actual, status, err)
	}
	summary, err := s.pricingSummary(ctx, until)
	if err != nil || summary["netRevenueMinor"] != "200" {
		t.Fatal("summary changed exact ledger receipt bounds", summary, err)
	}
	if err := s.PricingProductForecasts(ctx, "combo", combo.ID, until); !errors.Is(err, ErrPricingComputePending) {
		t.Fatal(err)
	}
	job, err := s.ClaimComputeOutboxJob(ctx, time.Now().UTC().Add(time.Second))
	if err != nil || job == nil {
		t.Fatal(job, err)
	}
	var payload struct {
		ID string `json:"inputId"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	task, err := s.PricingComputeInput(ctx, payload.ID)
	if err != nil {
		t.Fatal(err)
	}
	var total float64
	for _, value := range task.Samples {
		total += value
	}
	if len(task.Samples) != 30 || total != 200 || task.Samples[0] != 100 || task.Samples[29] != 100 {
		t.Fatal("forecast evidence used imprecise receipt bounds", task.Samples)
	}
}
