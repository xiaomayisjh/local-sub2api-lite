//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository/sqldialect"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestSQLiteUsageTrendQueries_DateBucketingWithGoTimeFormat 验证 SQLite 下所有
// usage trend 查询能正确处理 ent 写入的 Go time.String() 默认时间格式。
//
// 历史 bug：ent 在 SQLite 上把 time.Time 用 Go 默认 Stringer 写入，得到
// "2026-05-31 09:02:57.463304 +0800 CST m=+474.119079101"。SQLite 的 strftime
// 解析不了尾部 "+0800 CST m=+..."，返回 NULL，导致 Scan 到 string 类型字段时
// 报 "converting NULL to string is unsupported" → API 500。
//
// 修复：usageLogDateGroupExpr 在 SQLite 路径用 substr(col, 1, 19) 截断时间字符串。
// 这个测试是回归网：所有 trend 查询路径都跑 hour 粒度，确保 SQL 不再报错且 date 非空。
func TestSQLiteUsageTrendQueries_DateBucketingWithGoTimeFormat(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")

	previousDriver := sqldialect.Driver()
	t.Cleanup(func() { sqldialect.SetDriver(previousDriver) })
	t.Cleanup(viper.Reset)
	viper.Reset()

	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.RunMode = config.RunModeLocal
	cfg.Timezone = "UTC"
	cfg.Database.Driver = config.DatabaseDriverSQLite
	cfg.Database.SqlitePath = "trend_test.db"
	cfg.Redis.Mode = config.RedisModeEmbedded
	cfg.Local.DefaultAdminEmail = "trend-admin@test.local"
	cfg.Local.AutoAPIKeyName = "default-local-key"

	client, db, err := InitEntSQLite(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
		require.NoError(t, db.Close())
	})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	admin, err := client.User.Query().Where(dbuser.RoleEQ(service.RoleAdmin)).Only(ctx)
	require.NoError(t, err)

	// 创建一个 API key 给 admin（usage_log 有 FK 到 api_keys）
	now := time.Now().UTC()
	keyRepo := newAPIKeyRepositoryWithSQL(client, SQLExecutorFromDB(db))
	apiKey := &service.APIKey{
		UserID: admin.ID,
		Key:    "sk-trend-test-key",
		Name:   "trend-test",
		Status: service.StatusActive,
	}
	require.NoError(t, keyRepo.Create(ctx, apiKey))

	// 创建一个 account（usage_log 也有 FK 到 accounts）
	accountRepo := newAccountRepositoryWithSQL(client, SQLExecutorFromDB(db), nil)
	account := &service.Account{
		Name:        "trend-test-account",
		Platform:    service.PlatformAnthropic,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Concurrency: 3,
		Priority:    50,
		Credentials: map[string]any{"api_key": "test-key"},
		Extra:       map[string]any{},
	}
	require.NoError(t, accountRepo.Create(ctx, account))

	// 写 5 条 usage_log，时间散布在最近 3 小时内（覆盖 hour 粒度的多个 bucket）
	usageRepo := newUsageLogRepositoryWithSQL(client, SQLExecutorFromDB(db))
	for i := 0; i < 5; i++ {
		logTime := now.Add(-time.Duration(i) * 30 * time.Minute)
		log := &service.UsageLog{
			UserID:       admin.ID,
			APIKeyID:     apiKey.ID,
			AccountID:    account.ID,
			RequestID:    "req-trend-test-" + time.Now().Format("150405.000000000") + "-" + string(rune('0'+i)),
			Model:        "claude-sonnet-4",
			InputTokens:  100,
			OutputTokens: 50,
			TotalCost:    0.01,
			ActualCost:   0.01,
			CreatedAt:    logTime,
		}
		_, err = usageRepo.Create(ctx, log)
		require.NoError(t, err, "insert usage_log #%d", i)
	}

	// 关键断言：跑所有 trend / aggregation 查询并验证不报错 + 返回非空数据
	start := now.Add(-24 * time.Hour)
	end := now.Add(time.Hour)

	t.Run("GetUsageTrendWithFilters_hour", func(t *testing.T) {
		results, err := usageRepo.GetUsageTrendWithFilters(ctx, start, end, "hour", 0, 0, 0, 0, "", nil, nil, nil)
		require.NoError(t, err, "hour granularity must not error after substr fix")
		require.NotEmpty(t, results, "should have aggregated rows from inserted data")
		for _, r := range results {
			require.NotEmpty(t, r.Date, "Date column must not be NULL/empty under SQLite")
		}
	})

	t.Run("GetUsageTrendWithFilters_day", func(t *testing.T) {
		results, err := usageRepo.GetUsageTrendWithFilters(ctx, start, end, "day", 0, 0, 0, 0, "", nil, nil, nil)
		require.NoError(t, err)
		require.NotEmpty(t, results)
		for _, r := range results {
			require.NotEmpty(t, r.Date)
		}
	})

	t.Run("GetUserUsageTrend_hour", func(t *testing.T) {
		results, err := usageRepo.GetUserUsageTrend(ctx, start, end, "hour", 12)
		require.NoError(t, err, "users-trend hour granularity must not error")
		require.NotEmpty(t, results)
		for _, r := range results {
			require.NotEmpty(t, r.Date)
		}
	})

	t.Run("GetAPIKeyUsageTrend_hour", func(t *testing.T) {
		results, err := usageRepo.GetAPIKeyUsageTrend(ctx, start, end, "hour", 5)
		require.NoError(t, err, "api-keys-trend hour granularity must not error")
		require.NotEmpty(t, results)
		for _, r := range results {
			require.NotEmpty(t, r.Date)
		}
	})

	t.Run("GetDailyStatsAggregated", func(t *testing.T) {
		results, err := usageRepo.GetDailyStatsAggregated(ctx, admin.ID, start, end)
		require.NoError(t, err)
		require.NotEmpty(t, results)
		// date 字段也是 string scan，需要确保非空
		for _, r := range results {
			date, _ := r["date"].(string)
			require.NotEmpty(t, date, "date must not be empty in daily stats")
		}
	})

	t.Run("GetAccountUsageStats", func(t *testing.T) {
		// 这个查询也走 usageLogDateGroupExpr (day 粒度) → 同样依赖 substr fix
		resp, err := usageRepo.GetAccountUsageStats(ctx, account.ID, start, end)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.NotEmpty(t, resp.History, "account usage history must have rows")
		for _, h := range resp.History {
			require.NotEmpty(t, h.Date, "date must not be empty in account usage history")
		}
	})
}
