-- Keep historical operation/audit facts, but never dispatch retired mutations.
UPDATE outbox_jobs SET status='failed',last_error='FEATURE_RETIRED',
 updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
 WHERE kind='provider_operation' AND status IN ('pending','processing')
 AND json_extract(payload,'$.operationId') IN
 (SELECT id FROM provider_operations WHERE kind IN ('admin_remna_relink','host_remark_update'));

UPDATE provider_operation_items SET status='failed',error_code='FEATURE_RETIRED',
 completed_at=strftime('%Y-%m-%dT%H:%M:%fZ','now'),updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
 WHERE status IN ('queued','processing','pending_review') AND operation_id IN
 (SELECT id FROM provider_operations WHERE kind IN ('admin_remna_relink','host_remark_update'));

UPDATE provider_operations SET status='failed',error_code='FEATURE_RETIRED',
 completed_at=strftime('%Y-%m-%dT%H:%M:%fZ','now'),updated_at=strftime('%Y-%m-%dT%H:%M:%fZ','now')
 WHERE kind IN ('admin_remna_relink','host_remark_update') AND status IN ('queued','processing','pending_review');

ALTER TABLE squad_product_overrides DROP COLUMN geocheck_disabled;
