package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	neturl "net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func multiplierFixture(t *testing.T) (context.Context, *sql.DB) {
	t.Helper()
	if os.Getenv("POSTGRES_TEST_ADMIN_URL") == "" {
		t.Skip("hosted isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	admin, err := pgx.Connect(ctx, os.Getenv("POSTGRES_TEST_ADMIN_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close(context.Background()) })
	name := fmt.Sprintf("vansail_multiplier_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, `CREATE DATABASE `+quote(name)+` OWNER vansail_migration`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Exec(context.Background(), `DROP DATABASE `+quote(name)+` WITH (FORCE)`) })
	u, err := neturl.Parse(os.Getenv("POSTGRES_TEST_URL"))
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	parsed, err := pgx.ParseConfig(u.String())
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(connector{Connector: stdlib.GetConnector(*parsed), metrics: &transactionMetrics{}})
	db.SetMaxOpenConns(6)
	t.Cleanup(func() { db.Close() })
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, files, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(ctx, 17); err != nil {
		t.Fatal(err)
	}
	seedRetirementUpgrade(t, ctx, db)
	if _, err = provider.UpTo(ctx, 45); err != nil {
		t.Fatal(err)
	}
	const at = "2026-10-20T00:00:00.000000000Z"
	queries := []string{
		`INSERT INTO xnode_configs(id,name,document_cipher,created_at,updated_at) VALUES('multiplier-config','Historical','cipher',$1,$1)`,
		`INSERT INTO xnode_nodes(id,name,config_id,settings_cipher,multiplier_fp,agent_id,created_at,updated_at) VALUES('multiplier-node','Historical','multiplier-config','cipher',1250000,'multiplier-agent',$1,$1),('one-node','One','multiplier-config','cipher',1000000,NULL,$1,$1),('max-node','Max','multiplier-config','cipher',0,'max-agent',$1,$1),('new-node','New','multiplier-config','cipher',2000000,'new-agent',$1,$1)`,
		`INSERT INTO xnode_report_cursors(agent_id,node_id,last_sequence,last_digest,updated_at) VALUES('multiplier-agent','multiplier-node',4,'committed',$1),('max-agent','max-node',9223372036854775807,'exhausted',$1)`,
		`INSERT INTO xnode_node_rates VALUES('multiplier-agent',1,1250000),('multiplier-agent',3,0),('multiplier-agent',5,2000000),('multiplier-agent',9,3000000),('max-agent',1,0),('orphan-history',1,1750000),('new-agent',1,2000000),('new-agent',8,3000000)`,
		`INSERT INTO xnode_traffic_streams(agent_id,stream_id,node_id,last_sequence,last_digest,closed,created_at,updated_at) VALUES('multiplier-agent','old-stream','multiplier-node',6,'stream-committed',false,$1,$1),('orphan-agent','max-stream','missing-node',9223372036854775807,'exhausted',true,$1,$1)`,
		`INSERT INTO xnode_stream_rates VALUES('multiplier-agent','old-stream',1,1250000),('multiplier-agent','old-stream',4,0),('multiplier-agent','old-stream',7,2000000),('multiplier-agent','old-stream',10,3000000),('orphan-agent','max-stream',1,2500000)`,
		`INSERT INTO xnode_identities(proxy_user_id,user_id,credentials_cipher,created_at) VALUES(7654,'retirement-owner','cipher',$1)`,
		`INSERT INTO xnode_periods(id,proxy_user_id,purchase_id,started_at,limit_bytes,strategy,used_bytes) VALUES('historical-period',7654,'retirement-settled',$1,1000,'no_reset',7)`,
		`INSERT INTO xnode_daily_usage(proxy_user_id,node_id,period_id,date,upload_bytes,download_bytes,weighted_bytes,weight_remainder) VALUES(7654,'multiplier-node','historical-period','2026-10-20',3,4,7,750000)`,
		`INSERT INTO xnode_imported_history(proxy_user_id,node_id,date,raw_bytes,weighted_bytes) VALUES(7654,'multiplier-node','2026-10-19',123,456)`,
	}
	for i, query := range queries {
		var args []any
		if i != 3 && i != 5 && i != 8 && i != 9 {
			args = []any{at}
		}
		if _, err = db.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("multiplier fixture %d: %v", i, err)
		}
	}
	return ctx, db
}

func historicalMultiplierProjection(t *testing.T, ctx context.Context, db *sql.DB) []TableContract {
	t.Helper()
	c, err := SchemaContract()
	if err != nil {
		t.Fatal(err)
	}
	// The fixture seeds at schema 45 before the retirement migration runs. Keep
	// only columns that already exist so a later additive migration (for example
	// hourly_offers.required_hours) is not queried before it is applied.
	c.Tables, err = existingUpgradeProjection(ctx, db, c.Tables)
	if err != nil {
		t.Fatal(err)
	}
	var tables []TableContract
	for _, table := range c.Tables {
		// Only policy/derived invalidation can change. Full native/financial facts,
		// credentials, node identity/revisions and timestamps remain fingerprinted.
		if table.Name == "xnode_node_rates" || table.Name == "xnode_stream_rates" || table.Name == "xnode_report_cursors" || table.Name == "xnode_compile_revision" {
			continue
		}
		var columns []ColumnContract
		for _, column := range table.Columns {
			if table.Name != "xnode_nodes" || column.Name != "multiplier_fp" {
				columns = append(columns, column)
			}
		}
		table.Columns = columns
		tables = append(tables, table)
	}
	return tables
}
