-- Required encrypted desired state and deployment barriers; no live graph data.
CREATE TABLE xnode_routing_plans(
 proxy_user_id INTEGER PRIMARY KEY REFERENCES xnode_identities(proxy_user_id) ON DELETE CASCADE,
 cipher TEXT NOT NULL, previous_cipher TEXT NOT NULL DEFAULT '', legacy_cipher TEXT NOT NULL DEFAULT '',
 revision INTEGER NOT NULL CHECK(revision>0), phase TEXT NOT NULL CHECK(phase IN('revoking','applying','enabling','active','disabled')),
 operation_id TEXT NOT NULL REFERENCES provider_operations(id), reason TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL
);
CREATE TRIGGER compile_xnode_routing_insert AFTER INSERT ON xnode_routing_plans BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_xnode_routing_update AFTER UPDATE ON xnode_routing_plans BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_routing_templates_update AFTER UPDATE ON settings WHEN NEW.key='xnode.routing_templates' BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_routing_templates_insert AFTER INSERT ON settings WHEN NEW.key='xnode.routing_templates' BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
