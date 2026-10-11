ALTER TABLE ip_lookup_reports ADD COLUMN heartbeat_at TEXT;
ALTER TABLE ip_lookup_reports ADD COLUMN lease_owner TEXT;
ALTER TABLE ip_lookup_reports ADD COLUMN lease_expires_at TEXT;
ALTER TABLE ip_lookup_reports ADD COLUMN cache_key TEXT NOT NULL DEFAULT '';
CREATE INDEX ip_lookup_cache_key ON ip_lookup_reports(cache_key,status);

CREATE TABLE ip_lookup_checks_next (
 id TEXT NOT NULL UNIQUE,
 operation_id TEXT PRIMARY KEY REFERENCES provider_operations(id) ON DELETE CASCADE,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 report_id TEXT REFERENCES ip_lookup_reports(id) ON DELETE SET NULL,
 allowance_id TEXT REFERENCES ip_lookup_allowances(purchase_id) ON DELETE SET NULL,
 charge_minor INTEGER NOT NULL CHECK(charge_minor >= 0),
 cached INTEGER NOT NULL CHECK(cached IN (0,1)),
 refunded INTEGER NOT NULL DEFAULT 0 CHECK(refunded IN (0,1)),
 quota_consumed INTEGER NOT NULL DEFAULT 0 CHECK(quota_consumed BETWEEN 0 AND 1000000)
);
INSERT INTO ip_lookup_checks_next SELECT c.id,c.operation_id,c.user_id,c.report_id,c.allowance_id,c.charge_minor,c.cached,c.refunded,
 COALESCE(json_extract(o.result_json,'$.usedQuota'),c.allowance_id IS NOT NULL)
 FROM ip_lookup_checks c JOIN provider_operations o ON o.id=c.operation_id;
DROP TABLE ip_lookup_checks;
ALTER TABLE ip_lookup_checks_next RENAME TO ip_lookup_checks;
CREATE INDEX ip_lookup_check_report ON ip_lookup_checks(report_id);

-- Clear old completed cache payloads; in-flight operations keep their recovery data.
UPDATE provider_operations SET result_json=json_set(result_json,'$.requestedIP',
 COALESCE(json_extract(result_json,'$.requestedIP'),(SELECT r.ip FROM ip_lookup_checks c JOIN ip_lookup_reports r ON r.id=c.report_id WHERE c.operation_id=provider_operations.id),''))
 WHERE id IN (SELECT operation_id FROM ip_lookup_checks);
DELETE FROM ip_lookup_reports WHERE status<>'processing';
DELETE FROM ip_lookup_live_snapshots;
DELETE FROM outbox_jobs WHERE kind IN ('ip_lookup_report_retry','ip_lookup_live_retry');
