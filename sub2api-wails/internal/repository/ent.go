package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"sub2api-wails/ent"
	"sub2api-wails/ent/migrate"
	_ "sub2api-wails/ent/runtime"
	"sub2api-wails/ent/user"
	"sub2api-wails/internal/config"
	"sub2api-wails/internal/pkg/timezone"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

func InitEnt(cfg *config.Config) (*ent.Client, *sql.DB, error) {
	if err := timezone.Init(cfg.Timezone); err != nil {
		return nil, nil, err
	}

	dsn := cfg.Database.DSN()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, err
	}

	applyDBPoolSettings(db, cfg)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("set sqlite WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))

	migrationCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if err := client.Schema.Create(migrationCtx, migrate.WithForeignKeys(true)); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("auto-migrate schema: %w", err)
	}

	if err := ensureBootstrapSecrets(migrationCtx, client, cfg); err != nil {
		_ = client.Close()
		return nil, nil, err
	}

	if err := cfg.Validate(); err != nil {
		_ = client.Close()
		return nil, nil, fmt.Errorf("validate config after secret bootstrap: %w", err)
	}

	if cfg.RunMode == config.RunModeSimple {
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

	if err := ensureDefaultAdmin(migrationCtx, client, cfg); err != nil {
		_ = client.Close()
		return nil, nil, err
	}

	if err := ensureSQLiteExtraTables(migrationCtx, db); err != nil {
		_ = client.Close()
		return nil, nil, err
	}

	return client, db, nil
}

func ensureDefaultAdmin(ctx context.Context, client *ent.Client, cfg *config.Config) error {
	count, err := client.User.Query().Where(user.RoleEQ("admin")).Count(ctx)
	if err != nil {
		return fmt.Errorf("check admin user: %w", err)
	}
	if count > 0 {
		return nil
	}

	email := cfg.Default.AdminEmail
	password := cfg.Default.AdminPassword
	if email == "" {
		email = "admin@sub2api.local"
	}
	if password == "" {
		password = "admin123"
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	_, err = client.User.Create().
		SetEmail(email).
		SetPasswordHash(string(hashedBytes)).
		SetRole("admin").
		SetStatus("active").
		SetUsername("admin").
		SetBalance(0).
		SetConcurrency(10).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}

	log.Printf("Default admin user created: %s", email)
	return nil
}

func ensureSQLiteExtraTables(ctx context.Context, db *sql.DB) error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS user_avatars (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			storage_provider TEXT NOT NULL DEFAULT 'database',
			storage_key TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			content_type TEXT NOT NULL DEFAULT '',
			byte_size INTEGER NOT NULL DEFAULT 0,
			sha256 TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS user_avatars_user_id_key ON user_avatars (user_id)`,

		`CREATE TABLE IF NOT EXISTS channels (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_channels_name ON channels (name)`,
		`CREATE INDEX IF NOT EXISTS idx_channels_status ON channels (status)`,

		`CREATE TABLE IF NOT EXISTS channel_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
			group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_channel_groups_group_id ON channel_groups (group_id)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_groups_channel_id ON channel_groups (channel_id)`,

		`CREATE TABLE IF NOT EXISTS channel_model_pricing (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
			models TEXT NOT NULL DEFAULT '[]',
			input_price REAL,
			output_price REAL,
			cache_write_price REAL,
			cache_read_price REAL,
			image_output_price REAL,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_channel_model_pricing_channel_id ON channel_model_pricing (channel_id)`,

		`CREATE TABLE IF NOT EXISTS user_affiliates (
			user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			aff_code TEXT NOT NULL UNIQUE,
			inviter_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
			aff_count INTEGER NOT NULL DEFAULT 0,
			aff_quota REAL NOT NULL DEFAULT 0,
			aff_history_quota REAL NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_user_affiliates_inviter_id ON user_affiliates(inviter_id)`,

		`CREATE TABLE IF NOT EXISTS user_group_rates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			rate REAL NOT NULL DEFAULT 1.0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
			UNIQUE(user_id, group_id)
		)`,

		`CREATE TABLE IF NOT EXISTS scheduled_test_plans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
			model_id TEXT NOT NULL DEFAULT '',
			cron_expression TEXT NOT NULL DEFAULT '*/30 * * * *',
			enabled INTEGER NOT NULL DEFAULT 1,
			max_results INTEGER NOT NULL DEFAULT 50,
			last_run_at DATETIME,
			next_run_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stp_account_id ON scheduled_test_plans(account_id)`,

		`CREATE TABLE IF NOT EXISTS scheduled_test_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plan_id INTEGER NOT NULL REFERENCES scheduled_test_plans(id) ON DELETE CASCADE,
			status TEXT NOT NULL DEFAULT 'success',
			response_text TEXT NOT NULL DEFAULT '',
			error_message TEXT NOT NULL DEFAULT '',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			started_at DATETIME NOT NULL DEFAULT (datetime('now')),
			finished_at DATETIME NOT NULL DEFAULT (datetime('now')),
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_str_plan_created ON scheduled_test_results(plan_id, created_at DESC)`,

		`CREATE TABLE IF NOT EXISTS usage_billing_dedup (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL,
			api_key_id INTEGER NOT NULL,
			request_fingerprint TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_usage_billing_dedup_request_api_key ON usage_billing_dedup (request_id, api_key_id)`,

		`CREATE TABLE IF NOT EXISTS usage_billing_dedup_archive (
			request_id TEXT NOT NULL,
			api_key_id INTEGER NOT NULL,
			request_fingerprint TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			archived_at DATETIME NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (request_id, api_key_id)
		)`,

		`CREATE TABLE IF NOT EXISTS content_moderation_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT NOT NULL DEFAULT '',
			user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
			user_email TEXT NOT NULL DEFAULT '',
			api_key_id INTEGER REFERENCES api_keys(id) ON DELETE SET NULL,
			api_key_name TEXT NOT NULL DEFAULT '',
			group_id INTEGER REFERENCES groups(id) ON DELETE SET NULL,
			group_name TEXT NOT NULL DEFAULT '',
			endpoint TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			mode TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL DEFAULT '',
			flagged INTEGER NOT NULL DEFAULT 0,
			highest_category TEXT NOT NULL DEFAULT '',
			highest_score REAL NOT NULL DEFAULT 0,
			category_scores TEXT NOT NULL DEFAULT '{}',
			threshold_snapshot TEXT NOT NULL DEFAULT '{}',
			input_excerpt TEXT NOT NULL DEFAULT '',
			upstream_latency_ms INTEGER,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS content_moderation_cache (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cache_key TEXT NOT NULL UNIQUE,
			result TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			expires_at DATETIME
		)`,

		`CREATE TABLE IF NOT EXISTS ops_metrics_hourly (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket_start DATETIME NOT NULL,
			platform TEXT,
			group_id INTEGER,
			success_count INTEGER NOT NULL DEFAULT 0,
			error_count_total INTEGER NOT NULL DEFAULT 0,
			business_limited_count INTEGER NOT NULL DEFAULT 0,
			error_count_sla INTEGER NOT NULL DEFAULT 0,
			upstream_error_count_excl_429_529 INTEGER NOT NULL DEFAULT 0,
			upstream_429_count INTEGER NOT NULL DEFAULT 0,
			upstream_529_count INTEGER NOT NULL DEFAULT 0,
			token_consumed INTEGER NOT NULL DEFAULT 0,
			duration_p50_ms INTEGER,
			duration_p90_ms INTEGER,
			duration_p95_ms INTEGER,
			duration_p99_ms INTEGER,
			duration_max_ms INTEGER,
			request_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_metrics_hourly_bucket ON ops_metrics_hourly (bucket_start, platform, group_id)`,

		`CREATE TABLE IF NOT EXISTS ops_metrics_daily (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket_date TEXT NOT NULL,
			platform TEXT,
			group_id INTEGER,
			success_count INTEGER NOT NULL DEFAULT 0,
			error_count_total INTEGER NOT NULL DEFAULT 0,
			business_limited_count INTEGER NOT NULL DEFAULT 0,
			error_count_sla INTEGER NOT NULL DEFAULT 0,
			upstream_error_count_excl_429_529 INTEGER NOT NULL DEFAULT 0,
			upstream_429_count INTEGER NOT NULL DEFAULT 0,
			upstream_529_count INTEGER NOT NULL DEFAULT 0,
			token_consumed INTEGER NOT NULL DEFAULT 0,
			duration_p50_ms INTEGER,
			duration_p90_ms INTEGER,
			duration_p95_ms INTEGER,
			duration_p99_ms INTEGER,
			duration_max_ms INTEGER,
			request_count INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_ops_metrics_daily_bucket ON ops_metrics_daily (bucket_date, platform, group_id)`,

		`CREATE TABLE IF NOT EXISTS ops_error_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			platform TEXT NOT NULL,
			group_id INTEGER,
			error_code INTEGER NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT '',
			request_id TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS ops_alert_rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			severity TEXT NOT NULL DEFAULT 'warning',
			metric_type TEXT NOT NULL,
			operator TEXT NOT NULL,
			threshold REAL NOT NULL,
			window_minutes INTEGER NOT NULL DEFAULT 5,
			sustained_minutes INTEGER NOT NULL DEFAULT 5,
			cooldown_minutes INTEGER NOT NULL DEFAULT 10,
			filters TEXT,
			notify_channels TEXT NOT NULL DEFAULT '[]',
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS ops_alert_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id INTEGER,
			severity TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'firing',
			title TEXT,
			description TEXT,
			metric_value REAL,
			threshold_value REAL,
			dimensions TEXT,
			fired_at DATETIME NOT NULL DEFAULT (datetime('now')),
			resolved_at DATETIME,
			email_sent INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS ops_alert_silences (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			rule_id INTEGER NOT NULL,
			platform TEXT NOT NULL,
			group_id INTEGER,
			region TEXT,
			until DATETIME NOT NULL,
			reason TEXT,
			created_by INTEGER,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS scheduler_outbox (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			event_type TEXT NOT NULL,
			account_id INTEGER,
			group_id INTEGER,
			payload TEXT,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_scheduler_outbox_created_at ON scheduler_outbox (created_at)`,

		`CREATE TABLE IF NOT EXISTS usage_dashboard_hourly (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket_start DATETIME NOT NULL,
			group_id INTEGER,
			request_count INTEGER NOT NULL DEFAULT 0,
			success_count INTEGER NOT NULL DEFAULT 0,
			error_count INTEGER NOT NULL DEFAULT 0,
			token_consumed INTEGER NOT NULL DEFAULT 0,
			estimated_cost REAL NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_udh_bucket ON usage_dashboard_hourly (bucket_start, group_id)`,

		`CREATE TABLE IF NOT EXISTS usage_dashboard_hourly_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket_start DATETIME NOT NULL,
			group_id INTEGER,
			user_id INTEGER NOT NULL,
			request_count INTEGER NOT NULL DEFAULT 0,
			token_consumed INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_udhu_bucket_user ON usage_dashboard_hourly_users (bucket_start, group_id, user_id)`,

		`CREATE TABLE IF NOT EXISTS usage_dashboard_daily (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket_date TEXT NOT NULL,
			group_id INTEGER,
			request_count INTEGER NOT NULL DEFAULT 0,
			success_count INTEGER NOT NULL DEFAULT 0,
			error_count INTEGER NOT NULL DEFAULT 0,
			token_consumed INTEGER NOT NULL DEFAULT 0,
			estimated_cost REAL NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_udd_bucket ON usage_dashboard_daily (bucket_date, group_id)`,

		`CREATE TABLE IF NOT EXISTS usage_dashboard_daily_users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bucket_date TEXT NOT NULL,
			group_id INTEGER,
			user_id INTEGER NOT NULL,
			request_count INTEGER NOT NULL DEFAULT 0,
			token_consumed INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_uddu_bucket_user ON usage_dashboard_daily_users (bucket_date, group_id, user_id)`,
	}

	for _, ddl := range tables {
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("create extra table: %w\nDDL: %s", err, ddl)
		}
	}

	log.Printf("SQLite extra tables created successfully")
	return nil
}
