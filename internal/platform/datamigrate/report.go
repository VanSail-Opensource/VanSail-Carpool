package datamigrate

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

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/database/postgres"
)

// TableDigest covers every original column, including exact ciphertext/payloads.
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

// Inspect hashes one frozen source snapshot and checks recoverability invariants.
func Inspect(ctx context.Context, source *Source) (Report, error) {
	return inspect(ctx, source.Tx, source.Tables, false)
}

func inspect(ctx context.Context, q queryer, tables []postgres.TableContract, pg bool) (Report, error) {
	report := Report{Tables: make([]TableDigest, 0, len(tables))}
	var err error
	report.Sequences, err = readSequences(ctx, q, pg)
	if err != nil {
		return report, err
	}
	for _, table := range tables {
		digest, err := digestTable(ctx, q, table, pg)
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

func digestTable(ctx context.Context, q queryer, table postgres.TableContract, pg bool) (TableDigest, error) {
	result := TableDigest{Name: table.Name}
	columns, order := tableSelection(table, pg)
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

func tableSelection(table postgres.TableContract, pg bool) (string, string) {
	var columns []string
	var keys []postgres.ColumnContract
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
		if c.Type == "TEXT" {
			if pg {
				key = "convert_to(" + key + ",'UTF8')"
			} else {
				key = "CAST(" + key + " AS BLOB)"
			}
		}
		order = append(order, key)
	}
	if hasOrdering(table.Name) {
		if pg {
			columns = append(columns, "record_order")
		} else {
			columns = append(columns, "rowid")
		}
	}
	return strings.Join(columns, ","), strings.Join(order, ",")
}

func hashValue(h hash.Hash, value any) error {
	var kind, text string
	switch v := value.(type) {
	case nil:
		kind = "n"
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
	default:
		return fmt.Errorf("unsupported migration value type %T", value)
	}
	_, err := fmt.Fprintf(h, "%s%d:%s;", kind, len(text), text)
	return err
}
