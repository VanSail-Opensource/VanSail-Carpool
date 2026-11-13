package databaseadmin

import (
	"context"
	"errors"
	"testing"
)

func TestQueryRecordsUsesNativeAndLegacyBooleanTypes(t *testing.T) {
	t.Parallel()
	service, db, _, _, _ := newTestService(t)
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE boolean_filter_records(
		id TEXT PRIMARY KEY, enabled BOOLEAN NOT NULL, active BIGINT NOT NULL);
		INSERT INTO boolean_filter_records VALUES ('disabled',false,0),('enabled',true,1)`); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"enabled", "active"} {
		for _, tc := range []struct {
			name, operator, want string
			value                Value
		}{
			{"true", "eq", "enabled", BooleanValue(true)},
			{"false", "eq", "disabled", BooleanValue(false)},
			{"not true", "ne", "disabled", BooleanValue(true)},
			{"text true", "eq", "enabled", StringValue("true")},
			{"text zero", "eq", "disabled", StringValue("0")},
		} {
			t.Run(column+"/"+tc.name, func(t *testing.T) {
				page, err := service.QueryRecords(ctx, "boolean_filter_records", QueryRequest{
					Filters: []QueryFilter{{Column: column, Operator: tc.operator, Value: &tc.value}},
				})
				if err != nil || len(page.Items) != 1 {
					t.Fatalf("boolean filter returned %d rows: %v", len(page.Items), err)
				}
				if page.Items[0].Key["id"] != tc.want {
					t.Fatalf("matched %q, want %q", page.Items[0].Key["id"], tc.want)
				}
			})
		}
	}
	value := BooleanValue(true)
	for _, operator := range []string{"contains", "starts_with"} {
		if _, err := service.QueryRecords(ctx, "boolean_filter_records", QueryRequest{
			Filters: []QueryFilter{{Column: "enabled", Operator: operator, Value: &value}},
		}); !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("text operator %q accepted boolean column: %v", operator, err)
		}
	}
}
