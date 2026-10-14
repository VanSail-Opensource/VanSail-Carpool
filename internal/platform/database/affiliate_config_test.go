package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/affiliates"
)

func TestAffiliateConfigRejectsStaleVersion(t *testing.T) {
	store := newTestStore(t)
	actor := createTestUser(t, store, 1001)
	input := affiliates.ConfigInput{ExpectedVersion: 1, Tiers: []affiliates.Tier{{Name: "Starter", Threshold: 0, Enabled: true, Reward: affiliates.Reward{Kind: "none"}}}}
	first, err := store.SaveAffiliateConfig(context.Background(), actor.ID, input, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != 2 {
		t.Fatalf("version = %d, want 2", first.Version)
	}
	_, err = store.SaveAffiliateConfig(context.Background(), actor.ID, input, time.Now().UTC())
	if !errors.Is(err, affiliates.ErrVersionConflict) {
		t.Fatalf("error = %v, want version conflict", err)
	}
}
