package database

import (
	"context"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
	"path/filepath"
	"strings"
	"testing"
)

func TestZeroMultiplierUpgradePreservesReferencesSchemaAndBackup(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upgrade.db")
	db, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	store := NewStore(db)
	if err = store.SaveXNodeConfiguration(ctx, xnode.Configuration{ID: "config", Name: "Upgrade", DocumentCipher: "encrypted"}); err != nil {
		t.Fatal(err)
	}
	node := xnode.Node{ID: "node", Name: "Upgrade", ConfigID: "config", SettingsCipher: "encrypted", MultiplierFP: 1250000, Enabled: true}
	if err = store.SaveXNodeNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	if err = store.BindXNodeAgent(ctx, node.ID, "agent"); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveXNodeGroup(ctx, xnode.Group{ID: "group", Name: "Upgrade", Enabled: true, Bindings: []xnode.GroupBinding{{NodeID: node.ID, InboundTag: "paid"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, `CREATE VIEW retained_node_names AS SELECT id,name FROM xnode_nodes;
CREATE INDEX retained_node_country ON xnode_nodes(country_code);`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version=?", zeroMultiplierMigration); err != nil {
		t.Fatal(err)
	}
	script, err := migrations.ReadFile("migrations/" + zeroMultiplierMigration)
	if err != nil {
		t.Fatal(err)
	}
	legacy := strings.ReplaceAll(string(script), "CHECK(multiplier_fp>=0)", "CHECK(multiplier_fp>0)")
	if err = migrateReferencedTables(ctx, db, []byte(legacy), zeroMultiplierMigration); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "DELETE FROM schema_migrations WHERE version=?", zeroMultiplierMigration); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store = NewStore(db)
	var enforcement, references, indexes int
	if err = db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enforcement); err != nil || enforcement != 1 {
		t.Fatal("foreign keys not restored", err)
	}
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM xnode_group_bindings WHERE node_id='node'").Scan(&references); err != nil || references != 1 {
		t.Fatal("node references were lost", err)
	}
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE name='retained_node_country'").Scan(&indexes); err != nil || indexes != 1 {
		t.Fatal("custom index was lost", err)
	}
	var name string
	if err = db.QueryRowContext(ctx, "SELECT name FROM retained_node_names WHERE id='node'").Scan(&name); err != nil || name != "Upgrade" {
		t.Fatal("dependent view was lost", err)
	}
	var before, after int64
	if err = db.QueryRowContext(ctx, "SELECT revision FROM xnode_compile_revision WHERE singleton=1").Scan(&before); err != nil {
		t.Fatal(err)
	}
	node.MultiplierFP = 0
	// Pricing changes have their own accounting rate boundary. Exercise a field
	// that intentionally invalidates compiled configuration to check its trigger.
	node.SettingsCipher = "updated encrypted settings"
	if err = store.SaveXNodeNode(ctx, node); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, "SELECT revision FROM xnode_compile_revision WHERE singleton=1").Scan(&after); err != nil || after <= before {
		t.Fatal("compile trigger was lost", err)
	}
	if err = store.SetXNodeRate(ctx, node.ID, "agent", 0, 0); err != nil {
		t.Fatal(err)
	}
	backups, err := filepath.Glob(path + ".pre-xnode-zero-*.db")
	if err != nil || len(backups) != 1 {
		t.Fatal("verified upgrade backup missing", err)
	}
}
