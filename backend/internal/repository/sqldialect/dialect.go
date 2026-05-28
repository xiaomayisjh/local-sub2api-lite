package sqldialect

import (
	"regexp"
	"strings"
	"sync"
)

const (
	DriverPostgres = "postgres"
	DriverSQLite   = "sqlite"
)

var (
	currentDriver   string
	driverMu        sync.RWMutex
	pgPlaceholder   = regexp.MustCompile(`\$\d+`)
	pgNowFunc       = regexp.MustCompile(`(?i)\bNOW\s*\(\s*\)`)
)

// SetDriver records the active SQL driver for placeholder rebinding.
func SetDriver(driver string) {
	driverMu.Lock()
	defer driverMu.Unlock()
	currentDriver = strings.ToLower(strings.TrimSpace(driver))
}

// Driver returns the active SQL driver name.
func Driver() string {
	driverMu.RLock()
	defer driverMu.RUnlock()
	return currentDriver
}

// UsesSQLite reports whether the active driver is SQLite.
func UsesSQLite() bool {
	return Driver() == DriverSQLite
}

// Rebind converts PostgreSQL $N placeholders to SQLite ? when needed.
// On SQLite it also rewrites NOW() to datetime('now').
func Rebind(query string) string {
	if Driver() != DriverSQLite {
		return query
	}
	query = pgNowFunc.ReplaceAllString(query, "datetime('now')")
	return pgPlaceholder.ReplaceAllString(query, "?")
}

// NowExpr returns a dialect-appropriate current timestamp expression.
func NowExpr() string {
	if Driver() == DriverSQLite {
		return "datetime('now')"
	}
	return "NOW()"
}

// SecondsBeforeNowExpr returns SQL for a timestamp threshold N seconds before now.
// placeholder must be a bind placeholder such as $3 (rebound to ? on SQLite).
func SecondsBeforeNowExpr(placeholder string) string {
	if Driver() == DriverSQLite {
		return "datetime('now', '-' || " + placeholder + " || ' seconds')"
	}
	return "NOW() - (" + placeholder + " * interval '1 second')"
}
