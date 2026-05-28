package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/repository/sqldialect"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

const localBootstrapMigration = "local-bootstrap-v1"

// InitEntSQLite opens SQLite, creates schema via Ent, and runs local-mode bootstrap.
func InitEntSQLite(cfg *config.Config) (*ent.Client, *sql.DB, error) {
	if err := timezone.Init(cfg.Timezone); err != nil {
		return nil, nil, err
	}

	sqldialect.SetDriver(sqldialect.DriverSQLite)

	dbPath, err := resolveSQLitePath(cfg)
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, nil, fmt.Errorf("create sqlite directory: %w", err)
	}

	sqlDB, err := sql.Open("sqlite", cfg.Database.SqliteDSN(dbPath))
	if err != nil {
		return nil, nil, err
	}
	applyDBPoolSettings(sqlDB, cfg)

	drv := entsql.OpenDB(dialect.SQLite, sqlDB)
	client := ent.NewClient(ent.Driver(drv))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := ensureSQLiteSchema(ctx, client, sqlDB); err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	if err := ensureSQLiteCompatibilitySchema(ctx, sqlDB); err != nil {
		_ = client.Close()
		return nil, nil, err
	}

	if err := ensureBootstrapSecrets(ctx, client, cfg); err != nil {
		_ = client.Close()
		return nil, nil, err
	}
	if err := cfg.Validate(); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("validate config after secret bootstrap: %w", err)
	}

	if cfg.RunMode == config.RunModeLocal || cfg.RunMode == config.RunModeSimple {
		seedCtx, seedCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer seedCancel()
		if err := ensureSimpleModeDefaultGroups(seedCtx, client); err != nil {
			_ = client.Close()
			return nil, nil, err
		}
		if err := ensureSimpleModeAdminConcurrency(seedCtx, client); err != nil {
			_ = client.Close()
			return nil, nil, err
		}
	}

	if cfg.IsLocalMode() {
		bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer bootstrapCancel()
		if err := bootstrapLocalMode(bootstrapCtx, client, cfg); err != nil {
			_ = client.Close()
			return nil, nil, err
		}
	}

	return client, sqlDB, nil
}

func resolveSQLitePath(cfg *config.Config) (string, error) {
	path := strings.TrimSpace(cfg.Database.SqlitePath)
	if path == "" {
		path = "sub2api.db"
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	dataDir := configDataDir()
	return filepath.Join(dataDir, path), nil
}

func configDataDir() string {
	if dir := os.Getenv("DATA_DIR"); dir != "" {
		return dir
	}
	return "."
}

func ensureSQLiteSchema(ctx context.Context, client *ent.Client, db *sql.DB) error {
	if err := client.Schema.Create(ctx); err != nil {
		return fmt.Errorf("create sqlite schema: %w", err)
	}

	applied, err := sqliteMigrationApplied(ctx, db, localBootstrapMigration)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}
	return recordSQLiteMigration(ctx, db, localBootstrapMigration)
}

func ensureSQLiteCompatibilitySchema(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS user_provider_default_grants (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			provider_type TEXT NOT NULL,
			grant_reason TEXT NOT NULL DEFAULT 'first_bind',
			granted_at TEXT NOT NULL DEFAULT (datetime('now')),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			CHECK (provider_type IN ('email', 'linuxdo', 'wechat', 'oidc', 'github', 'google', 'dingtalk')),
			CHECK (grant_reason IN ('signup', 'first_bind'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS user_provider_default_grants_user_provider_reason_key
			ON user_provider_default_grants (user_id, provider_type, grant_reason)`,
		`CREATE INDEX IF NOT EXISTS user_provider_default_grants_user_id_idx
			ON user_provider_default_grants (user_id)`,
		`CREATE TABLE IF NOT EXISTS user_avatars (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			storage_provider TEXT NOT NULL DEFAULT 'database',
			storage_key TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			content_type TEXT NOT NULL DEFAULT '',
			byte_size INTEGER NOT NULL DEFAULT 0,
			sha256 TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS user_avatars_user_id_key
			ON user_avatars (user_id)`,
		`CREATE TABLE IF NOT EXISTS user_group_rate_multipliers (
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			rate_multiplier REAL,
			rpm_override INTEGER,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (user_id, group_id)
		)`,
		`CREATE INDEX IF NOT EXISTS user_group_rate_multipliers_group_id_idx
			ON user_group_rate_multipliers (group_id)`,
		`CREATE TABLE IF NOT EXISTS scheduled_test_plans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			model_id TEXT NOT NULL DEFAULT '',
			cron_expression TEXT NOT NULL DEFAULT '*/30 * * * *',
			enabled BOOLEAN NOT NULL DEFAULT TRUE,
			max_results INTEGER NOT NULL DEFAULT 50,
			auto_recover BOOLEAN NOT NULL DEFAULT FALSE,
			last_run_at TEXT,
			next_run_at TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stp_account_id ON scheduled_test_plans(account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_stp_enabled_next_run
			ON scheduled_test_plans(enabled, next_run_at)`,
		`CREATE TABLE IF NOT EXISTS scheduled_test_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plan_id INTEGER NOT NULL REFERENCES scheduled_test_plans(id) ON DELETE CASCADE,
			status TEXT NOT NULL DEFAULT 'success',
			response_text TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			started_at TEXT NOT NULL DEFAULT (datetime('now')),
			finished_at TEXT NOT NULL DEFAULT (datetime('now')),
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_str_plan_created
			ON scheduled_test_results(plan_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			model_mapping TEXT NOT NULL DEFAULT '{}',
			billing_model_source TEXT NOT NULL DEFAULT 'channel_mapped',
			restrict_models BOOLEAN NOT NULL DEFAULT FALSE,
			features TEXT NOT NULL DEFAULT '',
			features_config TEXT NOT NULL DEFAULT '{}',
			apply_pricing_to_account_stats BOOLEAN NOT NULL DEFAULT FALSE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_channels_name ON channels(name)`,
		`CREATE INDEX IF NOT EXISTS idx_channels_status ON channels(status)`,
		`CREATE TABLE IF NOT EXISTS channel_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
			group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_groups_group_id ON channel_groups(group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_groups_channel_id ON channel_groups(channel_id)`,
		`CREATE TABLE IF NOT EXISTS channel_model_pricing (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
			platform TEXT NOT NULL DEFAULT 'anthropic',
			models TEXT NOT NULL DEFAULT '[]',
			billing_mode TEXT NOT NULL DEFAULT 'token',
			input_price REAL,
			output_price REAL,
			cache_write_price REAL,
			cache_read_price REAL,
			image_output_price REAL,
			per_request_price REAL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_model_pricing_channel_id
			ON channel_model_pricing(channel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_model_pricing_platform
			ON channel_model_pricing(platform)`,
		`CREATE TABLE IF NOT EXISTS channel_pricing_intervals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pricing_id INTEGER NOT NULL REFERENCES channel_model_pricing(id) ON DELETE CASCADE,
			min_tokens INTEGER NOT NULL DEFAULT 0,
			max_tokens INTEGER,
			tier_label TEXT NOT NULL DEFAULT '',
			input_price REAL,
			output_price REAL,
			cache_write_price REAL,
			cache_read_price REAL,
			per_request_price REAL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_pricing_intervals_pricing_id
			ON channel_pricing_intervals(pricing_id)`,
		`CREATE TABLE IF NOT EXISTS channel_account_stats_pricing_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
			name TEXT NOT NULL DEFAULT '',
			group_ids TEXT NOT NULL DEFAULT '[]',
			account_ids TEXT NOT NULL DEFAULT '[]',
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cas_pricing_rules_channel_id
			ON channel_account_stats_pricing_rules(channel_id)`,
		`CREATE TABLE IF NOT EXISTS channel_account_stats_model_pricing (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id INTEGER NOT NULL REFERENCES channel_account_stats_pricing_rules(id) ON DELETE CASCADE,
			platform TEXT NOT NULL DEFAULT '',
			models TEXT NOT NULL DEFAULT '[]',
			billing_mode TEXT NOT NULL DEFAULT 'token',
			input_price REAL,
			output_price REAL,
			cache_write_price REAL,
			cache_read_price REAL,
			image_output_price REAL,
			per_request_price REAL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cas_model_pricing_rule_id
			ON channel_account_stats_model_pricing(rule_id)`,
		`CREATE TABLE IF NOT EXISTS channel_account_stats_pricing_intervals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			pricing_id INTEGER NOT NULL REFERENCES channel_account_stats_model_pricing(id) ON DELETE CASCADE,
			min_tokens INTEGER NOT NULL DEFAULT 0,
			max_tokens INTEGER,
			tier_label TEXT NOT NULL DEFAULT '',
			input_price REAL,
			output_price REAL,
			cache_write_price REAL,
			cache_read_price REAL,
			per_request_price REAL,
			sort_order INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_account_stats_pricing_intervals_pricing_id
			ON channel_account_stats_pricing_intervals(pricing_id)`,
	}
	for _, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure sqlite compatibility schema: %w", err)
		}
	}
	columns := []struct {
		table string
		name  string
		def   string
	}{
		{table: "usage_logs", name: "image_output_tokens", def: "INTEGER NOT NULL DEFAULT 0"},
		{table: "usage_logs", name: "image_output_cost", def: "REAL NOT NULL DEFAULT 0"},
		{table: "usage_logs", name: "request_type", def: "INTEGER NOT NULL DEFAULT 0"},
		{table: "usage_logs", name: "openai_ws_mode", def: "BOOLEAN NOT NULL DEFAULT FALSE"},
		{table: "usage_logs", name: "service_tier", def: "TEXT"},
		{table: "usage_logs", name: "reasoning_effort", def: "TEXT"},
		{table: "usage_logs", name: "inbound_endpoint", def: "TEXT"},
		{table: "usage_logs", name: "upstream_endpoint", def: "TEXT"},
		{table: "usage_logs", name: "channel_id", def: "INTEGER"},
		{table: "usage_logs", name: "model_mapping_chain", def: "TEXT"},
		{table: "usage_logs", name: "billing_tier", def: "TEXT"},
		{table: "usage_logs", name: "billing_mode", def: "TEXT"},
		{table: "usage_logs", name: "account_stats_cost", def: "REAL"},
		{table: "channels", name: "model_mapping", def: "TEXT NOT NULL DEFAULT '{}'"},
		{table: "channels", name: "billing_model_source", def: "TEXT NOT NULL DEFAULT 'channel_mapped'"},
		{table: "channels", name: "restrict_models", def: "BOOLEAN NOT NULL DEFAULT FALSE"},
		{table: "channels", name: "features", def: "TEXT NOT NULL DEFAULT ''"},
		{table: "channels", name: "features_config", def: "TEXT NOT NULL DEFAULT '{}'"},
		{table: "channels", name: "apply_pricing_to_account_stats", def: "BOOLEAN NOT NULL DEFAULT FALSE"},
		{table: "channel_model_pricing", name: "platform", def: "TEXT NOT NULL DEFAULT 'anthropic'"},
		{table: "channel_model_pricing", name: "billing_mode", def: "TEXT NOT NULL DEFAULT 'token'"},
		{table: "channel_model_pricing", name: "per_request_price", def: "REAL"},
	}
	for _, col := range columns {
		if err := ensureSQLiteColumn(ctx, db, col.table, col.name, col.def); err != nil {
			return fmt.Errorf("ensure sqlite column %s.%s: %w", col.table, col.name, err)
		}
	}

	defaults := []struct {
		key   string
		value string
	}{
		{"auth_source_default_email_balance", "0"},
		{"auth_source_default_email_concurrency", "5"},
		{"auth_source_default_email_subscriptions", "[]"},
		{"auth_source_default_email_grant_on_signup", "false"},
		{"auth_source_default_email_grant_on_first_bind", "false"},
		{"auth_source_default_linuxdo_balance", "0"},
		{"auth_source_default_linuxdo_concurrency", "5"},
		{"auth_source_default_linuxdo_subscriptions", "[]"},
		{"auth_source_default_linuxdo_grant_on_signup", "false"},
		{"auth_source_default_linuxdo_grant_on_first_bind", "false"},
		{"auth_source_default_oidc_balance", "0"},
		{"auth_source_default_oidc_concurrency", "5"},
		{"auth_source_default_oidc_subscriptions", "[]"},
		{"auth_source_default_oidc_grant_on_signup", "false"},
		{"auth_source_default_oidc_grant_on_first_bind", "false"},
		{"auth_source_default_wechat_balance", "0"},
		{"auth_source_default_wechat_concurrency", "5"},
		{"auth_source_default_wechat_subscriptions", "[]"},
		{"auth_source_default_wechat_grant_on_signup", "false"},
		{"auth_source_default_wechat_grant_on_first_bind", "false"},
		{"auth_source_default_github_balance", "0"},
		{"auth_source_default_github_concurrency", "5"},
		{"auth_source_default_github_subscriptions", "[]"},
		{"auth_source_default_github_grant_on_signup", "false"},
		{"auth_source_default_github_grant_on_first_bind", "false"},
		{"auth_source_default_google_balance", "0"},
		{"auth_source_default_google_concurrency", "5"},
		{"auth_source_default_google_subscriptions", "[]"},
		{"auth_source_default_google_grant_on_signup", "false"},
		{"auth_source_default_google_grant_on_first_bind", "false"},
		{"auth_source_default_dingtalk_balance", "0"},
		{"auth_source_default_dingtalk_concurrency", "5"},
		{"auth_source_default_dingtalk_subscriptions", "[]"},
		{"auth_source_default_dingtalk_grant_on_signup", "false"},
		{"auth_source_default_dingtalk_grant_on_first_bind", "false"},
		{"force_email_on_third_party_signup", "false"},
	}
	for _, item := range defaults {
		if _, err := db.ExecContext(ctx,
			`INSERT OR IGNORE INTO settings (key, value, updated_at) VALUES (?, ?, datetime('now'))`,
			item.key,
			item.value,
		); err != nil {
			return fmt.Errorf("ensure sqlite compatibility setting %s: %w", item.key, err)
		}
	}
	return nil
}

func ensureSQLiteColumn(ctx context.Context, db *sql.DB, table, column, definition string) error {
	exists, err := sqliteColumnExists(ctx, db, table, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, column, definition))
	return err
}

func sqliteColumnExists(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid        int
			name       string
			columnType string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultVal, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func sqliteMigrationApplied(ctx context.Context, db *sql.DB, name string) (bool, error) {
	if err := ensureSQLiteMigrationsTable(ctx, db); err != nil {
		return false, err
	}
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM schema_migrations WHERE filename = ?`, name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func recordSQLiteMigration(ctx context.Context, db *sql.DB, name string) error {
	if err := ensureSQLiteMigrationsTable(ctx, db); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO schema_migrations (filename, checksum, applied_at) VALUES (?, ?, datetime('now'))`,
		name, "ent-schema-create",
	)
	return err
}

func ensureSQLiteMigrationsTable(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	filename   TEXT PRIMARY KEY,
	checksum   TEXT NOT NULL,
	applied_at TEXT NOT NULL DEFAULT (datetime('now'))
)`)
	return err
}
