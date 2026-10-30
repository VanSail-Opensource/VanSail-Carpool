package postgres

import (
	"context"
	"database/sql"
	"testing"
)

func seedCommunityRetirement(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	queries := []string{
		`INSERT INTO users(id,telegram_id,onboarding_state,created_at,updated_at) VALUES('retired-community-owner',49001,'complete','2026-10-20T00:00:00Z','2026-10-20T00:00:00Z')`,
		`INSERT INTO balances(user_id,txb_minor,updated_at) VALUES('retired-community-owner',75,'2026-10-20T00:00:00Z')`,
		`INSERT INTO ledger_entries(id,user_id,delta_txb_minor,balance_after,kind,reference_id,created_at) VALUES('retired-community-ledger','retired-community-owner',75,75,'questionnaire_reward','retired-participant','2026-10-20T00:00:00Z')`,
		`INSERT INTO questionnaires(id,title,form_url,reward_txb_minor,status,created_at,updated_at) VALUES('retired-form','Historical reward','https://forms.example',75,'settled','2026-10-20T00:00:00Z','2026-10-20T00:00:00Z')`,
		`INSERT INTO questionnaire_participants(id,questionnaire_id,user_id,validation_code,awarded_at,created_at) VALUES('retired-participant','retired-form','retired-community-owner','historical-validation','2026-10-20T00:00:00Z','2026-10-20T00:00:00Z')`,
		`INSERT INTO questionnaire_imports(id,questionnaire_id,status,raw_csv,delimiter,headers_json,sample_rows_json,data_row_count,malformed_row_count,report_json,idempotency_key,created_at,updated_at) VALUES('retired-import','retired-form','settled',decode('','hex'),',','[]','[]',1,0,'{"rewardedCount":1}','retired-import','2026-10-20T00:00:00Z','2026-10-20T00:00:00Z')`,
		`INSERT INTO provider_operations(id,actor_user_id,owner_user_id,kind,idempotency_key,request_fingerprint,status,created_at,updated_at) VALUES('retired-operation','retired-community-owner','retired-community-owner','questionnaire_settlement','retired-operation','1234567890abcdef','queued','2026-10-20T00:00:00Z','2026-10-20T00:00:00Z')`,
		`INSERT INTO outbox_jobs(id,kind,payload,status,available_at,created_at,updated_at) VALUES('retired-job','questionnaire_settlement','{"importId":"retired-import"}','pending','2026-10-20T00:00:00Z','2026-10-20T00:00:00Z','2026-10-20T00:00:00Z')`,
	}
	for _, query := range queries {
		if _, err := db.ExecContext(ctx, query); err != nil {
			t.Fatal("seed retired feature", err)
		}
	}
}

func verifyCommunityRetirement(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	checks := []struct {
		query string
		want  int64
	}{
		{`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_name IN ('questionnaires','questionnaire_imports','questionnaire_participants')`, 0},
		{`SELECT txb_minor FROM balances WHERE user_id='retired-community-owner'`, 75},
		{`SELECT delta_txb_minor FROM ledger_entries WHERE id='retired-community-ledger'`, 75},
		{`SELECT COUNT(*) FROM audit_events WHERE id IN ('retired-questionnaire-reward:retired-participant','retired-questionnaire-import:retired-import')`, 2},
		{`SELECT COUNT(*) FROM provider_operations WHERE id='retired-operation' AND status='failed' AND error_code='FEATURE_RETIRED'`, 1},
		{`SELECT COUNT(*) FROM outbox_jobs WHERE id='retired-job'`, 0},
	}
	for _, check := range checks {
		var got int64
		if err := db.QueryRowContext(ctx, check.query).Scan(&got); err != nil || got != check.want {
			t.Fatal("retirement contract", check.query, got, check.want, err)
		}
	}
}
