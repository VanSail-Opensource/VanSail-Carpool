-- +goose Up
INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
SELECT 'retired-rollover:'||purchase_id,NULL,'commerce.retired_settlement','purchase',purchase_id,
 json_build_object('status',status,'creditedMinor',credited_txb_minor,'netPaidMinor',net_paid_txb_minor,
 'allocatedBytes',allocated_traffic_bytes,'usedBytes',used_traffic_bytes,
 'eligibleBytes',eligible_unused_bytes,'algorithm',algorithm_version,
 'completedAt',completed_at,'exception',exception_code)::text,updated_at
FROM purchase_rollovers WHERE status IN ('credited','zero','exception') ON CONFLICT(id) DO NOTHING;
INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
SELECT 'rollover-retirement',NULL,'commerce.rollover_retired','system','rollover',
 json_build_object('cancelledCalculations',COUNT(*),'walletCreditMinor',0)::text,
 to_char(clock_timestamp() AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS.US"Z"')
FROM purchase_rollovers WHERE status IN ('pending','processing','calculated') ON CONFLICT(id) DO NOTHING;
INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
SELECT 'retired-rollover-day:'||user_id||':'||local_date,NULL,'commerce.retired_daily_settlement','user',user_id,
 json_build_object('date',local_date,'settlements',settlement_count,'creditedMinor',credited_txb_minor,
 'allocatedBytes',allocated_traffic_bytes,'usedBytes',used_traffic_bytes)::text,updated_at
FROM rollover_member_daily_rollups ON CONFLICT(id) DO NOTHING;
UPDATE activity_lucky_prizes SET reward_payload=(reward_payload::jsonb-'rolloverMinRemainingBps')::text;
DELETE FROM outbox_jobs WHERE kind IN ('rollover_finalize','pricing_renewal_notice');
DELETE FROM settings WHERE key='renewal.rollover_before_auto_renewal_enabled';
DROP TABLE purchase_rollovers;
DROP TABLE rollover_member_daily_rollups;
ALTER TABLE combos DROP COLUMN rollover_min_remaining_bps;
ALTER TABLE purchases DROP COLUMN reward_rollover_min_remaining_bps;

INSERT INTO schema_migrations(version,applied_at) VALUES('106_retire_rollover.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;

-- +goose Down
-- Retirement preserves immutable ledger/audit facts and is forward-only.
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
