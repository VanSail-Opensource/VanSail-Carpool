package database

import (
	"context"
	"testing"
)

func TestNativeCadenceUpdatePersistsWithoutResettingUsage(t *testing.T) {
	store, _, period := xnodeTrafficFixture(t)
	ctx := context.Background()
	period.Strategy = "WEEK"
	if err := store.SaveXNodePeriod(ctx, period); err != nil {
		t.Fatal(err)
	}
	got, err := store.XNodePeriod(ctx, period.ID)
	if err != nil || got.Strategy != "WEEK" || got.ID != period.ID {
		t.Fatalf("updated period = %+v, %v", got, err)
	}
}
