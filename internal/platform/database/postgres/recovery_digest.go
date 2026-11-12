package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// TableDigest covers every authoritative column, including exact ciphertext/payloads.
type TableDigest struct {
	Name string `json:"name"`
	Rows int64  `json:"rows"`
	Hash string `json:"sha256"`
}

// Report is deterministic evidence, without plaintext rows or credentials.
type Report struct {
	Sequences           map[string]int64 `json:"sequences"`
	Tables              []TableDigest    `json:"tables"`
	Hash                string           `json:"sha256"`
	ProcessingJobs      int64            `json:"processingJobs"`
	ProcessingProviders int64            `json:"processingProviders"`
	FinancialMismatches int64            `json:"financialMismatches"`
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func hasOrdering(table string) bool {
	return table == "outbox_jobs" || table == "provider_operations" || table == "ip_lookup_reports" || table == "ip_lookup_live_snapshots"
}

func inspect(ctx context.Context, q queryer, tables []TableContract) (Report, error) {
	report := Report{Tables: make([]TableDigest, 0, len(tables))}
	var err error
	report.Sequences, err = readSequences(ctx, q)
	if err != nil {
		return report, err
	}
	for _, table := range tables {
		digest, err := digestTable(ctx, q, table)
		if err != nil {
			return report, err
		}
		report.Tables = append(report.Tables, digest)
	}
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_jobs WHERE status='processing'`).Scan(&report.ProcessingJobs); err != nil {
		return report, err
	}
	if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM provider_operations WHERE status='processing'`).Scan(&report.ProcessingProviders); err != nil {
		return report, err
	}
	if err := checkBalances(ctx, q, &report); err != nil {
		return report, err
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return report, err
	}
	sum := sha256.Sum256(raw)
	report.Hash = hex.EncodeToString(sum[:])
	return report, nil
}

func digestTable(ctx context.Context, q queryer, table TableContract) (TableDigest, error) {
	result := TableDigest{Name: table.Name}
	columns, order := tableSelection(table)
	rows, err := q.QueryContext(ctx, `SELECT `+columns+` FROM `+quote(table.Name)+` ORDER BY `+order)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	count := len(table.Columns)
	if hasOrdering(table.Name) {
		count++
	}
	values := make([]any, count)
	dest := make([]any, count)
	for i := range values {
		dest[i] = &values[i]
	}
	h := sha256.New()
	for rows.Next() {
		if err = rows.Scan(dest...); err != nil {
			return result, err
		}
		for _, value := range values {
			if err = hashValue(h, value); err != nil {
				return result, err
			}
		}
		result.Rows++
	}
	if err = rows.Err(); err != nil {
		return result, err
	}
	result.Hash = hex.EncodeToString(h.Sum(nil))
	return result, nil
}

func tableSelection(table TableContract) (string, string) {
	var columns []string
	var keys []ColumnContract
	for _, c := range table.Columns {
		columns = append(columns, quote(c.Name))
		if c.PrimaryKey > 0 {
			keys = append(keys, c)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].PrimaryKey < keys[j].PrimaryKey })
	var order []string
	for _, c := range keys {
		key := quote(c.Name)

		order = append(order, key)
	}
	if hasOrdering(table.Name) {
		columns = append(columns, "record_order")
	}
	return strings.Join(columns, ","), strings.Join(order, ",")
}

func hashValue(h hash.Hash, value any) error {
	var kind, text string
	switch v := value.(type) {
	case nil:
		kind = "n"
	case bool:
		kind = "t"
		text = strconv.FormatBool(v)
	case int64:
		kind = "i"
		text = strconv.FormatInt(v, 10)
	case float64:
		kind = "f"
		text = strconv.FormatUint(math.Float64bits(v), 16)
	case string:
		kind = "s"
		text = v
	case []byte:
		kind = "b"
		text = hex.EncodeToString(v)
	case time.Time:
		// Native DATE/TIMESTAMPTZ values arrive through pgx as time.Time.
		// Canonical UTC removes driver/session location differences while
		// preserving all stored precision and a distinct scalar type.
		kind = "d"
		text = v.UTC().Format(time.RFC3339Nano)
	default:
		return fmt.Errorf("unsupported recovery value type %T", value)
	}
	_, err := fmt.Fprintf(h, "%s%d:%s;", kind, len(text), text)
	return err
}

func quote(value string) string { return `"` + strings.ReplaceAll(value, `"`, `""`) + `"` }
