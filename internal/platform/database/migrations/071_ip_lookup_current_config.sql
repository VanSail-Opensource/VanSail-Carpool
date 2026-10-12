-- Execution settings are read from settings; checkpoints retain completed evidence.
ALTER TABLE ip_lookup_reports DROP COLUMN config_json;
