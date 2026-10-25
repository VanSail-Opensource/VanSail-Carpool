-- Endpoint references affect routing compilation and DDNS bypass retention.
CREATE TRIGGER compile_xnode_endpoints_insert AFTER INSERT ON xnode_endpoints BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_xnode_endpoints_delete AFTER DELETE ON xnode_endpoints BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_xnode_endpoints_update AFTER UPDATE OF node_id,inbound_tag,name,address,port,enabled,hidden,overrides_cipher ON xnode_endpoints
WHEN OLD.node_id IS NOT NEW.node_id OR OLD.inbound_tag IS NOT NEW.inbound_tag OR OLD.name IS NOT NEW.name OR OLD.address IS NOT NEW.address OR OLD.port IS NOT NEW.port OR OLD.enabled IS NOT NEW.enabled OR OLD.hidden IS NOT NEW.hidden OR OLD.overrides_cipher IS NOT NEW.overrides_cipher
BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
