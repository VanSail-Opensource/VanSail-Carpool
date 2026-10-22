ALTER TABLE combos ADD COLUMN whitelist_ip_quota INTEGER NOT NULL DEFAULT 5
 CHECK(whitelist_ip_quota BETWEEN 0 AND 1000);
CREATE INDEX firewall_bans_detecting_ip_active ON xnode_firewall_bans(ip)
 WHERE released_version=0;
CREATE TRIGGER compile_combo_whitelist_update AFTER UPDATE OF whitelist_ip_quota ON combos
 BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
