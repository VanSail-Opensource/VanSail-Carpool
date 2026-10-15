-- Retire detector execution and secrets; retained incident/financial/audit history stays intact.
UPDATE outbox_jobs SET status='failed',last_error='FEATURE_RETIRED',updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
 WHERE kind IN ('abuse_punishment','abuse_restore','abuse_notification') AND status IN ('pending','processing');

-- Only acknowledged temporary bans have proven mutation ownership. One bounded
-- compatibility job releases each user's retired restriction without resetting quota.
INSERT INTO outbox_jobs(id,kind,aggregate_id,payload,status,available_at,created_at,updated_at)
SELECT 'detector-retirement:'||ban.user_id,'proxy_restriction_release',ban.user_id,
 json_object('userId',ban.user_id),'pending',strftime('%Y-%m-%dT%H:%M:%fZ','now'),
 strftime('%Y-%m-%dT%H:%M:%fZ','now'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
FROM abuse_temp_bans ban JOIN abuse_records record ON record.id=ban.record_id
WHERE ban.restored_at IS NULL AND record.punishment_completed_at IS NOT NULL
GROUP BY ban.user_id;

INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
SELECT 'detector-retirement-review:'||ban.record_id,NULL,'migration.detector_restriction_review','user',ban.user_id,
 json_object('recordId',ban.record_id,'reason','unconfirmed_legacy_mutation'),strftime('%Y-%m-%dT%H:%M:%fZ','now')
FROM abuse_temp_bans ban JOIN abuse_records record ON record.id=ban.record_id
WHERE ban.restored_at IS NULL AND record.punishment_completed_at IS NULL;

DROP TABLE abuse_node_credentials;
DROP TABLE abuse_policy;
DROP TABLE abuse_domain_rules;
DROP TABLE abuse_whitelist;
DROP TABLE abuse_punishment_rules;
DROP TABLE abuse_log_fingerprints;
DROP TABLE abuse_qps_samples;
DROP TABLE abuse_detector_state;
DROP TABLE abuse_pending_log_events;
DROP TABLE abuse_qps_rollups;
DROP TABLE abuse_ip_ban_scans;
