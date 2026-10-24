package postgres

import (
	"database/sql/driver"
	"fmt"
	"strconv"
	"strings"
)

// Bind converts the explicitly supported legacy SQL grammar. Unknown metadata,
// PRAGMA and implicit row identities remain errors rather than unsafe emulation.
func Bind(query string, args []driver.NamedValue) (string, error) {
	tokens, err := tokenize(query)
	if err != nil {
		return "", err
	}
	parameter := 0
	ignore := false
	for i := range tokens {
		t := &tokens[i]
		if t.raw == "/*pg:claim-generation*/" {
			t.raw = ""
			j := significant(tokens, i+1)
			if j < len(tokens) && tokens[j].raw == "0" {
				tokens[j].raw = "claim_generation"
			}
			continue
		}
		if t.raw == "/*pg:advance-claim*/" {
			t.raw = ",claim_generation=claim_generation+1"
			continue
		}
		if t.raw == "/*pg:FOR UPDATE*/" {
			t.raw = "FOR UPDATE"
			continue
		}
		if t.raw == "/*pg:FOR UPDATE SKIP LOCKED*/" {
			t.raw = "FOR UPDATE SKIP LOCKED"
			continue
		}
		if t.protected {
			continue
		}
		if t.raw == "?" {
			if i+1 < len(tokens) && (tokens[i+1].raw == "|" || tokens[i+1].raw == "&") {
				return "", fmt.Errorf("use authored PostgreSQL parameters for JSON operators")
			}
			parameter++
			if parameter > len(args) {
				return "", fmt.Errorf("SQL placeholder count exceeds arguments")
			}
			t.raw = "$" + strconv.Itoa(parameter)
			switch args[parameter-1].Value.(type) {
			case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
				t.raw += "::bigint"
			}
			continue
		}
		if !t.word {
			continue
		}
		switch strings.ToUpper(t.raw) {
		case "JSON":
			if j := significant(tokens, i+1); j < len(tokens) && tokens[j].raw == "(" {
				t.raw = "vansail_json"
			}
		case "INTEGER":
			t.raw = "BIGINT"
		case "REAL":
			t.raw = "DOUBLE PRECISION"
		case "LIKE":
			t.raw = "ILIKE"
		case "ROWID":
			t.raw = "record_order"
		case "PRAGMA", "SQLITE_SCHEMA", "SQLITE_MASTER":
			return "", fmt.Errorf("SQLite-only SQL requires a backend implementation")
		case "COLLATE":
			j := significant(tokens, i+1)
			if j < len(tokens) && strings.EqualFold(tokens[j].raw, "NOCASE") {
				t.raw = ""
				tokens[j].raw = ""
			}
		case "IS":
			j := significant(tokens, i+1)
			if j >= len(tokens) {
				return "", fmt.Errorf("incomplete SQL comparison")
			}
			not := strings.EqualFold(tokens[j].raw, "NOT")
			k := j
			if not {
				k = significant(tokens, j+1)
			}
			if k < len(tokens) && !strings.EqualFold(tokens[k].raw, "NULL") && !strings.EqualFold(tokens[k].raw, "TRUE") && !strings.EqualFold(tokens[k].raw, "FALSE") && !strings.EqualFold(tokens[k].raw, "DISTINCT") {
				if not {
					t.raw = "IS DISTINCT FROM"
					tokens[j].raw = ""
				} else {
					t.raw = "IS NOT DISTINCT FROM"
				}
			}
		case "MAX", "MIN":
			j := significant(tokens, i+1)
			if j < len(tokens) && tokens[j].raw == "(" && multipleArguments(tokens, j) {
				if strings.EqualFold(t.raw, "MAX") {
					t.raw = "GREATEST"
				} else {
					t.raw = "LEAST"
				}
			}
		case "INSERT":
			j := significant(tokens, i+1)
			k := significant(tokens, j+1)
			if j < len(tokens) && k < len(tokens) && strings.EqualFold(tokens[j].raw, "OR") && strings.EqualFold(tokens[k].raw, "IGNORE") {
				tokens[j].raw = ""
				tokens[k].raw = ""
				ignore = true
			}
		}
	}
	if parameter > 0 && parameter != len(args) {
		return "", fmt.Errorf("SQL argument count differs from placeholders")
	}
	if ignore {
		// Existing INSERT OR IGNORE calls are single statements. Refuse a new
		// compound form until it has an explicit reviewed conversion.
		position := len(tokens)
		for i, t := range tokens {
			if !t.protected && (t.raw == ";" || strings.EqualFold(t.raw, "RETURNING")) {
				position = i
				break
			}
		}
		query = joinTokens(tokens[:position]) + " ON CONFLICT DO NOTHING " + joinTokens(tokens[position:])
	} else {
		query = joinTokens(tokens)
	}
	return query, nil
}

func multipleArguments(tokens []token, start int) bool {
	depth := 0
	for _, t := range tokens[start:] {
		if t.protected {
			continue
		}
		switch t.raw {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return false
			}
		case ",":
			if depth == 1 {
				return true
			}
		}
	}
	return false
}
