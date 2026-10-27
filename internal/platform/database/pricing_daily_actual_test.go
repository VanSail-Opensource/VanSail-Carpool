package database

import (
	"context"
	"testing"
	"time"
)

func TestPricingActualUsesDailyTrafficWindows(t *testing.T) {
	s, identity, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	day := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	for offset, bytes := range []int64{11, 23} {
		_, err := s.db.ExecContext(ctx, `INSERT INTO xnode_daily_usage(proxy_user_id,node_id,period_id,date,upload_bytes,download_bytes) VALUES(?,'node',?,?,?,0)`, identity.ProxyUserID, period.ID, day.AddDate(0, 0, offset).Format(time.DateOnly), bytes)
		if err != nil {
			t.Fatal(err)
		}
	}
	for offset, expected := range []float64{11, 23} {
		start := day.AddDate(0, 0, offset)
		actual, status, err := s.pricingActual(ctx, "user_node", identity.UserID+":node", "traffic_bytes", start, start.AddDate(0, 0, 1))
		if err != nil || status != "COMPLETE" || actual == nil || *actual != expected {
			t.Fatalf("day %d: actual=%v status=%s err=%v", offset, actual, status, err)
		}
	}
	actual, status, err := s.pricingActual(ctx, "user_node", identity.UserID+":node", "traffic_bytes", day.AddDate(0, 0, 2), day.AddDate(0, 0, 3))
	if err != nil || status != "MISSING" || actual != nil {
		t.Fatalf("missing day: actual=%v status=%s err=%v", actual, status, err)
	}
}
