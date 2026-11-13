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
	if err := store.DB().QueryRowContext(ctx, `SELECT activation_code_hash FROM squad_product_overrides WHERE group_id=$1`, squadID).Scan(&hash); err != nil {
		t.Fatalf("load activation hash: %v", err)
	}
	if hash == "secret-code" || bcrypt.CompareHashAndPassword([]byte(hash), []byte("secret-code")) != nil {
		t.Fatalf("stored activation value is not a bcrypt hash: %q", hash)
	}
}

func TestRequiredActivationGateCannotLoseItsRetainedHash(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	user := createTestUser(t, store, 26_501)
	squad := saveTestSquad(t, store, "damaged-activation", 100, true)
	if _, err := store.SaveSquadProduct(ctx, SquadProductInput{GroupID: squad.GroupID,
		Name: squad.Name, PriceTXBMinor: 100, Visible: true, UpstreamPresent: true,
		ActivationRequired: true, ActivationCode: "retained-code"}); err != nil {
		t.Fatal(err)
	}
	combo := saveTestCombo(t, store, "Damaged activation gate", 100, 30, squad.GroupID)
	now := time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)
	if _, err := store.AdjustBalance(ctx, user.ID, 1000, "damaged-gate-seed", "seed", now); err != nil {
		t.Fatal(err)
	}
	for _, hash := range []any{nil, ""} {
		if _, err := store.DB().ExecContext(ctx, `UPDATE squad_product_overrides SET activation_code_hash=$1 WHERE group_id=$2`, hash, squad.GroupID); err != nil {
			t.Fatal(err)
		}
		if _, err := store.CreatePurchase(ctx, PurchaseInput{UserID: user.ID, ComboID: combo.ID,
			IdempotencyKey: "damaged-gate-purchase"}, now); !errors.Is(err, ErrConflict) {
			t.Fatalf("required gate with missing hash allowed purchase: %v", err)
		}
	}
	var purchases int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM purchases WHERE user_id=$1`, user.ID).Scan(&purchases); err != nil || purchases != 0 {
		t.Fatalf("rejected gate left purchase: %d, %v", purchases, err)
	}
	if balance := adminWorkflowBalance(t, store, user.ID); balance != 1000 {
		t.Fatalf("rejected gate charged wallet: %d", balance)
	}
}
