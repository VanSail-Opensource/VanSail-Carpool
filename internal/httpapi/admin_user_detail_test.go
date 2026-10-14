package httpapi

import (
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/admin"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

func TestMapAdminUserDetailRestoresOwnerIDsOnNestedRecords(t *testing.T) {
	t.Parallel()
	detail := admin.UserDetail{User: model.User{ID: "user-1"},
		ActiveCombo:  &model.Purchase{ID: "active", UserID: "user-1"},
		Entitlements: []model.Purchase{{ID: "active", UserID: "user-1"}},
		Payments:     []model.PaymentOrder{{ID: "payment-1", UserID: "user-1"}}}

	response := mapAdminUserDetail(detail)
	if response.ActiveCombo == nil || response.ActiveCombo.UserID != "user-1" ||
		len(response.Entitlements) != 1 || response.Entitlements[0].UserID != "user-1" ||
		len(response.Payments) != 1 || response.Payments[0].UserID != "user-1" {
		t.Fatalf("mapped detail = %+v", response)
	}
	if response.EmbyAccounts == nil {
		t.Fatal("Emby accounts must serialize as an array")
	}
	if response.IPBlocks == nil {
		t.Fatal("IP blocks must serialize as an array")
	}
}
