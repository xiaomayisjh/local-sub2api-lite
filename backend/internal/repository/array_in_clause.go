package repository

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/repository/sqldialect"
	"github.com/lib/pq"
)

// arrayInClause renders a SQL fragment for "<column> IN (...)" or "<column> = ANY($N)"
// depending on the active driver.
//
// Returns (sqlFragment, args). The caller is responsible for sticking the fragment
// into the larger query. PostgreSQL path uses pq.Array (with the supplied placeholder
// index, e.g. "$1"); SQLite path generates a comma-separated "?" list and appends
// individual scalars to args (so the SQLite Rebind path doesn't need any further help).
//
// Example usage on a query that originally used `account_id = ANY($1)`:
//
//	frag, expanded := arrayInClause("account_id", accountIDs, "$1")
//	args := append(expanded, startTime) // append other args after
//	query := "... WHERE " + frag + " AND created_at >= $2 ..."
//
// Note: when SQLite is active and arrayInClause emits placeholders for accountIDs,
// any remaining query placeholders (e.g. $2, $3) get rewritten to ? by Rebind in
// the executor. arrayInClause does NOT shift placeholder numbering for you, so it
// is safe to keep $2/$3 in the literal query — Rebind sees them and converts.
func arrayInClause(column string, ids []int64, pgPlaceholder string) (string, []any) {
	if sqldialect.UsesSQLite() {
		if len(ids) == 0 {
			// "0 = 1" keeps the WHERE clause syntactically valid while matching nothing.
			return "0 = 1", nil
		}
		placeholders := make([]string, len(ids))
		args := make([]any, len(ids))
		for i, id := range ids {
			placeholders[i] = "?"
			args[i] = id
		}
		return column + " IN (" + strings.Join(placeholders, ",") + ")", args
	}
	return column + " = ANY(" + pgPlaceholder + ")", []any{pq.Array(ids)}
}
