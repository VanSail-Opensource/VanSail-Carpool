-- Allow one DDNS family to update multiple endpoints while keeping each host
-- assigned to only one node/job/family binding.
DROP TRIGGER compile_ddns_hosts_insert;
DROP TRIGGER compile_ddns_hosts_update;
DROP TRIGGER compile_ddns_hosts_delete;
ALTER TABLE xnode_ddns_hosts RENAME TO xnode_ddns_hosts_single;
CREATE TABLE xnode_ddns_hosts (
 node_id TEXT NOT NULL,job_id TEXT NOT NULL,
 family TEXT NOT NULL CHECK(family IN ('ipv4','ipv6')),
 host_id TEXT NOT NULL UNIQUE,
 PRIMARY KEY(node_id,job_id,family,host_id),
 FOREIGN KEY(node_id,job_id) REFERENCES xnode_ddns_jobs(node_id,job_id) ON DELETE CASCADE,
 FOREIGN KEY(host_id,node_id) REFERENCES xnode_endpoints(id,node_id)
);
INSERT INTO xnode_ddns_hosts(node_id,job_id,family,host_id)
 SELECT node_id,job_id,family,host_id FROM xnode_ddns_hosts_single;
DROP TABLE xnode_ddns_hosts_single;
CREATE TRIGGER compile_ddns_hosts_insert AFTER INSERT ON xnode_ddns_hosts BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_ddns_hosts_update AFTER UPDATE ON xnode_ddns_hosts BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_ddns_hosts_delete AFTER DELETE ON xnode_ddns_hosts BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
