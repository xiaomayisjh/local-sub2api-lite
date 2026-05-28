package repository

import (
	"context"
	"database/sql"

	"github.com/Wei-Shaw/sub2api/internal/repository/sqldialect"
)

// SQLExecutorFromDB returns an sqlExecutor that rebinds placeholders on SQLite.
func SQLExecutorFromDB(db *sql.DB) sqlExecutor {
	if db == nil {
		return nil
	}
	return wrapSQLExecutor(db)
}

// RebindDB wraps *sql.DB and rewrites PostgreSQL placeholders for SQLite.
type RebindDB struct {
	*sql.DB
}

// WrapDB returns a RebindDB when using SQLite, otherwise the original *sql.DB as RebindDB wrapper.
func WrapDB(db *sql.DB) *RebindDB {
	if db == nil {
		return nil
	}
	if sqldialect.Driver() != sqldialect.DriverSQLite {
		return &RebindDB{DB: db}
	}
	return &RebindDB{DB: db}
}

func (d *RebindDB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.DB.ExecContext(ctx, sqldialect.Rebind(query), args...)
}

func (d *RebindDB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.DB.QueryContext(ctx, sqldialect.Rebind(query), args...)
}

func (d *RebindDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.DB.QueryRowContext(ctx, sqldialect.Rebind(query), args...)
}

func (d *RebindDB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*RebindTx, error) {
	tx, err := d.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &RebindTx{Tx: tx}, nil
}

// RebindTx wraps sql.Tx with placeholder rebinding.
type RebindTx struct {
	*sql.Tx
}

func (t *RebindTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.Tx.ExecContext(ctx, sqldialect.Rebind(query), args...)
}

func (t *RebindTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return t.Tx.QueryContext(ctx, sqldialect.Rebind(query), args...)
}

func (t *RebindTx) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return t.Tx.QueryRowContext(ctx, sqldialect.Rebind(query), args...)
}

// rebindExecutor wraps sqlExecutor and rebinds PostgreSQL placeholders to SQLite ? when needed.
type rebindExecutor struct {
	inner sqlExecutor
}

func wrapSQLExecutor(inner sqlExecutor) sqlExecutor {
	if inner == nil || sqldialect.Driver() != sqldialect.DriverSQLite {
		return inner
	}
	return &rebindExecutor{inner: inner}
}

func (r *rebindExecutor) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.inner.ExecContext(ctx, sqldialect.Rebind(query), args...)
}

func (r *rebindExecutor) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.inner.QueryContext(ctx, sqldialect.Rebind(query), args...)
}
