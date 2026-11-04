package databaseadmin

import (
	"context"
	"encoding/json"
	"errors"

	"testing"
)

func TestSchemaRowsAndCursorAreTypedAndAllowlisted(t *testing.T) {
	t.Parallel()
	service, db, _, _, actor := newTestService(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE keyed_records(label TEXT COLLATE "C" PRIMARY KEY,amount BIGINT NOT NULL,enabled BIGINT NOT NULL DEFAULT 0)`); err != nil {
		t.Fatalf("create rowid table: %v", err)
	}
	for _, values := range []struct {
		label   string
		amount  int64
		enabled int
	}{{"first", 9007199254740993, 1}, {"second", 2, 0}, {"third", 3, 1}} {
		if _, err := db.ExecContext(ctx, `INSERT INTO keyed_records(label,amount,enabled) VALUES($1,$2,$3)`, values.label, values.amount, values.enabled); err != nil {
			t.Fatalf("insert record_order record: %v", err)
		}
	}
	tables, err := service.Tables(ctx)
	if err != nil {
		t.Fatalf("Tables(): %v", err)
	}
	foundRowID, foundMigration := false, false
	for _, table := range tables {
		if table.Name == "keyed_records" {
			foundRowID = table.HighRisk
		}
		if table.Name == "schema_migrations" || table.Name == "goose_db_version" {
			foundMigration = true
		}
	}
	if !foundRowID || foundMigration {
		t.Fatalf("rowid table found=%v, excluded table leaked=%v", foundRowID, foundMigration)
	}
	page, err := service.Records(ctx, "keyed_records", "", 2)
	if err != nil {
		t.Fatalf("Records(first): %v", err)
	}
	if len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatalf("first page = %+v", page)
	}
	if amount, ok := page.Items[0].Values["amount"].Text(); !ok || amount != "9007199254740993" {
		t.Fatalf("integer value = %q, string=%v", amount, ok)
	}
	if enabled, ok := page.Items[0].Values["enabled"].Bool(); !ok || !enabled {
		t.Fatalf("boolean value = %v, bool=%v", enabled, ok)
	}
	page2, err := service.Records(ctx, "keyed_records", *page.NextCursor, 2)
	if err != nil || len(page2.Items) != 1 || page2.NextCursor != nil {
		t.Fatalf("Records(second) = %+v, %v", page2, err)
	}
	if _, err := service.Records(ctx, `users"; DROP TABLE users;--`, "", 50); !errors.Is(err, ErrTableNotFound) {
		t.Fatalf("identifier injection error = %v", err)
	}

	// Ensure the generated actor remains usable; this also guards against the
	// schema introspection accidentally mutating application state.
	if _, err := db.ExecContext(ctx, `SELECT id FROM users WHERE id=$1`, actor); err != nil {
		t.Fatalf("actor query: %v", err)
	}
}

func TestDatabaseValueRequiresDecimalStringForNumericJSON(t *testing.T) {
	t.Parallel()
	var value Value
	if err := json.Unmarshal([]byte(`9007199254740993`), &value); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("raw JSON integer error = %v", err)
	}
	if err := json.Unmarshal([]byte(`"9007199254740993"`), &value); err != nil {
		t.Fatalf("decimal-string JSON value: %v", err)
	}
	if text, ok := value.Text(); !ok || text != "9007199254740993" {
		t.Fatalf("decimal-string value=%q, string=%v", text, ok)
	}
}

func TestNumericPrimaryKeyCursorPreservesExactLargeInteger(t *testing.T) {
	t.Parallel()
	service, db, _, _, actor := newTestService(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE numeric_key_records(id NUMERIC PRIMARY KEY NOT NULL,label TEXT NOT NULL)`); err != nil {
		t.Fatalf("create numeric-key table: %v", err)
	}
	for _, id := range []int64{9007199254740993, 9007199254740995} {
		if _, err := db.ExecContext(ctx, `INSERT INTO numeric_key_records(id,label) VALUES($1,$2)`, id, "record"); err != nil {
			t.Fatalf("insert numeric key %d: %v", id, err)
		}
	}
	first, err := service.Records(ctx, "numeric_key_records", "", 1)
	if err != nil || len(first.Items) != 1 || first.NextCursor == nil {
		t.Fatalf("first numeric-key page = %+v, %v", first, err)
	}
	if first.Items[0].Key["id"] != "9007199254740993" {
		t.Fatalf("first numeric key = %q", first.Items[0].Key["id"])
	}
	second, err := service.Records(ctx, "numeric_key_records", *first.NextCursor, 1)
	if err != nil || len(second.Items) != 1 {
		t.Fatalf("second numeric-key page = %+v, %v", second, err)
	}
	if second.Items[0].Key["id"] != "9007199254740995" {
		t.Fatalf("second numeric key = %q", second.Items[0].Key["id"])
	}

	request := MutationRequest{Action: "update", Table: "numeric_key_records", Key: first.Items[0].Key,
		ExpectedHash: first.Items[0].RecordHash, Values: map[string]Value{"label": StringValue("reviewed")},
		Reason: "Verify exact numeric key handling"}
	review, err := service.ReviewMutation(ctx, actor, request)
	if err != nil || review.Key["id"] != "9007199254740993" {
		t.Fatalf("ReviewMutation did not preserve the exact large numeric key: %+v, %v", review, err)
	}
}

func TestMutationReasonContractRequiresFourCharacters(t *testing.T) {
	t.Parallel()

	if _, err := normalizeReason("abc"); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("normalizeReason(three characters) error = %v, want ErrInvalidValue", err)
	}
	if reason, err := normalizeReason(" abcd "); err != nil || reason != "abcd" {
		t.Fatalf("normalizeReason(four characters) = (%q, %v), want (abcd, nil)", reason, err)
	}
}
