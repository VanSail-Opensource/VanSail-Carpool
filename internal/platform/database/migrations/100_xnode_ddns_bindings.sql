-- Persist only panel-owned job/host/audit references, separately from encrypted
-- agent discovery settings. Composite ownership prevents cross-node bindings.
CREATE UNIQUE INDEX xnode_endpoint_ownership_idx ON xnode_endpoints(id,node_id);
CREATE TABLE xnode_ddns_jobs (
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id) ON DELETE CASCADE,
 job_id TEXT NOT NULL CHECK(length(job_id) BETWEEN 1 AND 128),
 configured_by TEXT NOT NULL REFERENCES users(id),
 PRIMARY KEY(node_id,job_id)
);
CREATE TABLE xnode_ddns_hosts (
 node_id TEXT NOT NULL,job_id TEXT NOT NULL,
 family TEXT NOT NULL CHECK(family IN ('ipv4','ipv6')),
 host_id TEXT NOT NULL UNIQUE,
 PRIMARY KEY(node_id,job_id,family),
 FOREIGN KEY(node_id,job_id) REFERENCES xnode_ddns_jobs(node_id,job_id) ON DELETE CASCADE,
 FOREIGN KEY(host_id,node_id) REFERENCES xnode_endpoints(id,node_id)
);
CREATE TRIGGER compile_ddns_jobs_insert AFTER INSERT ON xnode_ddns_jobs BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_ddns_jobs_update AFTER UPDATE ON xnode_ddns_jobs BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_ddns_jobs_delete AFTER DELETE ON xnode_ddns_jobs BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_ddns_hosts_insert AFTER INSERT ON xnode_ddns_hosts BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_ddns_hosts_update AFTER UPDATE ON xnode_ddns_hosts BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_ddns_hosts_delete AFTER DELETE ON xnode_ddns_hosts BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
