// Package repository 提供应用程序的基础设施层组件。
// 包括数据库连接初始化、ORM 客户端管理、Redis 连接、数据库迁移等核心功能。
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"sub2api-wails/ent"
	"sub2api-wails/internal/config"
	"sub2api-wails/internal/pkg/timezone"
	"sub2api-wails/migrations"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

// InitEnt 初始化 Ent ORM 客户端并返回客户端实例和底层的 *sql.DB。
//
// 该函数执行以下操作：
//  1. 初始化全局时区设置，确保时间处理一致性
//  2. 建立 SQLite 数据库连接
//  3. 配置 SQLite PRAGMA（WAL 模式、外键约束）
//  4. 自动执行数据库迁移，确保 schema 与代码同步
//  5. 创建并返回 Ent 客户端实例
//
// 重要提示：调用者必须负责关闭返回的 ent.Client（关闭时会自动关闭底层的 driver/db）。
//
// 参数：
//   - cfg: 应用程序配置，包含数据库连接信息和时区设置
//
// 返回：
//   - *ent.Client: Ent ORM 客户端，用于执行数据库操作
//   - *sql.DB: 底层的 SQL 数据库连接，可用于直接执行原生 SQL
//   - error: 初始化过程中的错误
func InitEnt(cfg *config.Config) (*ent.Client, *sql.DB, error) {
	if err := timezone.Init(cfg.Timezone); err != nil {
		return nil, nil, err
	}

	dsn := cfg.Database.DSN()

	drv, err := entsql.Open(dialect.SQLite, dsn)
	if err != nil {
		return nil, nil, err
	}

	db := drv.DB()
	applyDBPoolSettings(db, cfg)

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = drv.Close()
		return nil, nil, fmt.Errorf("set sqlite WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = drv.Close()
		return nil, nil, fmt.Errorf("enable sqlite foreign keys: %w", err)
	}

	migrationCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := applyMigrationsFS(migrationCtx, db, migrations.FS); err != nil {
		_ = drv.Close()
		return nil, nil, err
	}

	client := ent.NewClient(ent.Driver(drv))

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

	return client, db, nil
}
