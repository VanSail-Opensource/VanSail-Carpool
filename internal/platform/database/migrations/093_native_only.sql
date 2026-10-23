-- Retire the completed source bridge without changing immutable usage or money.
-- Empty installations may proceed; an unfinished installation must finish using
-- its previous release before upgrading to the native-only application.
CREATE TEMP TABLE native_retirement_guard (blocked INTEGER);
CREATE TEMP TRIGGER require_completed_native_accounting
BEFORE INSERT ON native_retirement_guard WHEN NEW.blocked<>0
BEGIN
 SELECT RAISE(ABORT,'Complete and finalize source accounting with the previous release before upgrading to native-only');
END;
INSERT INTO native_retirement_guard SELECT
 EXISTS(SELECT 1 FROM xnode_migrations WHERE phase<>'complete' OR finalized_at IS NULL)
 OR EXISTS(SELECT 1 FROM xnode_migration_nodes WHERE phase<>'native')
 OR (EXISTS(SELECT 1 FROM settings WHERE key IN ('remnawave.base_url','remnawave.api_token') AND value<>'')
     AND NOT EXISTS(SELECT 1 FROM xnode_migrations WHERE phase='complete' AND finalized_at IS NOT NULL));
DROP TRIGGER require_completed_native_accounting;
DROP TABLE native_retirement_guard;

-- These queued intents still provision paid native access.
UPDATE outbox_jobs SET kind=CASE kind
 WHEN 'remna_apply_entitlement' THEN 'xnode_apply_entitlement'
 WHEN 'remna_sync_user' THEN 'xnode_sync_user'
 WHEN 'remna_prepare_continuity' THEN 'xnode_prepare_continuity' END
WHERE kind IN ('remna_apply_entitlement','remna_sync_user','remna_prepare_continuity');

DELETE FROM outbox_jobs WHERE kind='provider_operation'
 AND json_extract(payload,'$.operationId') IN (SELECT id FROM provider_operations WHERE kind='xnode_migration');
DELETE FROM outbox_jobs WHERE kind LIKE 'remna_%';
DELETE FROM provider_operation_items WHERE operation_id IN (SELECT id FROM provider_operations WHERE kind='xnode_migration');
DELETE FROM provider_operations WHERE kind='xnode_migration';
DELETE FROM settings WHERE key LIKE 'remnawave.%';

-- Remove migration-only scopes while preserving unrelated grants.
UPDATE admin_api_tokens SET scopes_json=(SELECT json_group_array(value)
 FROM json_each(admin_api_tokens.scopes_json) WHERE value NOT IN ('migration:read','migration:write'));
DELETE FROM admin_api_tokens WHERE json_array_length(scopes_json)=0;

ALTER TABLE users RENAME COLUMN remna_user_id TO proxy_user_id;
ALTER TABLE users DROP COLUMN remna_subscription_url;
ALTER TABLE squad_product_overrides RENAME COLUMN remna_squad_uuid TO group_id;
ALTER TABLE purchase_addons RENAME COLUMN remna_squad_uuid TO group_id;
ALTER TABLE host_connectivity_attempts RENAME COLUMN remnawave_user_id TO proxy_user_id;

-- Retained facts are frozen historical accounting, not a provider integration.
ALTER TABLE xnode_legacy_usage RENAME TO xnode_imported_usage;
ALTER TABLE xnode_legacy_history RENAME TO xnode_imported_history;

-- Stored connectivity configuration is a plain validated settings document.
UPDATE settings SET value=json_remove(json_set(value,'$.proxyUserId',json_extract(value,'$.remnawaveUserId')),'$.remnawaveUserId')
WHERE key='connectivity.config' AND encrypted=0 AND json_valid(value)
 AND json_type(value,'$.remnawaveUserId') IS NOT NULL;

DROP TABLE xnode_migration_nodes;
DROP TABLE xnode_migrations;
UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
