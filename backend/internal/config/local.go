package config

import "strings"

const (
	RunModeLocal = "local"

	DatabaseDriverPostgres = "postgres"
	DatabaseDriverSQLite   = "sqlite"

	RedisModeExternal  = "external"
	RedisModeEmbedded  = "embedded"
)

// LocalConfig holds desktop / single-user local mode options.
type LocalConfig struct {
	DefaultAdminEmail    string `mapstructure:"default_admin_email"`
	DefaultAdminPassword string `mapstructure:"default_admin_password"`
	AutoAPIKeyName       string `mapstructure:"auto_api_key_name"`
}

// IsLocalMode reports whether the app runs in desktop local mode.
func (c *Config) IsLocalMode() bool {
	return c != nil && c.RunMode == RunModeLocal
}

// IsSimpleLike reports modes that skip SaaS billing/quota checks (simple + local).
func (c *Config) IsSimpleLike() bool {
	if c == nil {
		return false
	}
	return c.RunMode == RunModeSimple || c.RunMode == RunModeLocal
}

// UsesSQLite reports whether the primary database driver is SQLite.
func (c *Config) UsesSQLite() bool {
	if c == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(c.Database.Driver), DatabaseDriverSQLite)
}

// UsesEmbeddedRedis reports whether Redis runs in-process (miniredis).
func (c *Config) UsesEmbeddedRedis() bool {
	if c == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(c.Redis.Mode), RedisModeEmbedded)
}
