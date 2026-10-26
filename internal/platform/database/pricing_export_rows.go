package database

import (
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing"
)

type pricingExportFile struct {
	Name          string   `json:"name"`
	Rows          int64    `json:"rows,string"`
	SHA256        string   `json:"sha256"`
	Columns       []string `json:"columns"`
	Types         []string `json:"databaseTypes"`
	RetentionDays int      `json:"retentionDays"`
}

func pseudonym(key []byte, value string) string {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(value))
	return "user_" + hex.EncodeToString(h.Sum(nil))[:24]
}

func pricingRedactRecord(columns []string, values []any, key []byte) {
	entityType := ""
	scopeType := ""
	for i, c := range columns {
		if c == "entity_type" {
			entityType = fmt.Sprint(values[i])
		}
		if c == "scope_type" {
			scopeType = fmt.Sprint(values[i])
		}
	}
	for i, c := range columns {
		if values[i] == nil {
			continue
		}
		if c == "scope_id" && scopeType == "user_product" {
			values[i] = pricingPseudonymScope(fmt.Sprint(values[i]), key)
		}
		if c == "id" && strings.HasPrefix(fmt.Sprint(values[i]), "user:") {
			parts := strings.SplitN(fmt.Sprint(values[i]), ":", 3)
			if len(parts) == 3 {
				values[i] = "user:" + pseudonym(key, parts[1]) + ":" + parts[2]
			}
		}
		if strings.HasSuffix(c, "_json") {
			values[i] = redactPricingJSON(fmt.Sprint(values[i]), key)
		}
		if c == "user_id" || c == "actor_id" || c == "updated_by" || c == "created_by" || c == "approved_by" || c == "acknowledged_by" || c == "id" && len(columns) == 3 && columns[1] == "created_at" {
			values[i] = pseudonym(key, fmt.Sprint(values[i]))
		}
		if c == "entity_id" && entityType == "user" {
			values[i] = pseudonym(key, fmt.Sprint(values[i]))
		}
		if c == "entity_id" && entityType == "user_node" {
			parts := strings.SplitN(fmt.Sprint(values[i]), ":", 2)
			if len(parts) == 2 {
				values[i] = pseudonym(key, parts[0]) + ":" + parts[1]
			}
		}
	}
}

func streamPricingDataset(ctx context.Context, tx *sql.Tx, z *zip.Writer, d pricing.Dataset, scope PricingExportScope, key []byte) (pricingExportFile, error) {
	meta := pricingExportFile{Name: pricingExportName(d, scope.Format), RetentionDays: d.RetentionDays}
	query, args := pricingDatasetQuery(d, scope)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return meta, fmt.Errorf("export %s: %w", d.Name, err)
	}
	defer rows.Close()
	meta.Columns, err = rows.Columns()
	if err != nil {
		return meta, err
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return meta, err
	}
	for _, t := range types {
		meta.Types = append(meta.Types, t.DatabaseTypeName())
	}
	entry, err := z.CreateHeader(&zip.FileHeader{Name: meta.Name, Method: zip.Store})
	if err != nil {
		return meta, err
	}
	hash := sha256.New()
	gz, err := gzip.NewWriterLevel(io.MultiWriter(entry, hash), gzip.BestSpeed)
	if err != nil {
		return meta, err
	}
	csvWriter := csv.NewWriter(gz)
	encoder := json.NewEncoder(gz)
	encoder.SetEscapeHTML(false)
	if scope.Format == "csv" {
		if err = csvWriter.Write(meta.Columns); err != nil {
			return meta, err
		}
	}
	values := make([]any, len(meta.Columns))
	pointers := make([]any, len(values))
	for i := range values {
		pointers[i] = &values[i]
	}
	for rows.Next() {
		if err = ctx.Err(); err != nil {
			return meta, err
		}
		if err = rows.Scan(pointers...); err != nil {
			return meta, err
		}
		for i, v := range values {
			switch n := v.(type) {
			case []byte:
				if len(n) > 256<<10 {
					return meta, pricing.ExportResourceLimit
				}
				values[i] = string(n)
			case int64:
				values[i] = strconv.FormatInt(n, 10)
			case string:
				if len(n) > 256<<10 {
					return meta, pricing.ExportResourceLimit
				}
			}
		}
		if scope.Pseudonymize {
			pricingRedactRecord(meta.Columns, values, key)
		}
		if scope.Format == "csv" {
			record := make([]string, len(values))
			for i, v := range values {
				if v != nil {
					record[i] = fmt.Sprint(v)
					if strings.Contains(strings.ToUpper(meta.Types[i]), "TEXT") && strings.ContainsAny(record[i][:min(1, len(record[i]))], "=+-@\t\r") {
						record[i] = "'" + record[i]
					}
				}
			}
			if err = csvWriter.Write(record); err != nil {
				return meta, err
			}
		} else {
			record := make(map[string]any, len(values))
			for i, c := range meta.Columns {
				record[c] = values[i]
			}
			if err = encoder.Encode(record); err != nil {
				return meta, err
			}
		}
		meta.Rows++
	}
	if err = rows.Err(); err != nil {
		return meta, err
	}
	csvWriter.Flush()
	if err = csvWriter.Error(); err != nil {
		return meta, err
	}
	if err = gz.Close(); err != nil {
		return meta, err
	}
	meta.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return meta, nil
}
