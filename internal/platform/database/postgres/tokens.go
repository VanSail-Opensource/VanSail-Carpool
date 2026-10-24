package postgres

import (
	"fmt"
	"strings"
	"unicode"
)

type token struct {
	raw       string
	word      bool
	protected bool
}

// tokenize recognizes SQL strings, quoted identifiers, dollar strings, nested
// comments and parameter operators before binding. Protected text is never edited.
func tokenize(query string) ([]token, error) {
	var result []token
	for i := 0; i < len(query); {
		start := i
		protected, word := false, false
		switch {
		case strings.HasPrefix(query[i:], "--"):
			protected = true
			for i < len(query) && query[i] != '\n' {
				i++
			}
		case strings.HasPrefix(query[i:], "/*"):
			protected = true
			i += 2
			depth := 1
			for i < len(query) && depth > 0 {
				if strings.HasPrefix(query[i:], "/*") {
					depth++
					i += 2
				} else if strings.HasPrefix(query[i:], "*/") {
					depth--
					i += 2
				} else {
					i++
				}
			}
			if depth > 0 {
				return nil, fmt.Errorf("unterminated SQL comment")
			}
		case query[i] == '\'' || query[i] == '"' || query[i] == '`':
			protected = true
			quote := query[i]
			i++
			closed := false
			for i < len(query) {
				if query[i] == quote {
					i++
					if i < len(query) && query[i] == quote {
						i++
						continue
					}
					closed = true
					break
				}
				if query[i] == '\\' && start > 0 && (query[start-1] == 'E' || query[start-1] == 'e') && i+1 < len(query) {
					i += 2
				} else {
					i++
				}
			}
			if !closed {
				return nil, fmt.Errorf("unterminated SQL quote")
			}
		case query[i] == '$':
			j := i + 1
			for j < len(query) && (query[j] == '_' || unicode.IsLetter(rune(query[j])) || (j > i+1 && query[j] >= '0' && query[j] <= '9')) {
				j++
			}
			if j < len(query) && query[j] == '$' {
				protected = true
				delimiter := query[i : j+1]
				end := strings.Index(query[j+1:], delimiter)
				if end < 0 {
					return nil, fmt.Errorf("unterminated SQL dollar quote")
				}
				i = j + 1 + end + len(delimiter)
			} else {
				i++
			}
		case query[i] == '_' || unicode.IsLetter(rune(query[i])):
			word = true
			i++
			for i < len(query) && (query[i] == '_' || unicode.IsLetter(rune(query[i])) || (query[i] >= '0' && query[i] <= '9')) {
				i++
			}
		default:
			i++
		}
		result = append(result, token{query[start:i], word, protected})
	}
	return result, nil
}

func significant(tokens []token, start int) int {
	for start < len(tokens) && (strings.TrimSpace(tokens[start].raw) == "" || strings.HasPrefix(tokens[start].raw, "--") || strings.HasPrefix(tokens[start].raw, "/*")) {
		start++
	}
	return start
}

func joinTokens(tokens []token) string {
	var b strings.Builder
	for _, t := range tokens {
		b.WriteString(t.raw)
	}
	return b.String()
}
