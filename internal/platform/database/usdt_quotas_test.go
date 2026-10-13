package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/txyyddss/Remna-User-Panel/internal/usdt"
)

func TestUSDTRPCBudgetSurvivesExecutorReplacementAndCountsFailures(t *testing.T) {
	t.Parallel()
	s := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	g := usdt.QuotaGroup{ID: "shared", RequestsPerSecond: 1, RequestsPerMinute: 2, RequestsPerDay: 3, Concurrency: 1}
	if err := s.AdmitUSDTRPC(ctx, g, now); err != nil {
		t.Fatal(err)
	}
	var pause *usdt.Pause
	if err := s.AdmitUSDTRPC(ctx, g, now); !errors.As(err, &pause) || pause.Code != "USDT_RPC_COOLDOWN" {
		t.Fatalf("burst=%v", err)
	}
	restarted := NewStore(s.db)
	if err := restarted.AdmitUSDTRPC(ctx, g, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := restarted.AdmitUSDTRPC(ctx, g, now.Add(2*time.Second)); !errors.As(err, &pause) || pause.Code != "USDT_RPC_QUOTA_EXHAUSTED" {
		t.Fatalf("minute=%v", err)
	}
	if err := s.AdmitUSDTRPC(ctx, g, now.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.AdmitUSDTRPC(ctx, g, now.Add(4*time.Minute)); !errors.As(err, &pause) || pause.Code != "USDT_RPC_QUOTA_EXHAUSTED" {
		t.Fatalf("day=%v", err)
	}
	status, err := s.USDTQuotaStatus(ctx, []usdt.QuotaGroup{g}, now.Add(4*time.Minute))
	if err != nil || status[0].DayUsed != 3 {
		t.Fatalf("quota=%v %v", status, err)
	}
	if err := s.FinishUSDTRPC(ctx, g.ID, now.Add(time.Hour), false); err != nil {
		t.Fatal(err)
	}
	if err := restarted.AdmitUSDTRPC(ctx, g, now.Add(10*time.Minute)); !errors.As(err, &pause) || pause.Code != "USDT_RPC_COOLDOWN" {
		t.Fatalf("cooldown=%v", err)
	}
	if err := s.FinishUSDTRPC(ctx, g.ID, time.Time{}, true); err != nil {
		t.Fatal(err)
	}
	if err := s.AdmitUSDTRPC(ctx, g, now.Add(25*time.Hour)); err != nil {
		t.Fatalf("rolling reset=%v", err)
	}
}
