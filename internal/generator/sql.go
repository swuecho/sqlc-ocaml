package generator

import (
	"fmt"
	"strconv"
	"strings"
)

// caqtiSQL converts PostgreSQL's $n placeholders to Caqti's portable ? syntax
// and returns their logical parameter numbers in occurrence order.
func caqtiSQL(sql string, params int) (string, []int, error) {
	var b strings.Builder
	var occurrences []int
	for i := 0; i < len(sql); {
		if delimiter, ok := dollarQuoteDelimiter(sql, i); ok {
			end := strings.Index(sql[i+len(delimiter):], delimiter)
			if end < 0 {
				return "", nil, fmt.Errorf("unterminated SQL dollar-quoted string")
			}
			end += i + 2*len(delimiter)
			b.WriteString(sql[i:end])
			i = end
			continue
		}
		if sql[i] == '\'' || sql[i] == '"' {
			quote, start, closed := sql[i], i, false
			escapeBackslash := quote == '\'' && i > 0 && (sql[i-1] == 'e' || sql[i-1] == 'E') && (i == 1 || !isSQLIdentifierByte(sql[i-2]))
			i++
			for i < len(sql) {
				if escapeBackslash && sql[i] == '\\' && i+1 < len(sql) {
					i += 2
					continue
				}
				if sql[i] == quote {
					i++
					if i < len(sql) && sql[i] == quote {
						i++
						continue
					}
					closed = true
					break
				}
				i++
			}
			if !closed {
				return "", nil, fmt.Errorf("unterminated SQL quoted string or identifier")
			}
			b.WriteString(sql[start:i])
			continue
		}
		if i+1 < len(sql) && sql[i:i+2] == "--" {
			j := strings.IndexByte(sql[i:], '\n')
			if j < 0 {
				b.WriteString(sql[i:])
				i = len(sql)
			} else {
				j += i + 1
				b.WriteString(sql[i:j])
				i = j
			}
			continue
		}
		if i+1 < len(sql) && sql[i:i+2] == "/*" {
			start, depth := i, 1
			i += 2
			for i < len(sql) && depth > 0 {
				switch {
				case i+1 < len(sql) && sql[i:i+2] == "/*":
					depth++
					i += 2
				case i+1 < len(sql) && sql[i:i+2] == "*/":
					depth--
					i += 2
				default:
					i++
				}
			}
			if depth != 0 {
				return "", nil, fmt.Errorf("unterminated SQL block comment")
			}
			b.WriteString(sql[start:i])
			continue
		}
		if sql[i] == '$' && i+1 < len(sql) && sql[i+1] >= '0' && sql[i+1] <= '9' {
			j := i + 1
			for j < len(sql) && sql[j] >= '0' && sql[j] <= '9' {
				j++
			}
			n, err := strconv.Atoi(sql[i+1 : j])
			if err != nil || n < 1 || n > params {
				return "", nil, fmt.Errorf("SQL placeholder %s has no matching parameter (metadata contains %d)", sql[i:j], params)
			}
			b.WriteByte('?')
			occurrences = append(occurrences, n)
			i = j
			continue
		}
		b.WriteByte(sql[i])
		i++
	}
	used := make([]bool, params)
	for _, n := range occurrences {
		used[n-1] = true
	}
	for i, ok := range used {
		if !ok {
			return "", nil, fmt.Errorf("query metadata parameter $%d is not used by SQL", i+1)
		}
	}
	return b.String(), occurrences, nil
}

func dollarQuoteDelimiter(sql string, start int) (string, bool) {
	if start >= len(sql) || sql[start] != '$' {
		return "", false
	}
	for i := start + 1; i < len(sql); i++ {
		if sql[i] == '$' {
			return sql[start : i+1], true
		}
		if (i == start+1 && !isSQLIdentifierStartByte(sql[i])) || (i > start+1 && !isSQLIdentifierByte(sql[i])) {
			return "", false
		}
	}
	return "", false
}

func isSQLIdentifierStartByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isSQLIdentifierByte(c byte) bool {
	return isSQLIdentifierStartByte(c) || c >= '0' && c <= '9'
}
