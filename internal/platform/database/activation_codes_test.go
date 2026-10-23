package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

func TestPurchaseActivationCodesAreHashedAndValidated(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	user := createTestUser(t, store, 26_500)
	squadID := "activation-squad"
	squad := saveTestSquad(t, store, squadID, 100, true)
	_, err := store.SaveSquadProduct(ctx, SquadProductInput{
		ID: squad.ID, GroupID: squadID, Name: squadID, Description: "gated",
		PriceTXBMinor: 100, Visible: true, UpstreamPresent: true,
		ActivationRequired: true, ActivationCode: "secret-code",
	})
	if err != nil {
		t.Fatalf("SaveSquadProduct(gated): %v", err)
	}
	combo := saveTestCombo(t, store, "gated-combo", 100, 30, squadID)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "activation-seed", "test", time.Now()); err != nil {
		t.Fatalf("AdjustBalance(): %v", err)
	}
	base := PurchaseInput{UserID: user.ID, ComboID: combo.ID, IdempotencyKey: "activation-test"}
	if _, err := store.CreatePurchase(ctx, base, time.Now()); !errors.Is(err, ErrActivationCodeRequired) {
		t.Fatalf("missing activation code = %v, want ErrActivationCodeRequired", err)
	}
	base.SquadActivationCodes = map[string]string{squadID: "wrong-code"}
	if _, err := store.CreatePurchase(ctx, base, time.Now()); !errors.Is(err, ErrActivationCodeInvalid) {
		t.Fatalf("invalid activation code = %v, want ErrActivationCodeInvalid", err)
	}
	base.SquadActivationCodes = map[string]string{"unselected": "secret-code"}
	if _, err := store.CreatePurchase(ctx, base, time.Now()); !errors.Is(err, ErrActivationCodeExtra) {
		t.Fatalf("extra activation code = %v, want ErrActivationCodeExtra", err)
	}
	base.SquadActivationCodes = map[string]string{squadID: "secret-code"}
	if _, err := store.CreatePurchase(ctx, base, time.Now()); err != nil {
		t.Fatalf("valid activation code: %v", err)
	}
	var hash string
	if err := store.DB().QueryRowContext(ctx, `SELECT activation_code_hash FROM squad_product_overrides WHERE group_id=?`, squadID).Scan(&hash); err != nil {
		t.Fatalf("load activation hash: %v", err)
	}
	if hash == "secret-code" || bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret-code")) != nil {
		t.Fatalf("stored activation value is not a bcrypt hash: %q", hash)
	}
}
