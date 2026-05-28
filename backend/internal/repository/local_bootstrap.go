package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	dbapikey "github.com/Wei-Shaw/sub2api/ent/apikey"
	dbsetting "github.com/Wei-Shaw/sub2api/ent/setting"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"gopkg.in/yaml.v3"
)

var (
	defaultAPIKeyMu        sync.RWMutex
	defaultAPIKeyPlaintext string
	generatedAdminMu       sync.RWMutex
	generatedAdminPassword string
)

// GetDefaultAPIKeyPlaintext returns the auto-generated default API key (memory only).
func GetDefaultAPIKeyPlaintext() string {
	defaultAPIKeyMu.RLock()
	defer defaultAPIKeyMu.RUnlock()
	return defaultAPIKeyPlaintext
}

// SetDefaultAPIKeyPlaintext stores the default API key for desktop UI exposure.
func SetDefaultAPIKeyPlaintext(key string) {
	defaultAPIKeyMu.Lock()
	defer defaultAPIKeyMu.Unlock()
	defaultAPIKeyPlaintext = key
}

// GetGeneratedAdminPassword returns a one-time generated admin password, if any.
func GetGeneratedAdminPassword() string {
	generatedAdminMu.RLock()
	defer generatedAdminMu.RUnlock()
	return generatedAdminPassword
}

func setGeneratedAdminPassword(password string) {
	generatedAdminMu.Lock()
	defer generatedAdminMu.Unlock()
	generatedAdminPassword = password
}

func bootstrapLocalMode(ctx context.Context, client *ent.Client, cfg *config.Config) error {
	if err := upsertSetting(ctx, client, service.SettingKeyBackendModeEnabled, "true"); err != nil {
		return fmt.Errorf("enable backend mode: %w", err)
	}

	adminID, err := ensureLocalAdmin(ctx, client, cfg)
	if err != nil {
		return err
	}
	return ensureDefaultAPIKey(ctx, client, cfg, adminID)
}

func ensureLocalAdmin(ctx context.Context, client *ent.Client, cfg *config.Config) (int64, error) {
	existing, err := client.User.Query().
		Where(dbuser.RoleEQ(service.RoleAdmin)).
		First(ctx)
	if err == nil {
		return existing.ID, nil
	}
	if !ent.IsNotFound(err) {
		return 0, fmt.Errorf("query admin user: %w", err)
	}

	email := stringsTrimLocal(cfg.Local.DefaultAdminEmail)
	if email == "" {
		email = "admin@localhost"
	}
	password := stringsTrimLocal(cfg.Local.DefaultAdminPassword)
	if password == "" {
		password, err = randomHexSecret(16)
		if err != nil {
			return 0, err
		}
		setGeneratedAdminPassword(password)
		cfg.Local.DefaultAdminPassword = password
		if err := persistGeneratedLocalAdminPassword(password); err != nil {
			return 0, fmt.Errorf("persist generated admin password: %w", err)
		}
	}

	admin := &service.User{
		Email:       email,
		Role:        service.RoleAdmin,
		Status:      service.StatusActive,
		Balance:     0,
		Concurrency: simpleModeTargetAdminConcurrency,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := admin.SetPassword(password); err != nil {
		return 0, fmt.Errorf("hash admin password: %w", err)
	}

	row, err := client.User.Create().
		SetEmail(admin.Email).
		SetPasswordHash(admin.PasswordHash).
		SetRole(admin.Role).
		SetBalance(admin.Balance).
		SetConcurrency(admin.Concurrency).
		SetStatus(admin.Status).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("create admin user: %w", err)
	}
	return row.ID, nil
}

func persistGeneratedLocalAdminPassword(password string) error {
	if stringsTrimLocal(password) == "" {
		return nil
	}
	configPath := filepath.Join(localBootstrapDataDir(), "config.yaml")
	raw, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if doc == nil {
		doc = map[string]any{}
	}
	local, _ := doc["local"].(map[string]any)
	if local == nil {
		local = map[string]any{}
		doc["local"] = local
	}
	if existing, ok := local["default_admin_password"]; ok {
		switch value := existing.(type) {
		case nil:
		case string:
			if stringsTrimLocal(value) != "" {
				return nil
			}
		default:
			if stringsTrimLocal(fmt.Sprint(value)) != "" {
				return nil
			}
		}
	}
	local["default_admin_password"] = password

	updated, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, updated, 0o600)
}

func localBootstrapDataDir() string {
	if dir := os.Getenv("DATA_DIR"); stringsTrimLocal(dir) != "" {
		return dir
	}
	return "."
}

func ensureDefaultAPIKey(ctx context.Context, client *ent.Client, cfg *config.Config, adminID int64) error {
	name := stringsTrimLocal(cfg.Local.AutoAPIKeyName)
	if name == "" {
		name = "default-local-key"
	}

	count, err := client.APIKey.Query().
		Where(
			dbapikey.UserIDEQ(adminID),
			dbapikey.NameEQ(name),
		).
		Count(ctx)
	if err != nil {
		return fmt.Errorf("count default api key: %w", err)
	}
	if count > 0 {
		return nil
	}

	prefix := cfg.Default.APIKeyPrefix
	if prefix == "" {
		prefix = "sk-"
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Errorf("generate api key entropy: %w", err)
	}
	plainKey := prefix + hex.EncodeToString(raw)

	if _, err := client.APIKey.Create().
		SetUserID(adminID).
		SetKey(plainKey).
		SetName(name).
		SetStatus(service.StatusActive).
		Save(ctx); err != nil {
		return fmt.Errorf("create default api key: %w", err)
	}
	SetDefaultAPIKeyPlaintext(plainKey)
	return nil
}

func upsertSetting(ctx context.Context, client *ent.Client, key, value string) error {
	existing, err := client.Setting.Query().Where(dbsetting.KeyEQ(key)).Only(ctx)
	if ent.IsNotFound(err) {
		_, err = client.Setting.Create().SetKey(key).SetValue(value).Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	return client.Setting.UpdateOne(existing).SetValue(value).Exec(ctx)
}

func randomHexSecret(byteLen int) (string, error) {
	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func stringsTrimLocal(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
