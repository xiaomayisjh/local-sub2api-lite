package repository

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/repository/sqldialect"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestInitEntSQLiteCompatibilitySchemaSupportsProfileIdentityTables(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	previousDriver := sqldialect.Driver()
	t.Cleanup(func() {
		sqldialect.SetDriver(previousDriver)
	})
	t.Cleanup(viper.Reset)
	viper.Reset()

	cfg, err := config.Load()
	require.NoError(t, err)

	cfg.RunMode = config.RunModeLocal
	cfg.Timezone = "UTC"
	cfg.Database.Driver = config.DatabaseDriverSQLite
	cfg.Database.SqlitePath = "compat.db"
	cfg.Redis.Mode = config.RedisModeEmbedded
	cfg.Local.DefaultAdminEmail = "admin@test.local"
	cfg.Local.AutoAPIKeyName = "default-local-key"

	client, db, err := InitEntSQLite(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
		require.NoError(t, db.Close())
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	admin, err := client.User.Query().Where(dbuser.RoleEQ(service.RoleAdmin)).Only(ctx)
	require.NoError(t, err)

	repo := newUserRepositoryWithSQL(client, SQLExecutorFromDB(db))
	avatar, err := repo.GetUserAvatar(ctx, admin.ID)
	require.NoError(t, err)
	require.Nil(t, avatar)

	avatar, err = repo.UpsertUserAvatar(ctx, admin.ID, service.UpsertUserAvatarInput{
		StorageProvider: "remote_url",
		URL:             "https://example.test/avatar.png",
		ContentType:     "image/png",
		ByteSize:        128,
		SHA256:          "abc123",
	})
	require.NoError(t, err)
	require.Equal(t, "https://example.test/avatar.png", avatar.URL)

	avatar, err = repo.GetUserAvatar(ctx, admin.ID)
	require.NoError(t, err)
	require.NotNil(t, avatar)
	require.Equal(t, "remote_url", avatar.StorageProvider)

	inserted, err := repo.RecordProviderGrant(ctx, ProviderGrantRecordInput{
		UserID:       admin.ID,
		ProviderType: "dingtalk",
		GrantReason:  ProviderGrantReasonFirstBind,
	})
	require.NoError(t, err)
	require.True(t, inserted)

	inserted, err = repo.RecordProviderGrant(ctx, ProviderGrantRecordInput{
		UserID:       admin.ID,
		ProviderType: "dingtalk",
		GrantReason:  ProviderGrantReasonFirstBind,
	})
	require.NoError(t, err)
	require.False(t, inserted)

	require.NoError(t, repo.DeleteUserAvatar(ctx, admin.ID))
	avatar, err = repo.GetUserAvatar(ctx, admin.ID)
	require.NoError(t, err)
	require.Nil(t, avatar)

	usageRepo := newUsageLogRepositoryWithSQL(client, SQLExecutorFromDB(db))
	now := time.Now().UTC()
	start := now.Add(-24 * time.Hour)
	stats, err := usageRepo.GetDashboardStatsWithRange(ctx, start, now)
	require.NoError(t, err)
	require.NotNil(t, stats)

	_, err = usageRepo.GetUsageTrendWithFilters(ctx, start, now, "day", 0, 0, 0, 0, "", nil, nil, nil)
	require.NoError(t, err)

	_, err = usageRepo.GetModelStatsWithFiltersBySource(ctx, start, now, 0, 0, 0, 0, nil, nil, nil, usagestats.ModelSourceRequested)
	require.NoError(t, err)

	_, err = usageRepo.GetGroupStatsWithFilters(ctx, start, now, 0, 0, 0, 0, nil, nil, nil)
	require.NoError(t, err)

	_, page, err := usageRepo.ListWithFilters(ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  20,
		SortBy:    "created_at",
		SortOrder: pagination.SortOrderDesc,
	}, usagestats.UsageLogFilters{
		StartTime:  &start,
		EndTime:    &now,
		ExactTotal: false,
	})
	require.NoError(t, err)
	require.NotNil(t, page)

	apiKeyStats, err := usageRepo.GetBatchAPIKeyUsageStats(ctx, []int64{1}, time.Time{}, time.Time{})
	require.NoError(t, err)
	require.Contains(t, apiKeyStats, int64(1))

	groupRates, err := NewUserGroupRateRepository(db).GetByUserID(ctx, admin.ID)
	require.NoError(t, err)
	require.Empty(t, groupRates)

	opsRepo := NewOpsRepository(db)
	filter := &service.OpsDashboardFilter{
		StartTime: start,
		EndTime:   now,
		QueryMode: service.OpsQueryModeAuto,
	}
	overview, err := opsRepo.GetDashboardOverview(ctx, filter)
	require.NoError(t, err)
	require.NotNil(t, overview)

	throughput, err := opsRepo.GetThroughputTrend(ctx, filter, 60)
	require.NoError(t, err)
	require.NotNil(t, throughput)

	errorTrend, err := opsRepo.GetErrorTrend(ctx, filter, 60)
	require.NoError(t, err)
	require.NotNil(t, errorTrend)

	distribution, err := opsRepo.GetErrorDistribution(ctx, filter)
	require.NoError(t, err)
	require.NotNil(t, distribution)

	realtime, err := opsRepo.GetRealtimeTrafficSummary(ctx, &service.OpsDashboardFilter{
		StartTime: now.Add(-time.Minute),
		EndTime:   now,
		QueryMode: service.OpsQueryModeAuto,
	})
	require.NoError(t, err)
	require.NotNil(t, realtime)

	systemLogs, err := opsRepo.ListSystemLogs(ctx, &service.OpsSystemLogFilter{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.NotNil(t, systemLogs)

	alertEvents, err := opsRepo.ListAlertEvents(ctx, &service.OpsAlertEventFilter{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, alertEvents)

	_, err = opsRepo.GetLatestSystemMetrics(ctx, 1)
	require.ErrorIs(t, err, sql.ErrNoRows)

	heartbeats, err := opsRepo.ListJobHeartbeats(ctx)
	require.NoError(t, err)
	require.Empty(t, heartbeats)

	scheduledPlanRepo := NewScheduledTestPlanRepository(db)
	duePlans, err := scheduledPlanRepo.ListDue(ctx, now)
	require.NoError(t, err)
	require.Empty(t, duePlans)

	accountPlans, err := scheduledPlanRepo.ListByAccountID(ctx, 0)
	require.NoError(t, err)
	require.Empty(t, accountPlans)

	scheduledResultRepo := NewScheduledTestResultRepository(db)
	results, err := scheduledResultRepo.ListByPlanID(ctx, 0, 10)
	require.NoError(t, err)
	require.Empty(t, results)

	accountRepo := newAccountRepositoryWithSQL(client, SQLExecutorFromDB(db), nil)
	account, err := client.Account.Create().
		SetName("sqlite-temp-unschedulable").
		SetPlatform(service.PlatformAnthropic).
		SetType(service.AccountTypeOAuth).
		SetCredentials(map[string]any{}).
		SetExtra(map[string]any{}).
		SetConcurrency(3).
		SetPriority(50).
		SetStatus(service.StatusActive).
		SetSchedulable(true).
		Save(ctx)
	require.NoError(t, err)

	until := now.Add(15 * time.Minute).UTC().Truncate(time.Second)
	reason := "oauth refresh failed"
	require.NoError(t, accountRepo.SetTempUnschedulable(ctx, account.ID, until, reason))

	updated, err := accountRepo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.TempUnschedulableUntil)
	require.WithinDuration(t, until, *updated.TempUnschedulableUntil, time.Second)
	require.Equal(t, reason, updated.TempUnschedulableReason)

	newPriority := 88
	schedulable := false
	affected, err := accountRepo.BulkUpdate(ctx, []int64{account.ID}, service.AccountBulkUpdate{
		Priority:    &newPriority,
		Schedulable: &schedulable,
		Credentials: map[string]any{"refresh_token": "sqlite-refresh"},
		Extra:       map[string]any{"bulk_note": "sqlite-ok"},
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)

	updated, err = accountRepo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, newPriority, updated.Priority)
	require.False(t, updated.Schedulable)
	require.Equal(t, "sqlite-refresh", updated.Credentials["refresh_token"])
	require.Equal(t, "sqlite-ok", updated.Extra["bulk_note"])

	lastUsed := now.Add(-time.Hour).UTC().Truncate(time.Second)
	require.NoError(t, accountRepo.BatchUpdateLastUsed(ctx, map[int64]time.Time{account.ID: lastUsed}))

	updated, err = accountRepo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.LastUsedAt)
	require.WithinDuration(t, lastUsed, *updated.LastUsedAt, time.Second)

	require.NoError(t, accountRepo.UpdateExtra(ctx, account.ID, map[string]any{"extra_patch": "sqlite-extra"}))
	updated, err = accountRepo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, "sqlite-extra", updated.Extra["extra_patch"])

	require.NoError(t, accountRepo.SetModelRateLimit(ctx, account.ID, "claude-test", now.Add(30*time.Minute)))
	updated, err = accountRepo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.Contains(t, updated.Extra, "model_rate_limits")

	require.NoError(t, accountRepo.IncrementQuotaUsed(ctx, account.ID, 2.5))
	updated, err = accountRepo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.InDelta(t, 2.5, updated.Extra["quota_used"], 0.001)

	require.NoError(t, accountRepo.ResetQuotaUsed(ctx, account.ID))
	updated, err = accountRepo.GetByID(ctx, account.ID)
	require.NoError(t, err)
	require.InDelta(t, 0, updated.Extra["quota_used"], 0.001)

	group, err := client.Group.Create().
		SetName("sqlite-channel-group").
		SetPlatform(service.PlatformAnthropic).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	channelRepo := NewChannelRepository(db)
	maxTokens := 1000
	inputPrice := 0.000001
	outputPrice := 0.000002
	requestPrice := 0.05
	channel := &service.Channel{
		Name:                       "sqlite-channel",
		Description:                "SQLite channel compatibility",
		Status:                     service.StatusActive,
		BillingModelSource:         service.BillingModelSourceChannelMapped,
		RestrictModels:             true,
		Features:                   `["local"]`,
		FeaturesConfig:             map[string]any{"web_search_emulation": map[string]any{service.PlatformAnthropic: true}},
		ApplyPricingToAccountStats: true,
		GroupIDs:                   []int64{group.ID},
		ModelMapping: map[string]map[string]string{
			service.PlatformAnthropic: {"claude-test": "claude-upstream"},
		},
		ModelPricing: []service.ChannelModelPricing{
			{
				Platform:    service.PlatformAnthropic,
				Models:      []string{"claude-test"},
				BillingMode: service.BillingModeToken,
				InputPrice:  &inputPrice,
				OutputPrice: &outputPrice,
				Intervals: []service.PricingInterval{
					{
						MinTokens:   0,
						MaxTokens:   &maxTokens,
						InputPrice:  &inputPrice,
						OutputPrice: &outputPrice,
						SortOrder:   1,
					},
				},
			},
		},
		AccountStatsPricingRules: []service.AccountStatsPricingRule{
			{
				Name:      "default",
				GroupIDs:  []int64{group.ID},
				SortOrder: 1,
				Pricing: []service.ChannelModelPricing{
					{
						Platform:        service.PlatformAnthropic,
						Models:          []string{"claude-test"},
						BillingMode:     service.BillingModePerRequest,
						PerRequestPrice: &requestPrice,
						Intervals: []service.PricingInterval{
							{
								TierLabel:       "1K",
								PerRequestPrice: &requestPrice,
								SortOrder:       1,
							},
						},
					},
				},
			},
		},
	}
	require.NoError(t, channelRepo.Create(ctx, channel))
	require.NotZero(t, channel.ID)

	loadedChannel, err := channelRepo.GetByID(ctx, channel.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{group.ID}, loadedChannel.GroupIDs)
	require.Equal(t, service.BillingModelSourceChannelMapped, loadedChannel.BillingModelSource)
	require.True(t, loadedChannel.RestrictModels)
	require.Equal(t, "claude-upstream", loadedChannel.ModelMapping[service.PlatformAnthropic]["claude-test"])
	require.Len(t, loadedChannel.ModelPricing, 1)
	require.Equal(t, service.BillingModeToken, loadedChannel.ModelPricing[0].BillingMode)
	require.Equal(t, []string{"claude-test"}, loadedChannel.ModelPricing[0].Models)
	require.Len(t, loadedChannel.ModelPricing[0].Intervals, 1)
	require.Equal(t, maxTokens, *loadedChannel.ModelPricing[0].Intervals[0].MaxTokens)
	require.Len(t, loadedChannel.AccountStatsPricingRules, 1)
	require.Equal(t, []int64{group.ID}, loadedChannel.AccountStatsPricingRules[0].GroupIDs)
	require.Len(t, loadedChannel.AccountStatsPricingRules[0].Pricing, 1)
	require.Equal(t, service.BillingModePerRequest, loadedChannel.AccountStatsPricingRules[0].Pricing[0].BillingMode)
	require.Len(t, loadedChannel.AccountStatsPricingRules[0].Pricing[0].Intervals, 1)

	channels, page, err := channelRepo.List(ctx, pagination.PaginationParams{Page: 1, PageSize: 10}, service.StatusActive, "sqlite")
	require.NoError(t, err)
	require.NotNil(t, page)
	require.Len(t, channels, 1)
	require.Equal(t, channel.ID, channels[0].ID)

	allChannels, err := channelRepo.ListAll(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, allChannels)

	groupPlatforms, err := channelRepo.GetGroupPlatforms(ctx, []int64{group.ID})
	require.NoError(t, err)
	require.Equal(t, service.PlatformAnthropic, groupPlatforms[group.ID])

	conflicting, err := channelRepo.GetGroupsInOtherChannels(ctx, 0, []int64{group.ID})
	require.NoError(t, err)
	require.Equal(t, []int64{group.ID}, conflicting)

	conflicting, err = channelRepo.GetGroupsInOtherChannels(ctx, channel.ID, []int64{group.ID})
	require.NoError(t, err)
	require.Empty(t, conflicting)
}

func TestInitEntSQLitePersistsGeneratedLocalAdminPassword(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DATA_DIR", dir)
	t.Setenv("JWT_SECRET", "01234567890123456789012345678901")
	setGeneratedAdminPassword("")
	t.Cleanup(func() {
		setGeneratedAdminPassword("")
	})
	previousDriver := sqldialect.Driver()
	t.Cleanup(func() {
		sqldialect.SetDriver(previousDriver)
	})
	t.Cleanup(viper.Reset)
	viper.Reset()

	configPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
run_mode: local
server:
  host: 127.0.0.1
  port: 8080
database:
  driver: sqlite
  sqlite_path: bootstrap.db
redis:
  mode: embedded
local:
  default_admin_email: admin@localhost
  default_admin_password: ""
  auto_api_key_name: default-local-key
jwt:
  secret: 01234567890123456789012345678901
  expire_hour: 24
totp:
  encryption_key: 0123456789012345678901234567890123456789012345678901234567890123
`), 0o600))

	cfg, err := config.Load()
	require.NoError(t, err)

	client, db, err := InitEntSQLite(cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
		require.NoError(t, db.Close())
	})

	password := GetGeneratedAdminPassword()
	require.NotEmpty(t, password)

	raw, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	local, _ := doc["local"].(map[string]any)
	require.Equal(t, password, local["default_admin_password"])
}
