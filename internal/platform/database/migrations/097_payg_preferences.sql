ALTER TABLE user_preferences ADD COLUMN payg_enabled INTEGER NOT NULL DEFAULT 1 CHECK(payg_enabled IN (0,1));
CREATE TABLE user_disabled_payg_squads (
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 group_id TEXT NOT NULL REFERENCES xnode_groups(id) ON DELETE CASCADE,
 PRIMARY KEY(user_id,group_id)
);
CREATE TRIGGER compile_payg_preference_insert AFTER INSERT ON user_preferences WHEN NEW.payg_enabled=0 BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_payg_preference_update AFTER UPDATE OF payg_enabled ON user_preferences WHEN OLD.payg_enabled<>NEW.payg_enabled BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_payg_preference_delete AFTER DELETE ON user_preferences WHEN OLD.payg_enabled=0 BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_disabled_payg_insert AFTER INSERT ON user_disabled_payg_squads BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
CREATE TRIGGER compile_disabled_payg_delete AFTER DELETE ON user_disabled_payg_squads BEGIN UPDATE xnode_compile_revision SET revision=revision+1 WHERE singleton=1; END;
