-- +goose Up
CREATE UNIQUE INDEX xnode_endpoint_ownership_idx ON xnode_endpoints(id,node_id);
CREATE TABLE xnode_ddns_jobs (
 node_id TEXT COLLATE "C" NOT NULL REFERENCES xnode_nodes(id) ON DELETE CASCADE,
 job_id TEXT COLLATE "C" NOT NULL CHECK(length(job_id) BETWEEN 1 AND 128),
 configured_by TEXT COLLATE "C" NOT NULL REFERENCES users(id),
 PRIMARY KEY(node_id,job_id)
);
CREATE TABLE xnode_ddns_hosts (
 node_id TEXT COLLATE "C" NOT NULL,job_id TEXT COLLATE "C" NOT NULL,
 family TEXT COLLATE "C" NOT NULL CHECK(family IN ('ipv4','ipv6')),
 host_id TEXT COLLATE "C" NOT NULL UNIQUE,
 PRIMARY KEY(node_id,job_id,family),
 FOREIGN KEY(node_id,job_id) REFERENCES xnode_ddns_jobs(node_id,job_id) ON DELETE CASCADE,
 FOREIGN KEY(host_id,node_id) REFERENCES xnode_endpoints(id,node_id)
);
-- +goose StatementBegin
CREATE FUNCTION compile_ddns_references_fn() RETURNS trigger LANGUAGE plpgsql AS $vansail$
BEGIN
 UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1;
 IF TG_OP='DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END;
$vansail$;
-- +goose StatementEnd
CREATE TRIGGER compile_ddns_jobs_insert AFTER INSERT ON xnode_ddns_jobs FOR EACH ROW EXECUTE FUNCTION compile_ddns_references_fn();
CREATE TRIGGER compile_ddns_jobs_update AFTER UPDATE ON xnode_ddns_jobs FOR EACH ROW EXECUTE FUNCTION compile_ddns_references_fn();
CREATE TRIGGER compile_ddns_jobs_delete AFTER DELETE ON xnode_ddns_jobs FOR EACH ROW EXECUTE FUNCTION compile_ddns_references_fn();
CREATE TRIGGER compile_ddns_hosts_insert AFTER INSERT ON xnode_ddns_hosts FOR EACH ROW EXECUTE FUNCTION compile_ddns_references_fn();
CREATE TRIGGER compile_ddns_hosts_update AFTER UPDATE ON xnode_ddns_hosts FOR EACH ROW EXECUTE FUNCTION compile_ddns_references_fn();
CREATE TRIGGER compile_ddns_hosts_delete AFTER DELETE ON xnode_ddns_hosts FOR EACH ROW EXECUTE FUNCTION compile_ddns_references_fn();
INSERT INTO schema_migrations(version,applied_at) VALUES('100_xnode_ddns_bindings.sql',vansail_now()) ON CONFLICT(version) DO NOTHING;
-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Authoritative schema downgrade is not supported'; END $$;
-- +goose StatementEnd
