package database

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"runtime/debug"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

type pricingExportLimit struct {
	w         io.Writer
	remaining int64
}

func (w *pricingExportLimit) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, pricing.ExportResourceLimit
	}
	n, err := w.w.Write(p)
	w.remaining -= int64(n)
	return n, err
}

// StreamPricingExport holds one bounded read-only snapshot, writes directly to
// the caller with backpressure and creates no permanent or temporary archive.
func (s *Store) retryAttempt_StreamPricingExport(ctx context.Context, writer io.Writer, scope PricingExportScope) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	select {
	case s.pricingExportSlot <- struct{}{}:
		defer func() { <-s.pricingExportSlot }()
	default:
		return pricing.ExportResourceLimit
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	options := &sql.TxOptions{ReadOnly: true}
	if IsPostgres(s.db) {
		options.Isolation = sql.LevelRepeatableRead
	}
	tx, err := s.db.BeginTx(ctx, options)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if IsPostgres(s.db) {
		if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout='15s'`); err != nil {
			return err
		}
	}
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return err
	}
	z := zip.NewWriter(&pricingExportLimit{w: writer, remaining: 750 << 20})
	jsonFile := func(name string, value any) error {
		w, e := z.Create("bapo-export/" + name)
		if e != nil {
			return e
		}
		return json.NewEncoder(w).Encode(value)
	}
	datasets := pricing.Datasets()
	if err = jsonFile("schema/tables.json", datasets); err != nil {
		return err
	}
	if err = jsonFile("schema/units.json", map[string]any{"integers": "decimal strings preserving signed 64-bit precision", "money": "TXB minor units (1/100 TXB)", "traffic": "bytes", "probability": "unit interval", "timezone": "UTC", "timestampSuffix_ns": "Unix nanoseconds, exact decimal strings", "missing": "null; never fabricated zero"}); err != nil {
		return err
	}
	if err = jsonFile("schema/features.json", map[string]any{"definitionSource": "forecasts/model_versions", "lineage": "feature_snapshot_hash joins immutable forecast inputs to features.snapshot_hash", "rawMachineTelemetry": "memory only; never collected into retained SQL history"}); err != nil {
		return err
	}
	files := []pricingExportFile{}
	omitted := []string{}
	digest := sha256.New()
	for _, d := range datasets {
		if !pricingDatasetIncluded(d, scope) {
			omitted = append(omitted, d.Name)
			continue
		}
		meta, e := streamPricingDataset(ctx, tx, z, d, scope, key)
		if e != nil {
			return e
		}
		files = append(files, meta)
		digest.Write([]byte(meta.Name + ":" + meta.SHA256 + "\n"))
	}
	var watermark sql.NullString
	if err = tx.QueryRowContext(ctx, `SELECT MAX(updated_at) FROM xnode_report_cursors`).Scan(&watermark); err != nil {
		return err
	}
	manifestScope := scope
	manifestScope.Users = append([]string(nil), scope.Users...)
	if scope.Pseudonymize {
		for i, id := range manifestScope.Users {
			manifestScope.Users[i] = pseudonym(key, id)
		}
	}
	var sourceWatermark any
	if watermark.Valid {
		sourceWatermark = watermark.String
	}
	version := "development"
	if build, ok := debug.ReadBuildInfo(); ok {
		version = build.Main.Version
		for _, setting := range build.Settings {
			if setting.Key == "vcs.revision" {
				version = setting.Value
			}
		}
	}
	manifest := map[string]any{"exportSchemaVersion": "bapo-5.1-v1", "applicationVersion": version, "createdAt": time.Now().UTC(), "consistency": "one read-only repeatable database snapshot; sequential cursor streams", "sourceWatermark": sourceWatermark, "timezone": "UTC", "scope": manifestScope, "files": files, "datasetSha256": hex.EncodeToString(digest.Sum(nil)), "omittedDatasets": omitted, "redactedColumns": []string{"authentication secrets", "sessions", "passwords", "API and private keys", "Xray credentials", "connection IPs"}, "pseudonymization": map[string]any{"enabled": scope.Pseudonymize, "joins": "HMAC-SHA256 stable within this export; random key is not exported"}, "completeness": map[string]any{"meaning": "every retained authorized row in scope; omitted datasets require scope/authorization", "rawTelemetry": "unavailable: machine metrics are memory only", "purgedHistory": "cannot be reconstructed"}, "checksums": "SHA-256 of compressed data files; manifest and schema entries use ZIP integrity CRC"}
	if err = jsonFile("manifest.json", manifest); err != nil {
		return err
	}
	if err = z.Close(); err != nil {
		return err
	}
	return tx.Commit()
}
