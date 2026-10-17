-- Native identity ownership extends existing routing without changing ciphertext scopes.
CREATE TABLE xnode_identity_customizations (
 proxy_user_id INTEGER NOT NULL REFERENCES xnode_identities(proxy_user_id),
 user_id TEXT REFERENCES users(id) ON DELETE CASCADE,
 node_id TEXT NOT NULL REFERENCES xnode_nodes(id), outbounds_cipher TEXT NOT NULL, routing_cipher TEXT NOT NULL,
 revision INTEGER NOT NULL DEFAULT 1, updated_at TEXT NOT NULL, PRIMARY KEY(proxy_user_id,node_id)
);
INSERT INTO xnode_identity_customizations
 SELECT (SELECT proxy_user_id FROM xnode_identities WHERE user_id=c.user_id),user_id,node_id,outbounds_cipher,routing_cipher,revision,updated_at FROM xnode_customizations c;
DROP TABLE xnode_customizations;
ALTER TABLE xnode_identity_customizations RENAME TO xnode_customizations;
CREATE TRIGGER compile_xnode_customizations_insert AFTER INSERT ON xnode_customizations BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_xnode_customizations_update AFTER UPDATE ON xnode_customizations BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_xnode_customizations_delete AFTER DELETE ON xnode_customizations BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
