-- Preserve settled financial evidence in the existing audit journal before
-- removing operational credit state. Unsettled calculations are cancelled.
INSERT OR IGNORE INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
SELECT 'retired-rollover:'||purchase_id,NULL,'commerce.retired_settlement','purchase',purchase_id,
 json_object('status',status,'creditedMinor',credited_txb_minor,'netPaidMinor',net_paid_txb_minor,
 'allocatedBytes',allocated_traffic_bytes,'usedBytes',used_traffic_bytes,
 'eligibleBytes',eligible_unused_bytes,'algorithm',algorithm_version,
 'completedAt',completed_at,'exception',exception_code),updated_at
FROM purchase_rollovers WHERE status IN ('credited','zero','exception');
INSERT OR IGNORE INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
SELECT 'rollover-retirement',NULL,'commerce.rollover_retired','system','rollover',
 json_object('cancelledCalculations',COUNT(*),'walletCreditMinor',0),strftime('%Y-%m-%dT%H:%M:%fZ','now')
FROM purchase_rollovers WHERE status IN ('pending','processing','calculated');
INSERT OR IGNORE INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
SELECT 'retired-rollover-day:'||user_id||':'||local_date,NULL,'commerce.retired_daily_settlement','user',user_id,
 json_object('date',local_date,'settlements',settlement_count,'creditedMinor',credited_txb_minor,
 'allocatedBytes',allocated_traffic_bytes,'usedBytes',used_traffic_bytes),updated_at
FROM rollover_member_daily_rollups;
UPDATE activity_lucky_prizes SET reward_payload=json_remove(reward_payload,'$.rolloverMinRemainingBps');
DELETE FROM outbox_jobs WHERE kind IN ('rollover_finalize','pricing_renewal_notice');
DELETE FROM settings WHERE key='renewal.rollover_before_auto_renewal_enabled';
DROP TABLE purchase_rollovers;
DROP TABLE rollover_member_daily_rollups;
ALTER TABLE combos DROP COLUMN rollover_min_remaining_bps;
ALTER TABLE purchases DROP COLUMN reward_rollover_min_remaining_bps;
