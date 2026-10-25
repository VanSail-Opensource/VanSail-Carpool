-- Reuse the authoritative compile epoch; never invalidate it for live agent
-- revisions, heartbeats or accounting that does not change access.
CREATE TRIGGER compile_xnode_node_catalog_metadata AFTER UPDATE OF name,country_code,provider_name,multiplier_fp ON xnode_nodes
WHEN OLD.name IS NOT NEW.name OR OLD.country_code IS NOT NEW.country_code OR OLD.provider_name IS NOT NEW.provider_name OR OLD.multiplier_fp IS NOT NEW.multiplier_fp
BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
