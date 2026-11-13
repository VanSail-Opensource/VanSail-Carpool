package database

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type pricingExportConflictWriter struct {
	writes int
	err    error
}

func (w *pricingExportConflictWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, w.err
}

func TestPricingExportDoesNotRetryPublishedOutput(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	conflict := &pgconn.PgError{Code: "40001", Message: "injected stream failure"}
	writer := &pricingExportConflictWriter{err: conflict}
	err := store.StreamPricingExport(context.Background(), writer, PricingExportScope{Format: "jsonl"})
	if !errors.Is(err, conflict) || writer.writes != 1 {
		t.Fatalf("stream failure = %v after %d writes; expected one attempt", err, writer.writes)
	}
}

func TestPricingExportPseudonymizationPreservesExactJSONIntegers(t *testing.T) {
	t.Parallel()
	raw := `{"userId":"member-1","traffic":9007199254740993,"nested":[{"actor_id":"member-2","value":9223372036854775807}]}`
	redacted, err := redactPricingJSON(raw, []byte("export-local-key"))
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		User    string      `json:"userId"`
		Traffic json.Number `json:"traffic"`
		Nested  []struct {
			Actor string      `json:"actor_id"`
			Value json.Number `json:"value"`
		} `json:"nested"`
	}
	if err := json.Unmarshal([]byte(redacted), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result.User, "user_") || len(result.Nested) != 1 || !strings.HasPrefix(result.Nested[0].Actor, "user_") {
		t.Fatalf("identity pseudonymization = %s", redacted)
	}
	if result.Traffic.String() != "9007199254740993" || result.Nested[0].Value.String() != "9223372036854775807" {
		t.Fatalf("pseudonymization changed exact retained integers: %s", redacted)
	}
}

func TestPricingExportRejectsUnredactableRetainedJSON(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{`{"userId":"private-user"`, `{"userId":"private-user"} trailing`, `{"userId":123}`} {
		redacted, err := redactPricingJSON(raw, []byte("key"))
		if !errors.Is(err, ErrPricingExportRedaction) || redacted != "" || strings.Contains(err.Error(), "private-user") {
			t.Fatalf("unsafe redaction returned source data: %q, %v", redacted, err)
		}
	}
	redacted, err := redactPricingJSON(`{"userIds":["private-user"],"nested":{"actor_ids":["private-actor"]}}`, []byte("key"))
	if err != nil || strings.Contains(redacted, "private-user") || strings.Contains(redacted, "private-actor") {
		t.Fatalf("plural identity redaction = %q, %v", redacted, err)
	}
	ctx, store := context.Background(), newTestStore(t)
	_, err = store.DB().ExecContext(ctx, `INSERT INTO pricing_model_versions
		(id,kind,created_at,calibration_status,parameters_json,feature_definition_json,watermark,diagnostics_json)
		VALUES('corrupt','test','2026-11-12T00:00:00Z','UNVALIDATED',$1,'{}','2026-11-12T00:00:00Z','{}')`, `{"userId":"private-user"`)
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	err = store.StreamPricingExport(ctx, &output, PricingExportScope{Format: "jsonl", Category: "forecasts", Pseudonymize: true})
	if !errors.Is(err, ErrPricingExportRedaction) || strings.Contains(err.Error(), "private-user") {
		t.Fatalf("corrupt export did not fail safely: %v", err)
	}
}
