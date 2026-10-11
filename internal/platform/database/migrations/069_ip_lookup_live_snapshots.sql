CREATE TABLE ip_lookup_live_snapshots (
    id TEXT PRIMARY KEY,
    ip TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('succeeded','partial')),
    details_json TEXT NOT NULL,
    completed_at TEXT NOT NULL,
    run_key TEXT UNIQUE
);
CREATE INDEX ip_lookup_live_cache ON ip_lookup_live_snapshots(ip,completed_at DESC);
ALTER TABLE ip_lookup_reports ADD COLUMN live_state_json TEXT NOT NULL DEFAULT '{}';
