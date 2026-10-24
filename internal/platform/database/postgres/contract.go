package postgres

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// ColumnContract is the exact legacy representation retained during conversion.
type ColumnContract struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	NotNull    bool   `json:"notNull"`
	PrimaryKey int    `json:"primaryKey"`
}

// TableContract names source columns; PostgreSQL operational columns are separate.
type TableContract struct {
	Name    string           `json:"name"`
	Columns []ColumnContract `json:"columns"`
}

// Contract is generated schema metadata, never application or upstream data.
type Contract struct {
	Tables   []TableContract `json:"tables"`
	Order    []string        `json:"order"`
	Indexes  []string        `json:"indexes"`
	Triggers []string        `json:"triggers"`
}

//go:embed source-schema.sha256
var sourceSchemaHash string

// SourceSchemaHash pins the canonical final SQLite schema accepted for import.
func SourceSchemaHash() string { return strings.TrimSpace(sourceSchemaHash) }

// SchemaContract returns the embedded inventory used by reconciliation tooling.
func SchemaContract() (Contract, error) {
	var c Contract
	err := json.Unmarshal(contractJSON, &c)
	return c, err
}
