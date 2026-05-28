package localconfig

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/setup"
	"gopkg.in/yaml.v3"
)

// DefaultDataDir returns the directory beside the running executable (portable desktop layout).
// config.yaml and sub2api.db are stored next to the exe unless DATA_DIR is set explicitly.
func DefaultDataDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("DATA_DIR")); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
		return dir, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// EnsureDesktopEnvironment sets DATA_DIR and creates config + install lock on first run.
func EnsureDesktopEnvironment() (string, error) {
	dataDir, err := DefaultDataDir()
	if err != nil {
		return "", err
	}
	if err := os.Setenv("DATA_DIR", dataDir); err != nil {
		return "", err
	}

	configPath := filepath.Join(dataDir, setup.ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		secret, err := randomHex(32)
		if err != nil {
			return "", err
		}
		totpSecret, err := randomHex(32)
		if err != nil {
			return "", err
		}
		doc := map[string]any{
			"run_mode": "local",
			"server": map[string]any{
				"host": "127.0.0.1",
				"port": 8080,
				"mode": "release",
			},
			"database": map[string]any{
				"driver":      "sqlite",
				"sqlite_path": "sub2api.db",
			},
			"redis": map[string]any{
				"mode": "embedded",
			},
			"local": map[string]any{
				"default_admin_email":    "admin@localhost",
				"default_admin_password": "",
				"auto_api_key_name":      "default-local-key",
			},
			"jwt": map[string]any{
				"secret":      secret,
				"expire_hour": 24,
			},
			"totp": map[string]any{
				"encryption_key": totpSecret,
			},
			"default": map[string]any{
				"user_concurrency": 30,
				"user_balance":     0,
				"api_key_prefix":   "sk-",
				"rate_multiplier":  1.0,
			},
			"timezone": "Asia/Shanghai",
			"log": map[string]any{
				"level":            "info",
				"format":           "console",
				"stacktrace_level": "error",
				"output": map[string]any{
					"to_stdout": true,
					"to_file":   false,
				},
				"rotation": map[string]any{
					"max_size_mb":  100,
					"max_backups":  3,
					"max_age_days": 7,
					"compress":     true,
				},
				"sampling": map[string]any{
					"enabled":    false,
					"initial":    100,
					"thereafter": 100,
				},
			},
			"cors": map[string]any{
				"allowed_origins": []string{
					"http://127.0.0.1:*",
					"http://localhost:*",
				},
			},
		}
		raw, err := yaml.Marshal(doc)
		if err != nil {
			return "", err
		}
		if err := os.WriteFile(configPath, raw, 0o600); err != nil {
			return "", err
		}
	}

	lockPath := filepath.Join(dataDir, setup.InstallLockFile)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		if err := os.WriteFile(lockPath, []byte("desktop\n"), 0o644); err != nil {
			return "", err
		}
	}
	return dataDir, nil
}

func randomHex(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
