//go:build integration

package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestInitEntSQLite_LocalBootstrap(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)

	cfg := &config.Config{
		RunMode:  config.RunModeLocal,
		Timezone: "UTC",
		Database: config.DatabaseConfig{
			Driver:     config.DatabaseDriverSQLite,
			SqlitePath: "test.db",
		},
		Redis: config.RedisConfig{Mode: config.RedisModeEmbedded},
		Local: config.LocalConfig{
			DefaultAdminEmail: "admin@test.local",
			AutoAPIKeyName:    "default-local-key",
		},
		Default: config.DefaultConfig{APIKeyPrefix: "sk-"},
		JWT: config.JWTConfig{
			Secret:    "01234567890123456789012345678901",
			ExpireHour: 24,
		},
		Log: config.LogConfig{
			Level:           "info",
			Format:          "console",
			StacktraceLevel: "none",
			Output:          config.LogOutputConfig{ToStdout: true},
			Rotation:        config.LogRotationConfig{MaxSizeMB: 1, MaxBackups: 1, MaxAgeDays: 1},
		},
	}

	client, db, err := InitEntSQLite(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = client.Close()
		_ = db.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	admin, err := client.User.Query().Where(dbuser.RoleEQ("admin")).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, "admin@test.local", admin.Email)

	key := GetDefaultAPIKeyPlaintext()
	require.NotEmpty(t, key)
	require.Contains(t, key, "sk-")

	_, err = os.Stat(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
}
