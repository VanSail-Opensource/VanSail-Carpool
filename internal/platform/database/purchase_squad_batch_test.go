package database

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type countedPurchaseQueries struct {
	*sql.DB
	queries int
}

func (q *countedPurchaseQueries) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	q.queries++
	return q.DB.QueryContext(ctx, query, args...)
}

func TestPurchaseSquadBatchPreservesSnapshotsAndBoundsQueries(t *testing.T) {
	t.Parallel()
	ctx, store := context.Background(), newTestStore(t)
	owner := createTestUser(t, store, 497728)
	combo := saveTestCombo(t, store, "batch-snapshot", 100, 30)
	now := time.Now().UTC()
	if _, err := store.DB().ExecContext(ctx, `INSERT INTO purchases(id,user_id,combo_id,charged_txb_minor,valid_from,valid_until,status,created_at,updated_at,
		entitlement_squad_uuids,combo_squad_uuids,entitlement_addon_squad_uuids)
		SELECT 'batch-'||lpad(i::text,3,'0'),$1,$2,100,$3,$4,'queued',$3,$3,
		CASE i%3 WHEN 0 THEN '["full","full","shared"]' WHEN 1 THEN '[]' ELSE NULL END,
		'["snapshot","shared"]','["addon","shared"]' FROM generate_series(1,257) AS ids(i)`,
		owner.ID, combo.ID, stamp(now), stamp(now.Add(24*time.Hour))); err != nil {
		t.Fatal(err)
	}
	items := make([]model.Purchase, 257)
	for i := range items {
		items[i].ID = fmt.Sprintf("batch-%03d", i+1)
	}
	queries := &countedPurchaseQueries{DB: store.DB()}
	if err := hydratePurchaseSquads(ctx, queries, items); err != nil {
		t.Fatal(err)
	}
	if queries.queries != 2 {
		t.Fatalf("257 purchases used %d squad queries, want 2", queries.queries)
	}
	for index, item := range items {
		want := []string{}
		switch (index + 1) % 3 {
		case 0:
			want = []string{"full", "shared"}
		case 2:
			want = []string{"addon", "shared", "snapshot"}
		}
		if !reflect.DeepEqual(item.SquadUUIDs, want) {
			t.Fatalf("purchase %s squads = %v, want %v", item.ID, item.SquadUUIDs, want)
		}
	}
	// The real member list and administrative page both use the same batched
	// hydration while retaining their existing chronological order and cursor.
	listed, err := store.ListPurchases(ctx, owner.ID)
	if err != nil || len(listed) != len(items) {
		t.Fatal("member history changed", len(listed), err)
	}
	page, next, err := store.ListAdminPurchasesPage(ctx, "", "batch-", "", 100)
	if err != nil || len(page) != 100 || next == nil {
		t.Fatal("administrative purchase page changed", len(page), next, err)
	}
	if err := hydratePurchaseSquads(ctx, queries, nil); err != nil || queries.queries != 2 {
		t.Fatal("empty page performed a query", queries.queries, err)
	}
}
