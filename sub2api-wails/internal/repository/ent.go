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
