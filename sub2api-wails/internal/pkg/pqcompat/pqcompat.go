package pqcompat

import (
	"database/sql/driver"
	"fmt"
	"strings"
)

type arrayWrapper struct {
	a interface{}
}

func Array(a interface{}) any {
	return &arrayWrapper{a: a}
}

func (a *arrayWrapper) Value() (driver.Value, error) {
	switch v := a.a.(type) {
	case []int64:
		if v == nil {
			return "[]", nil
		}
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = fmt.Sprintf("%d", x)
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	case []string:
		if v == nil {
			return "[]", nil
		}
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = fmt.Sprintf(`"%s"`, strings.ReplaceAll(x, `"`, `\"`))
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	case []int:
		if v == nil {
			return "[]", nil
		}
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = fmt.Sprintf("%d", x)
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	default:
		return fmt.Sprintf("%v", a.a), nil
	}
}

func (a *arrayWrapper) Scan(src interface{}) error {
	return nil
}

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("pq: %s: %s", e.Code, e.Message)
}

func QuoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func QuoteLiteral(literal string) string {
	return "'" + strings.ReplaceAll(literal, "'", "''") + "'"
}

func CopyIn(table string, columns ...string) string {
	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = QuoteIdentifier(col)
	}
	return fmt.Sprintf("COPY %s (%s) FROM STDIN", QuoteIdentifier(table), strings.Join(quotedCols, ", "))
}
