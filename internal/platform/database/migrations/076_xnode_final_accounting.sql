-- Final exports are encrypted; live connection/session data is never captured.
ALTER TABLE xnode_migrations ADD COLUMN final_snapshot_cipher TEXT NOT NULL DEFAULT '';
ALTER TABLE xnode_migrations ADD COLUMN final_digest TEXT NOT NULL DEFAULT '';
ALTER TABLE xnode_migrations ADD COLUMN final_source_backup_reference TEXT NOT NULL DEFAULT '';
ALTER TABLE xnode_migrations ADD COLUMN finalized_at TEXT;
