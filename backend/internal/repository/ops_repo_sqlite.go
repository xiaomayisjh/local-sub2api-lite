package repository

import (
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func emptyOpsDashboardOverview(filter *service.OpsDashboardFilter) *service.OpsDashboardOverview {
	start, end := opsFilterTimes(filter)
	return &service.OpsDashboardOverview{
		StartTime: start,
		EndTime:   end,
		Platform:  strings.TrimSpace(filter.Platform),
		GroupID:   filter.GroupID,
		SLA:       1,
		QPS:       service.OpsRateSummary{},
		TPS:       service.OpsRateSummary{},
		Duration:  service.OpsPercentiles{},
		TTFT:      service.OpsPercentiles{},
	}
}

func emptyOpsThroughputTrend(filter *service.OpsDashboardFilter, bucketSeconds int) *service.OpsThroughputTrendResponse {
	start, end := opsFilterTimes(filter)
	return &service.OpsThroughputTrendResponse{
		Bucket:     opsBucketLabel(bucketSeconds),
		Points:     fillOpsThroughputBuckets(start, end, bucketSeconds, nil),
		ByPlatform: []*service.OpsThroughputPlatformBreakdownItem{},
		TopGroups:  []*service.OpsThroughputGroupBreakdownItem{},
	}
}

func emptyOpsErrorTrend(filter *service.OpsDashboardFilter, bucketSeconds int) *service.OpsErrorTrendResponse {
	start, end := opsFilterTimes(filter)
	return &service.OpsErrorTrendResponse{
		Bucket: opsBucketLabel(bucketSeconds),
		Points: fillOpsErrorTrendBuckets(start, end, bucketSeconds, nil),
	}
}

func emptyOpsErrorDistribution() *service.OpsErrorDistributionResponse {
	return &service.OpsErrorDistributionResponse{
		Total: 0,
		Items: []*service.OpsErrorDistributionItem{},
	}
}

func emptyOpsRealtimeTrafficSummary(filter *service.OpsDashboardFilter) *service.OpsRealtimeTrafficSummary {
	start, end := opsFilterTimes(filter)
	return &service.OpsRealtimeTrafficSummary{
		StartTime: start,
		EndTime:   end,
		Platform:  strings.TrimSpace(filter.Platform),
		GroupID:   filter.GroupID,
		QPS:       service.OpsRateSummary{},
		TPS:       service.OpsRateSummary{},
	}
}

func emptyOpsSystemLogList(page, pageSize int) *service.OpsSystemLogList {
	return &service.OpsSystemLogList{
		Logs:     []*service.OpsSystemLog{},
		Total:    0,
		Page:     page,
		PageSize: pageSize,
	}
}

func emptyOpsErrorLogList(page, pageSize int) *service.OpsErrorLogList {
	return &service.OpsErrorLogList{
		Errors:   []*service.OpsErrorLog{},
		Total:    0,
		Page:     page,
		PageSize: pageSize,
	}
}

func opsFilterTimes(filter *service.OpsDashboardFilter) (time.Time, time.Time) {
	if filter == nil {
		now := time.Now().UTC()
		return now, now
	}
	return filter.StartTime.UTC(), filter.EndTime.UTC()
}
