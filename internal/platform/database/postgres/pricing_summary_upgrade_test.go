package postgres

import (
	"io/fs"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

func TestPricingSummaryUpgradePreservesCompactedEvidence(t *testing.T) {
	ctx, db := multiplierFixture(t)
	files, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, files, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(ctx, 49); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().AddDate(0, 0, -91).Format(time.DateOnly)
	recent := time.Now().UTC().AddDate(0, 0, -2).Format(time.DateOnly)
	for _, day := range []string{old, recent} {
		for _, query := range []string{
			`INSERT INTO pricing_calibration_daily(day,model_version,kind,bucket,samples,probability_sum,observed_sum,brier_sum,updated_at)
			 VALUES($1,'upgrade-model','purchase',5,4,2,1,0.5,$1||'T01:00:00.000000000Z')`,
			`INSERT INTO pricing_model_validation_daily(day,model_version,metric,horizon_seconds,samples,metrics_json,updated_at_ns)
			 VALUES($1,'upgrade-model','traffic_bytes',86400,4,'{"medianAbsoluteError":7}',1)`,
			`INSERT INTO pricing_offers(id,user_id,product_type,product_id,price_minor,reference_minor,exposed_at,expires_at,
			 outcome,observed_at,model_version,predicted_probability,features_json)
			 VALUES($1,'upgrade-owner','combo','upgrade-product',100,100,$1||'T00:00:00.000000000Z',
			 $1||'T01:00:00.000000000Z',1,$1||'T01:00:00.000000000Z','upgrade-model',0.5,'{}')`,
		} {
			if _, err := db.ExecContext(ctx, query, day); err != nil {
				t.Fatal(err)
			}
		}
	}
	contract, err := SchemaContract()
	if err != nil {
		t.Fatal(err)
	}
	contract.Tables, err = existingUpgradeProjection(ctx, db, contract.Tables)
	if err != nil {
		t.Fatal(err)
	}
	var authoritative []TableContract
	for _, table := range contract.Tables {
		if table.Name != "pricing_calibration_daily" && table.Name != "pricing_model_validation_daily" {
			authoritative = append(authoritative, table)
		}
	}
	before, err := inspect(ctx, db, authoritative)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(ctx, 50); err != nil {
		t.Fatal(err)
	}
	after, err := inspect(ctx, db, authoritative)
	if err != nil || before.Hash != after.Hash {
		t.Fatal("summary repair changed source or financial evidence", err)
	}
	if err := VerifySchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"pricing_calibration_daily", "pricing_model_validation_daily"} {
		var samples int
		var sealed, stale, dirty bool
		if err := db.QueryRowContext(ctx, `SELECT samples,sealed,stale,dirty FROM `+table+` WHERE day=$1`, old).
			Scan(&samples, &sealed, &stale, &dirty); err != nil || samples != 4 || !sealed || !stale || dirty {
			t.Fatal("potentially compacted metrics were lost or presented as current", table, samples, sealed, stale, err)
		}
		if err := db.QueryRowContext(ctx, `SELECT samples,sealed,stale,dirty FROM `+table+` WHERE day=$1`, recent).
			Scan(&samples, &sealed, &stale, &dirty); err != nil || samples != 4 || sealed || stale || !dirty {
			t.Fatal("provably retained cohort did not become rebuildable", table, samples, sealed, stale, dirty, err)
		}
		for _, change := range []string{"samples=samples+1", "sealed=false", "stale=false", "dirty=true"} {
			if _, err := db.ExecContext(ctx, `UPDATE `+table+` SET `+change+` WHERE day=$1`, old); err == nil {
				t.Fatal("sealed history accepted destructive rewrite", table, change)
			}
		}
	}
	var probability, brier float64
	var observed int
	if err := db.QueryRowContext(ctx, `SELECT probability_sum,observed_sum,brier_sum FROM pricing_calibration_daily WHERE day=$1`, old).
		Scan(&probability, &observed, &brier); err != nil || probability != 2 || observed != 1 || brier != 0.5 {
		t.Fatal("old probability evidence changed", probability, observed, brier, err)
	}
	var metrics string
	if err := db.QueryRowContext(ctx, `SELECT metrics_json FROM pricing_model_validation_daily WHERE day=$1`, old).
		Scan(&metrics); err != nil || metrics != `{"medianAbsoluteError":7}` {
		t.Fatal("old nonlinear validation evidence changed", metrics, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO pricing_calibration_daily(day,model_version,kind,bucket,samples,probability_sum,observed_sum,brier_sum,updated_at)
	 VALUES($1,'rebuilt','purchase',5,1,0.5,1,0.25,$1||'T01:00:00.000000000Z')`, recent); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	var retained int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pricing_calibration_daily WHERE model_version='rebuilt'`).Scan(&retained); err != nil || retained != 1 {
		t.Fatal("restart repeated the one-time derived rebuild", retained, err)
	}
}
