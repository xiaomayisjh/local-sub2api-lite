package admin

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

// ConfigureAdminCaches adjusts cache TTLs based on the run mode.
// Local mode is a single-user desktop app, so aggressive caching only hides recent activity
// from the user. We shrink TTL to ~2s in local mode (still useful for collapsing burst calls
// like multiple dashboard widgets rendering at the same time) and keep production defaults
// otherwise.
func ConfigureAdminCaches(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if cfg.RunMode != config.RunModeLocal {
		return
	}

	short := 2 * time.Second
	dashboardSnapshotV2Cache.SetTTL(short)
	accountTodayStatsBatchCache.SetTTL(short)
	dashboardTrendCache.SetTTL(short)
	dashboardModelStatsCache.SetTTL(short)
	dashboardGroupStatsCache.SetTTL(short)
	dashboardUsersTrendCache.SetTTL(short)
	dashboardAPIKeysTrendCache.SetTTL(short)
	dashboardBatchUsersUsageCache.SetTTL(short)
	dashboardBatchAPIKeysUsageCache.SetTTL(short)
	// users ranking only changes when usage_logs grow; keep it slightly longer to avoid
	// blocking the dashboard ranking widget on heavy queries.
	dashboardUsersRankingCache.SetTTL(10 * time.Second)
}

// invalidateUserFacingMetricCaches drops every cache the dashboard reads from.
// Triggered after the user performs an action that should reflect immediately
// (e.g. clicking refresh in local mode).
func invalidateUserFacingMetricCaches() {
	dashboardSnapshotV2Cache.Invalidate("")
	accountTodayStatsBatchCache.Invalidate("")
	dashboardTrendCache.Invalidate("")
	dashboardModelStatsCache.Invalidate("")
	dashboardGroupStatsCache.Invalidate("")
	dashboardUsersTrendCache.Invalidate("")
	dashboardAPIKeysTrendCache.Invalidate("")
	dashboardBatchUsersUsageCache.Invalidate("")
	dashboardBatchAPIKeysUsageCache.Invalidate("")
	dashboardUsersRankingCache.Invalidate("")
}
