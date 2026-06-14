package pgadapter

import (
	"encoding/json"
	"fmt"
	"strings"
)

func toJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal json: %w", err)
	}
	return string(b), nil
}

func fromJSON(s string, v any) error {
	if err := json.Unmarshal([]byte(s), v); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}
	return nil
}

// inPlaceholders returns "$1,$2,...,$n" for SQL IN clauses where all params
// start at position 1. Use inPlaceholdersAt when other params precede them.
func inPlaceholders(n int) string {
	return inPlaceholdersAt(n, 1)
}

// inPlaceholdersAt returns "$start,...,$(start+n-1)".
func inPlaceholdersAt(n, start int) string {
	if n == 0 {
		return ""
	}
	parts := make([]string, n)
	for i := range parts {
		parts[i] = fmt.Sprintf("$%d", start+i)
	}
	return strings.Join(parts, ",")
}

func stringsToAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
