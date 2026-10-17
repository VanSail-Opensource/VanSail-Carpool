-- Operational enforcement state; user session IP controls are independent.
CREATE TABLE xnode_firewall_clock(singleton INTEGER PRIMARY KEY CHECK(singleton=1),version INTEGER NOT NULL,source_version INTEGER NOT NULL,inventory_version INTEGER NOT NULL,active_bans INTEGER NOT NULL DEFAULT 0);
INSERT INTO xnode_firewall_clock(singleton,version,source_version,inventory_version) VALUES(1,1,1,1);
CREATE TABLE xnode_firewall_members(
 ordinal INTEGER PRIMARY KEY AUTOINCREMENT, node_id TEXT NOT NULL UNIQUE,
 active INTEGER NOT NULL DEFAULT 1, applied INTEGER NOT NULL DEFAULT 0, source_applied INTEGER NOT NULL DEFAULT 0,
 force_sync INTEGER NOT NULL DEFAULT 1,connection_epoch INTEGER NOT NULL DEFAULT 0
);
CREATE TRIGGER retire_xnode_firewall_member AFTER DELETE ON xnode_nodes BEGIN UPDATE xnode_firewall_members SET active=0 WHERE node_id=OLD.id; END;
CREATE TABLE xnode_firewall_cursors(node_id TEXT NOT NULL REFERENCES xnode_nodes(id) ON DELETE CASCADE,namespace TEXT NOT NULL,sequence INTEGER NOT NULL,PRIMARY KEY(node_id,namespace));
CREATE TABLE xnode_firewall_bans(
 generation TEXT PRIMARY KEY,prefix TEXT NOT NULL,ip TEXT NOT NULL,rule TEXT NOT NULL,detected_count TEXT NOT NULL,detected_at TEXT NOT NULL,
 until_at TEXT NOT NULL,refresh_seconds INTEGER NOT NULL,issued_version INTEGER NOT NULL,released_version INTEGER NOT NULL DEFAULT 0,
 released_at TEXT,participants BLOB NOT NULL,expired BLOB NOT NULL
);
CREATE UNIQUE INDEX xnode_firewall_active_prefix ON xnode_firewall_bans(prefix) WHERE released_version=0;
CREATE INDEX xnode_firewall_ban_versions ON xnode_firewall_bans(issued_version,released_version);
CREATE TRIGGER firewall_inventory_insert AFTER INSERT ON xnode_firewall_bans WHEN NEW.released_version=0 BEGIN UPDATE xnode_firewall_clock SET inventory_version=version,active_bans=active_bans+1 WHERE singleton=1; END;
CREATE TRIGGER firewall_inventory_release AFTER UPDATE OF released_version ON xnode_firewall_bans WHEN OLD.released_version=0 AND NEW.released_version>0 BEGIN UPDATE xnode_firewall_clock SET inventory_version=version,active_bans=active_bans-1 WHERE singleton=1; END;
CREATE TABLE xnode_firewall_sources(
 id TEXT NOT NULL,generation TEXT NOT NULL,until_at TEXT NOT NULL,refresh_seconds INTEGER NOT NULL,
 issued_version INTEGER NOT NULL,released_version INTEGER NOT NULL DEFAULT 0,released_at TEXT,participants BLOB NOT NULL,expired BLOB NOT NULL,PRIMARY KEY(id,generation)
);
CREATE UNIQUE INDEX xnode_firewall_active_source ON xnode_firewall_sources(id) WHERE released_version=0;
CREATE TABLE xnode_firewall_exemptions(
 id TEXT PRIMARY KEY,user_id TEXT REFERENCES users(id),idempotency_key TEXT NOT NULL,ip TEXT NOT NULL,
 price_minor INTEGER NOT NULL,hours INTEGER NOT NULL,until_at TEXT NOT NULL,created_at TEXT NOT NULL,
 issued_version INTEGER NOT NULL,released_version INTEGER NOT NULL DEFAULT 0,ledger_id TEXT REFERENCES ledger_entries(id),
 UNIQUE(user_id,idempotency_key)
);
CREATE INDEX xnode_firewall_exemption_versions ON xnode_firewall_exemptions(issued_version,released_version);
