package sqliteadapter

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const tsFormat = time.RFC3339Nano

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

func nowStr() string { return time.Now().UTC().Format(tsFormat) }

func parseTime(s string) (time.Time, error) { return time.Parse(tsFormat, s) }

// inPlaceholders returns n comma-separated "?" tokens for SQL IN clauses.
func inPlaceholders(n int) string {
	if n == 0 {
		return ""
	}
	return "?" + strings.Repeat(",?", n-1)
}

// stringsToAny converts a []string to []any for use as sql query args.
func stringsToAny(ss []string) []any {
	out := make([]any, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
