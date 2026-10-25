-- +goose Up
-- Keep global host uniqueness while permitting multiple endpoints per family.
ALTER TABLE xnode_ddns_hosts DROP CONSTRAINT xnode_ddns_hosts_pkey;
ALTER TABLE xnode_ddns_hosts ADD CONSTRAINT xnode_ddns_hosts_pkey PRIMARY KEY(node_id,job_id,family,host_id);
INSERT INTO schema_migrations(version,applied_at) VALUES('00016_xnode_ddns_multi_hosts.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
