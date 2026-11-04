-- +goose Up
-- Financial grant/payment rows and terminal receipts remain immutable evidence.
INSERT INTO audit_events(id,actor_user_id,action,target_type,target_id,detail,created_at)
VALUES('member-firewall-retirement',NULL,'member_firewall.retired','system','member-firewall',
 '{"paidExemptions":"released","manualBlocks":"queued_for_removal","whitelist":"retired","jobs":"retired"}',vansail_now());

UPDATE xnode_firewall_clock SET version=version+1 WHERE singleton=1
 AND EXISTS(SELECT 1 FROM xnode_firewall_exemptions WHERE grant_kind='member' AND released_version=0);
UPDATE xnode_firewall_exemptions
SET released_version=(SELECT version FROM xnode_firewall_clock WHERE singleton=1)
WHERE grant_kind='member' AND released_version=0;
-- Existing reconciliation detects the new revision, including offline members.
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1
 AND EXISTS(SELECT 1 FROM xnode_member_diy);

UPDATE provider_operation_items SET status='failed',error_code='MEMBER_FEATURE_RETIRED',completed_at=vansail_now(),updated_at=vansail_now()
WHERE status IN('queued','processing','pending_review') AND operation_id IN(
 SELECT id FROM provider_operations WHERE kind IN('connection_block','connection_drop')
 OR (kind='xnode_diy' AND id IN(SELECT operation_id FROM provider_operation_items WHERE target_type='member_whitelist')));
UPDATE provider_operations SET status='failed',error_code='MEMBER_FEATURE_RETIRED',completed_at=vansail_now(),updated_at=vansail_now()
WHERE status IN('queued','processing','pending_review') AND (kind IN('connection_block','connection_drop')
 OR (kind='xnode_diy' AND id IN(SELECT operation_id FROM provider_operation_items WHERE target_type='member_whitelist')));

UPDATE connection_scans SET status='failed',error_code='MEMBER_FEATURE_RETIRED',updated_at=vansail_now()
WHERE status IN('queued','processing');
UPDATE outbox_jobs SET status='failed',last_error='MEMBER_FEATURE_RETIRED',updated_at=vansail_now()
WHERE kind='connection_scan_request' AND status IN('pending','processing');
-- Reuse the queued encrypted-target cleanup; retain rows until node confirmation.
UPDATE connection_ip_blocks SET status='unblocking',expires_at=vansail_now(),updated_at=vansail_now();
UPDATE outbox_jobs SET status='pending',attempts=0,available_at=vansail_now(),updated_at=vansail_now()
WHERE id IN(SELECT expiry_job_id FROM connection_ip_blocks);
ALTER TABLE combos DROP COLUMN whitelist_ip_quota;

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
