package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/activity"
)

func TestMemberDrawPreviewIncludesPrizesAndOmitsPrivateFields(t *testing.T) {
	empty := int64(0)
	draw := activity.LuckyDraw{LuckyDrawInput: activity.LuckyDrawInput{
		ID: "draw-1", Name: "Autumn draw", Enabled: true,
		Prizes: []activity.PrizeInput{
			{ID: "available", Name: "50 TXB", Weight: 5},
			{ID: "bonus", Name: "Bonus", Weight: 1, StockRemaining: &empty},
		},
	}}
	got := mapLuckyDraw(draw)
	if len(got.Prizes) != 2 || got.Prizes[0].ID != "available" || got.Prizes[0].Name != "50 TXB" || got.Prizes[1].ID != "bonus" {
		t.Fatalf("member preview = %#v", got.Prizes)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "weight") || strings.Contains(string(encoded), "stockRemaining") {
		t.Fatalf("private prize data leaked: %s", encoded)
	}
}

func TestDrawReceiptKeepsSelectedPrizeIdentity(t *testing.T) {
	got := mapDrawResult(activity.DrawResult{
		ID: "result-1", DrawID: "draw-1", PrizeID: "prize-2", PrizeName: "50 TXB",
	})
	if got.Kind != "draw" || got.DrawID != "draw-1" || got.PrizeID != "prize-2" || got.PrizeName != "50 TXB" {
		t.Fatalf("draw receipt = %#v", got)
	}
}
