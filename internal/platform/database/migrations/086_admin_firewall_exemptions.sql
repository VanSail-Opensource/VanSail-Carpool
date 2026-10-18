-- Distinguish administrator grants from paid member grants without duplicating
-- desired addresses, delivery clocks or financial evidence.
ALTER TABLE xnode_firewall_exemptions ADD COLUMN grant_kind TEXT NOT NULL DEFAULT 'member'
 CHECK(grant_kind IN ('member','admin'));

CREATE TRIGGER firewall_exemption_inventory_insert AFTER INSERT ON xnode_firewall_exemptions
BEGIN UPDATE xnode_firewall_clock SET inventory_version=version WHERE singleton=1; END;
CREATE TRIGGER firewall_exemption_inventory_release AFTER UPDATE OF released_version ON xnode_firewall_exemptions
WHEN OLD.released_version=0 AND NEW.released_version>0
BEGIN UPDATE xnode_firewall_clock SET inventory_version=version WHERE singleton=1; END;
