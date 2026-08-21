package generator

import (
	"strings"
	"unicode"
)

var reserved = map[string]bool{
	"and": true, "as": true, "assert": true, "begin": true, "class": true, "constraint": true,
	"do": true, "done": true, "downto": true, "else": true, "end": true, "exception": true,
	"external": true, "false": true, "for": true, "fun": true, "function": true, "functor": true,
	"if": true, "in": true, "include": true, "inherit": true, "initializer": true, "lazy": true,
	"let": true, "match": true, "method": true, "module": true, "mutable": true, "new": true,
	"nonrec": true, "object": true, "of": true, "open": true, "or": true, "private": true,
	"rec": true, "sig": true, "struct": true, "then": true, "to": true, "true": true, "try": true,
	"type": true, "val": true, "virtual": true, "when": true, "while": true, "with": true,
}

func snake(s string) string {
	var b strings.Builder
	lastUnderscore := false
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 && !lastUnderscore {
				b.WriteByte('_')
			}
			r = unicode.ToLower(r)
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			if r == '_' {
				if lastUnderscore {
					continue
				}
				lastUnderscore = true
			} else {
				lastUnderscore = false
			}
			b.WriteRune(r)
		} else if !lastUnderscore && b.Len() > 0 {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	r := strings.Trim(b.String(), "_")
	if r == "" {
		r = "value"
	}
	if r[0] >= '0' && r[0] <= '9' {
		r = "v_" + r
	}
	if reserved[r] {
		r += "_"
	}
	return r
}

func constructor(s string) string {
	r := snake(s)
	if r == "" {
		return "Value"
	}
	return strings.ToUpper(r[:1]) + r[1:]
}

func moduleName(s string) string {
	parts := strings.Split(snake(s), "_")
	for i, part := range parts {
		if part != "" {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, "")
}
