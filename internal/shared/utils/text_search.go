package utils

import (
	"fmt"
	"strings"
)

// TokenizedMatchSQL builds a WHERE subexpression that requires every
// whitespace-separated token of `search` to match (case-insensitively,
// as a substring) at least one of the given columns. In other words it
// applies Meilisearch-style "all tokens must appear" semantics to plain
// SQL text matching: tokens may appear in different columns and in any
// order. The previous single "col ILIKE '%whole query%'" term failed
// whenever the query contained two or more words that only appear
// separately or in a different order than stored.
//
// It returns the SQL fragment and its bind arguments. If the term is empty
// or blank, it returns an empty fragment and nil args and callers should
// skip adding a WHERE clause.
func TokenizedMatchSQL(search string, columns []string) (string, []interface{}) {
	tokens := strings.Fields(strings.TrimSpace(search))
	if len(tokens) == 0 || len(columns) == 0 {
		return "", nil
	}
	var tokenClauses []string
	var args []interface{}
	for _, tok := range tokens {
		like := "%" + strings.ToLower(tok) + "%"
		var colClauses []string
		for _, col := range columns {
			colClauses = append(colClauses, fmt.Sprintf("LOWER(COALESCE(%s,'')) LIKE ?", col))
		}
		tokenClauses = append(tokenClauses, "("+strings.Join(colClauses, " OR ")+")")
		for range columns {
			args = append(args, like)
		}
	}
	return strings.Join(tokenClauses, " AND "), args
}
