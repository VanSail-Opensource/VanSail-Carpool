package postgres

import (
	"encoding/json"
)

// ColumnContract is the native PostgreSQL column contract.
type ColumnContract struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	NotNull    bool   `json:"notNull"`
	PrimaryKey int    `json:"primaryKey"`
}

// TableContract names authoritative application columns.
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

// SchemaContract returns the embedded inventory used by reconciliation tooling.
func SchemaContract() (Contract, error) {
	var c Contract
	err := json.Unmarshal(contractJSON, &c)
	return c, err
}
