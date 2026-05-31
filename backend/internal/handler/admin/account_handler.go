// Package admin provides HTTP handlers for administrative operations.
package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

// OAuthHandler handles OAuth-related operations for accounts
type OAuthHandler struct {
	oauthService *service.OAuthService
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(oauthService *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{
		oauthService: oauthService,
	}
}

// AccountHandler handles admin account management
type AccountHandler struct {
	adminService            service.AdminService
	oauthService            *service.OAuthService
	openaiOAuthService      *service.OpenAIOAuthService
	geminiOAuthService      *service.GeminiOAuthService
	antigravityOAuthService *service.AntigravityOAuthService
	rateLimitService        *service.RateLimitService
	accountUsageService     *service.AccountUsageService
	accountTestService      *service.AccountTestService
	concurrencyService      *service.ConcurrencyService
	crsSyncService          *service.CRSSyncService
	sessionLimitCache       service.SessionLimitCache
	rpmCache                service.RPMCache
	tokenCacheInvalidator   service.TokenCacheInvalidator
}

// NewAccountHandler creates a new admin account handler
func NewAccountHandler(
	adminService service.AdminService,
	oauthService *service.OAuthService,
	openaiOAuthService *service.OpenAIOAuthService,
	geminiOAuthService *service.GeminiOAuthService,
	antigravityOAuthService *service.AntigravityOAuthService,
	rateLimitService *service.RateLimitService,
	accountUsageService *service.AccountUsageService,
	accountTestService *service.AccountTestService,
	concurrencyService *service.ConcurrencyService,
	crsSyncService *service.CRSSyncService,
	sessionLimitCache service.SessionLimitCache,
	rpmCache service.RPMCache,
	tokenCacheInvalidator service.TokenCacheInvalidator,
) *AccountHandler {
	return &AccountHandler{
		adminService:            adminService,
		oauthService:            oauthService,
		openaiOAuthService:      openaiOAuthService,
		geminiOAuthService:      geminiOAuthService,
		antigravityOAuthService: antigravityOAuthService,
		rateLimitService:        rateLimitService,
		accountUsageService:     accountUsageService,
		accountTestService:      accountTestService,
		concurrencyService:      concurrencyService,
		crsSyncService:          crsSyncService,
		sessionLimitCache:       sessionLimitCache,
		rpmCache:                rpmCache,
		tokenCacheInvalidator:   tokenCacheInvalidator,
	}
}

// CreateAccountRequest represents create account request
type CreateAccountRequest struct {
	Name                    string         `json:"name" binding:"required"`
	Notes                   *string        `json:"notes"`
	Platform                string         `json:"platform" binding:"required"`
	Type                    string         `json:"type" binding:"required,oneof=oauth setup-token apikey upstream bedrock service_account"`
	Credentials             map[string]any `json:"credentials" binding:"required"`
	Extra                   map[string]any `json:"extra"`
	ProxyID                 *int64         `json:"proxy_id"`
	Concurrency             int            `json:"concurrency"`
	Priority                int            `json:"priority"`
	RateMultiplier          *float64       `json:"rate_multiplier"`
	LoadFactor              *int           `json:"load_factor"`
	GroupIDs                []int64        `json:"group_ids"`
	ExpiresAt               *int64         `json:"expires_at"`
	AutoPauseOnExpired      *bool          `json:"auto_pause_on_expired"`
	ConfirmMixedChannelRisk *bool          `json:"confirm_mixed_channel_risk"` // 用户确认混合渠道风险
}

// UpdateAccountRequest represents update account request
// 使用指针类型来区分"未提供"和"设置为0"
type UpdateAccountRequest struct {
	Name                    string         `json:"name"`
	Notes                   *string        `json:"notes"`
	Type                    string         `json:"type" binding:"omitempty,oneof=oauth setup-token apikey upstream bedrock service_account"`
	Credentials             map[string]any `json:"credentials"`
	Extra                   map[string]any `json:"extra"`
	ProxyID                 *int64         `json:"proxy_id"`
	Concurrency             *int           `json:"concurrency"`
	Priority                *int           `json:"priority"`
	RateMultiplier          *float64       `json:"rate_multiplier"`
	LoadFactor              *int           `json:"load_factor"`
	Status                  string         `json:"status" binding:"omitempty,oneof=active inactive error"`
	GroupIDs                *[]int64       `json:"group_ids"`
	ExpiresAt               *int64         `json:"expires_at"`
	AutoPauseOnExpired      *bool          `json:"auto_pause_on_expired"`
	ConfirmMixedChannelRisk *bool          `json:"confirm_mixed_channel_risk"` // 用户确认混合渠道风险
}

// BulkUpdateAccountsRequest represents the payload for bulk editing accounts
type BulkUpdateAccountsRequest struct {
	AccountIDs              []int64                   `json:"account_ids"`
	Filters                 *BulkUpdateAccountFilters `json:"filters"`
	Name                    string                    `json:"name"`
	ProxyID                 *int64                    `json:"proxy_id"`
	Concurrency             *int                      `json:"concurrency"`
	Priority                *int                      `json:"priority"`
	RateMultiplier          *float64                  `json:"rate_multiplier"`
	LoadFactor              *int                      `json:"load_factor"`
	Status                  string                    `json:"status" binding:"omitempty,oneof=active inactive error"`
	Schedulable             *bool                     `json:"schedulable"`
	GroupIDs                *[]int64                  `json:"group_ids"`
	Credentials             map[string]any            `json:"credentials"`
	Extra                   map[string]any            `json:"extra"`
	ConfirmMixedChannelRisk *bool                     `json:"confirm_mixed_channel_risk"` // 用户确认混合渠道风险
}

type BulkUpdateAccountFilters struct {
	Platform    string `json:"platform"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Group       string `json:"group"`
	Search      string `json:"search"`
	PrivacyMode string `json:"privacy_mode"`
}

// CheckMixedChannelRequest represents check mixed channel risk request
type CheckMixedChannelRequest struct {
	Platform  string  `json:"platform" binding:"required"`
	GroupIDs  []int64 `json:"group_ids"`
	AccountID *int64  `json:"account_id"`
}

// AccountWithConcurrency extends Account with real-time concurrency info
type AccountWithConcurrency struct {
	*dto.Account
	CurrentConcurrency int `json:"current_concurrency"`
	// 以下字段仅对 Anthropic OAuth/SetupToken 账号有效，且仅在启用相应功能时返回
	CurrentWindowCost *float64 `json:"current_window_cost,omitempty"` // 当前窗口费用
	ActiveSessions    *int     `json:"active_sessions,omitempty"`     // 当前活跃会话数
	CurrentRPM        *int     `json:"current_rpm,omitempty"`         // 当前分钟 RPM 计数
}

const accountListGroupUngroupedQueryValue = "ungrouped"

// truncateErrorMessage trims an error message to a reasonable persisted length
// so accounts.error_message stays readable in the admin UI.
func truncateErrorMessage(msg string) string {
	const maxLen = 480
	msg = strings.TrimSpace(msg)
	if len(msg) <= maxLen {
		return msg
	}
	return msg[:maxLen] + "..."
}

// extractRefreshErrorMessage 从 OAuth 刷新失败返回的 error 中提取人类可读的摘要：
//   - 优先解析 ApplicationError 的 Message（去掉 reason= 包裹的格式串）；
//   - 尝试解析嵌套的 upstream JSON 错误体，提取 .error.message；
//   - 否则回退到 err.Error()。
//
// 截断到 truncateErrorMessage 的长度上限，作为账号 error_message 持久化。
func extractRefreshErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	// 只在 err 真的是 ApplicationError 时走结构化路径；FromError 在非应用错误上
	// 会返回 cause 包装的 UnknownMessage="internal error"，那会把上游 body 信息覆盖掉。
	var appErr *infraerrors.ApplicationError
	if errors.As(err, &appErr) && appErr != nil && appErr.Message != "" && appErr.Message != infraerrors.UnknownMessage {
		// 在 message 内寻找 body: {"error":{"message":"..."}} 这种内嵌 JSON 段。
		if inner := extractUpstreamJSONErrorMessage(appErr.Message); inner != "" {
			return truncateErrorMessage(inner)
		}
		// 当 reason 已经能完整表达错误时优先用 reason，否则用 message。
		if appErr.Reason != "" && len(appErr.Message) > 200 {
			return truncateErrorMessage(appErr.Reason + ": " + firstLine(appErr.Message))
		}
		return truncateErrorMessage(appErr.Message)
	}
	// 非 ApplicationError（如 Claude OAuth 的 fmt.Errorf("token refresh failed: status 400, body: ...")）
	// 直接从原始 err.Error() 里提取 body JSON 的 message 字段。
	raw := err.Error()
	if inner := extractUpstreamJSONErrorMessage(raw); inner != "" {
		return truncateErrorMessage(inner)
	}
	return truncateErrorMessage(raw)
}

// extractUpstreamJSONErrorMessage 在错误字符串中查找 body: {"error":{"message":"..."}} 形式的 JSON 片段
// 并提取 message 字段。失败时返回空串，由调用方回退。
func extractUpstreamJSONErrorMessage(s string) string {
	idx := strings.Index(s, "body: ")
	if idx < 0 {
		idx = strings.Index(s, "body=")
	}
	if idx < 0 {
		return ""
	}
	// 从 body 之后开始截取，去除转义并尝试 JSON 解码。
	raw := s[idx:]
	if pos := strings.Index(raw, "{"); pos > 0 {
		raw = raw[pos:]
	} else {
		return ""
	}
	// 处理 fmt.Sprintf("%q", ...) 产生的双重转义形式。
	if strings.Contains(raw, `\"`) {
		if unq, err := strconv.Unquote(`"` + strings.ReplaceAll(raw, `"`, `\"`) + `"`); err == nil {
			raw = unq
		} else {
			raw = strings.ReplaceAll(raw, `\"`, `"`)
			raw = strings.ReplaceAll(raw, `\n`, "\n")
			raw = strings.ReplaceAll(raw, `\\`, `\`)
		}
	}
	// 截到最外层 } 配对。
	if end := lastJSONBraceEnd(raw); end > 0 {
		raw = raw[:end+1]
	}
	var parsed struct {
		Error struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil && parsed.Error.Message != "" {
		if parsed.Error.Code != "" {
			return parsed.Error.Code + ": " + parsed.Error.Message
		}
		return parsed.Error.Message
	}
	return ""
}

// lastJSONBraceEnd 返回 raw 中最外层 JSON 对象的右括号位置。
func lastJSONBraceEnd(raw string) int {
	depth := 0
	for i, ch := range raw {
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// firstLine 返回字符串第一行（去掉换行后的内容），用于压缩冗长堆栈或多行 message。
func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// Refresh-error reason 码，前端 i18n namespace = admin.accounts.refresh.errors
const (
	refreshReasonRTRevoked        = "REFRESH_TOKEN_REVOKED"      // refresh_token 已失效/被吊销，必须重新授权
	refreshReasonRTReused         = "REFRESH_TOKEN_REUSED"       // refresh_token 已被使用过（OpenAI rotation）
	refreshReasonInvalidClient    = "OAUTH_INVALID_CLIENT"       // 客户端配置错误
	refreshReasonAccessDenied     = "OAUTH_ACCESS_DENIED"        // 上游拒绝访问
	refreshReasonNoRefreshToken   = "OAUTH_NO_REFRESH_TOKEN"     // 账号 credentials 里压根没有 refresh_token
	refreshReasonMissingProjectID = "OAUTH_MISSING_PROJECT_ID"   // Gemini/Antigravity project_id 缺失
	refreshReasonUpstreamUnavail  = "OAUTH_UPSTREAM_UNAVAILABLE" // OAuth 服务网络/EOF/超时，临时故障
	refreshReasonUpstreamRejected = "OAUTH_UPSTREAM_REJECTED"    // 上游返回 4xx（非上面那些已分类的），还能重试
	refreshReasonUnknown          = "OAUTH_REFRESH_FAILED"       // 兜底
)

// classifyRefreshError 把 OAuth 刷新失败的 error 翻译成带 HTTP code + reason 的 ApplicationError，
// 让前端能根据 reason 找到对应 i18n 文案、并展示合适的 "重新授权" / "稍后再试" 操作。
//
// 输入 err 可能是：
//   - 上游 OAuth 返回的 4xx + body（含 invalid_grant / invalid_refresh_token 等）
//   - 网络层错误（unexpected EOF / connection refused / context deadline）
//   - 配置错误（缺 project_id、缺 refresh_token）
//   - 已经是 ApplicationError 的 OPENAI_OAUTH_* 系列
//
// 输出固定为 ApplicationError，保证 response.ErrorFrom 能拿到正确的 status + reason。
func classifyRefreshError(err error) *infraerrors.ApplicationError {
	if err == nil {
		return nil
	}
	// 只接受真正的 ApplicationError 的 Reason/Code；FromError 会给 plain error 套一个 code=500 的壳，
	// 那会把后面的 4xx 兜底路径误触发。
	var appErr *infraerrors.ApplicationError
	if !errors.As(err, &appErr) {
		appErr = nil
	}
	originalReason := ""
	if appErr != nil {
		originalReason = appErr.Reason
	}

	msg := extractRefreshErrorMessage(err) // 给前端展示用的人类可读摘要
	low := strings.ToLower(err.Error())

	// 1) 不可恢复：refresh_token 已被吊销/失效 → 409 + REFRESH_TOKEN_REVOKED
	if strings.Contains(low, "refresh token not found") ||
		strings.Contains(low, "invalid_grant") ||
		strings.Contains(low, "invalid_refresh_token") {
		return infraerrors.Conflict(refreshReasonRTRevoked, msg)
	}
	if strings.Contains(low, "refresh_token_reused") {
		return infraerrors.Conflict(refreshReasonRTReused, msg)
	}
	if strings.Contains(low, "no refresh token available") {
		return infraerrors.BadRequest(refreshReasonNoRefreshToken, msg)
	}
	if strings.Contains(low, "missing_project_id") {
		return infraerrors.BadRequest(refreshReasonMissingProjectID, msg)
	}
	if strings.Contains(low, "invalid_client") {
		return infraerrors.Conflict(refreshReasonInvalidClient, msg)
	}
	if strings.Contains(low, "access_denied") ||
		strings.Contains(low, "unauthorized_client") {
		return infraerrors.Forbidden(refreshReasonAccessDenied, msg)
	}

	// 2) 网络/上游不可用：unexpected EOF / connection refused / timeout / 502/503/504 → 502 + OAUTH_UPSTREAM_UNAVAILABLE
	if strings.Contains(low, "unexpected eof") ||
		strings.Contains(low, "connection refused") ||
		strings.Contains(low, "no such host") ||
		strings.Contains(low, "timeout") ||
		strings.Contains(low, "deadline exceeded") ||
		strings.Contains(low, "i/o timeout") ||
		strings.Contains(low, "tls handshake") ||
		strings.Contains(low, "broken pipe") {
		return infraerrors.New(http.StatusBadGateway, refreshReasonUpstreamUnavail, msg)
	}

	// 3) 原有的 OPENAI_OAUTH_* reason 直接透传（service 层已分类）
	switch originalReason {
	case "OPENAI_OAUTH_PROXY_REQUIRED":
		// 缺代理，可恢复 → 502
		return infraerrors.New(http.StatusBadGateway, originalReason, msg)
	case "OPENAI_OAUTH_REQUEST_FAILED",
		"OPENAI_OAUTH_TOKEN_EXCHANGE_FAILED",
		"OPENAI_OAUTH_TOKEN_REFRESH_FAILED",
		"OPENAI_OAUTH_CLIENT_INIT_FAILED":
		return infraerrors.New(http.StatusBadGateway, originalReason, msg)
	}

	// 4) 兜底
	if appErr != nil && appErr.Code >= 400 && appErr.Code < 500 {
		// 上游 4xx 但没匹配到已知关键字 → 仍然算上游拒绝（用户层一般得重新授权）
		return infraerrors.New(int(appErr.Code), refreshReasonUpstreamRejected, msg)
	}
	return infraerrors.New(http.StatusBadGateway, refreshReasonUnknown, msg)
}

// CRS sync reason 码，前端 i18n namespace = admin.accounts.crs.errors
const (
	crsReasonInvalidBaseURL     = "CRS_INVALID_BASE_URL"
	crsReasonMissingCredentials = "CRS_MISSING_CREDENTIALS"
	crsReasonAuthFailed         = "CRS_AUTH_FAILED"
	crsReasonUpstreamUnavail    = "CRS_UPSTREAM_UNAVAILABLE"
	crsReasonExportFailed       = "CRS_EXPORT_FAILED"
	crsReasonUnknown            = "CRS_SYNC_FAILED"
)

// classifyCRSError 把 CRS 同步/预览过程中产生的错误翻译成带正确 HTTP code + reason 的 ApplicationError，
// 让 admin UI 能准确告诉用户是"用户名密码错"还是"CRS 服务连不上"还是"配置写错了"。
//
// CRSSyncService 产出的是 plain fmt.Errorf / errors.New，前缀决定语义：
//   - "invalid base_url" / "config is not available" → 400
//   - "username and password are required" → 400
//   - "crs login failed: status=4xx" / "crs login failed: <message>" → 401
//   - 网络 / 5xx / 创建 client 失败 → 502
//   - 其他兜底 → 500
func classifyCRSError(err error) *infraerrors.ApplicationError {
	if err == nil {
		return nil
	}
	msg := truncateErrorMessage(err.Error())
	low := strings.ToLower(err.Error())

	switch {
	case strings.Contains(low, "invalid base_url"),
		strings.Contains(low, "config is not available"):
		return infraerrors.BadRequest(crsReasonInvalidBaseURL, msg)
	case strings.Contains(low, "username and password are required"):
		return infraerrors.BadRequest(crsReasonMissingCredentials, msg)
	case strings.Contains(low, "crs login failed"):
		return infraerrors.Unauthorized(crsReasonAuthFailed, msg)
	case strings.Contains(low, "create http client failed"),
		strings.Contains(low, "unexpected eof"),
		strings.Contains(low, "connection refused"),
		strings.Contains(low, "no such host"),
		strings.Contains(low, "timeout"),
		strings.Contains(low, "deadline exceeded"),
		strings.Contains(low, "tls handshake"):
		return infraerrors.New(http.StatusBadGateway, crsReasonUpstreamUnavail, msg)
	case strings.Contains(low, "export") || strings.Contains(low, "parse"):
		return infraerrors.New(http.StatusBadGateway, crsReasonExportFailed, msg)
	default:
		return infraerrors.New(http.StatusBadGateway, crsReasonUnknown, msg)
	}
}

func (h *AccountHandler) buildAccountResponseWithRuntime(ctx context.Context, account *service.Account) AccountWithConcurrency {
	item := AccountWithConcurrency{
		Account:            dto.AccountFromService(account),
		CurrentConcurrency: 0,
	}
	if account == nil {
		return item
	}

	if h.concurrencyService != nil {
		if counts, err := h.concurrencyService.GetAccountConcurrencyBatch(ctx, []int64{account.ID}); err == nil {
			item.CurrentConcurrency = counts[account.ID]
		}
	}

	if account.IsAnthropicOAuthOrSetupToken() {
		if h.accountUsageService != nil && account.GetWindowCostLimit() > 0 {
			startTime := account.GetCurrentWindowStartTime()
			if stats, err := h.accountUsageService.GetAccountWindowStats(ctx, account.ID, startTime); err == nil && stats != nil {
				cost := stats.StandardCost
				item.CurrentWindowCost = &cost
			}
		}

		if h.sessionLimitCache != nil && account.GetMaxSessions() > 0 {
			idleTimeout := time.Duration(account.GetSessionIdleTimeoutMinutes()) * time.Minute
			idleTimeouts := map[int64]time.Duration{account.ID: idleTimeout}
			if sessions, err := h.sessionLimitCache.GetActiveSessionCountBatch(ctx, []int64{account.ID}, idleTimeouts); err == nil {
				if count, ok := sessions[account.ID]; ok {
					item.ActiveSessions = &count
				}
			}
		}

		if h.rpmCache != nil && account.GetBaseRPM() > 0 {
			if rpm, err := h.rpmCache.GetRPM(ctx, account.ID); err == nil {
				item.CurrentRPM = &rpm
			}
		}
	}

	return item
}

// List handles listing all accounts with pagination
// GET /api/v1/admin/accounts
func (h *AccountHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	platform := c.Query("platform")
	accountType := c.Query("type")
	status := c.Query("status")
	search := c.Query("search")
	privacyMode := strings.TrimSpace(c.Query("privacy_mode"))
	sortBy := c.DefaultQuery("sort_by", "name")
	sortOrder := c.DefaultQuery("sort_order", "asc")
	// 标准化和验证 search 参数
	search = strings.TrimSpace(search)
	if len(search) > 100 {
		search = search[:100]
	}
	lite := parseBoolQueryWithDefault(c.Query("lite"), false)

	var groupID int64
	if groupIDStr := c.Query("group"); groupIDStr != "" {
		if groupIDStr == accountListGroupUngroupedQueryValue {
			groupID = service.AccountListGroupUngrouped
		} else {
			parsedGroupID, parseErr := strconv.ParseInt(groupIDStr, 10, 64)
			if parseErr != nil {
				response.ErrorFrom(c, infraerrors.BadRequest("INVALID_GROUP_FILTER", "invalid group filter"))
				return
			}
			if parsedGroupID < 0 {
				response.ErrorFrom(c, infraerrors.BadRequest("INVALID_GROUP_FILTER", "invalid group filter"))
				return
			}
			groupID = parsedGroupID
		}
	}

	accounts, total, err := h.adminService.ListAccounts(c.Request.Context(), page, pageSize, platform, accountType, status, search, groupID, privacyMode, sortBy, sortOrder)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Get current concurrency counts for all accounts
	accountIDs := make([]int64, len(accounts))
	for i, acc := range accounts {
		accountIDs[i] = acc.ID
	}

	concurrencyCounts := make(map[int64]int)
	var windowCosts map[int64]float64
	var activeSessions map[int64]int
	var rpmCounts map[int64]int

	// 始终获取并发数（Redis ZCARD，极低开销）
	if h.concurrencyService != nil {
		if cc, ccErr := h.concurrencyService.GetAccountConcurrencyBatch(c.Request.Context(), accountIDs); ccErr == nil && cc != nil {
			concurrencyCounts = cc
		}
	}

	// 识别需要查询窗口费用、会话数和 RPM 的账号（Anthropic OAuth/SetupToken 且启用了相应功能）
	windowCostAccountIDs := make([]int64, 0)
	sessionLimitAccountIDs := make([]int64, 0)
	rpmAccountIDs := make([]int64, 0)
	sessionIdleTimeouts := make(map[int64]time.Duration) // 各账号的会话空闲超时配置
	for i := range accounts {
		acc := &accounts[i]
		if acc.IsAnthropicOAuthOrSetupToken() {
			if acc.GetWindowCostLimit() > 0 {
				windowCostAccountIDs = append(windowCostAccountIDs, acc.ID)
			}
			if acc.GetMaxSessions() > 0 {
				sessionLimitAccountIDs = append(sessionLimitAccountIDs, acc.ID)
				sessionIdleTimeouts[acc.ID] = time.Duration(acc.GetSessionIdleTimeoutMinutes()) * time.Minute
			}
			if acc.GetBaseRPM() > 0 {
				rpmAccountIDs = append(rpmAccountIDs, acc.ID)
			}
		}
	}

	// 始终获取 RPM 计数（Redis GET，极低开销）
	if len(rpmAccountIDs) > 0 && h.rpmCache != nil {
		rpmCounts, _ = h.rpmCache.GetRPMBatch(c.Request.Context(), rpmAccountIDs)
		if rpmCounts == nil {
			rpmCounts = make(map[int64]int)
		}
	}

	// 始终获取活跃会话数（Redis ZCARD，低开销）
	if len(sessionLimitAccountIDs) > 0 && h.sessionLimitCache != nil {
		activeSessions, _ = h.sessionLimitCache.GetActiveSessionCountBatch(c.Request.Context(), sessionLimitAccountIDs, sessionIdleTimeouts)
		if activeSessions == nil {
			activeSessions = make(map[int64]int)
		}
	}

	// 始终获取窗口费用（PostgreSQL 聚合查询）
	if len(windowCostAccountIDs) > 0 {
		windowCosts = make(map[int64]float64)
		var mu sync.Mutex
		g, gctx := errgroup.WithContext(c.Request.Context())
		g.SetLimit(10) // 限制并发数

		for i := range accounts {
			acc := &accounts[i]
			if !acc.IsAnthropicOAuthOrSetupToken() || acc.GetWindowCostLimit() <= 0 {
				continue
			}
			accCopy := acc // 闭包捕获
			g.Go(func() error {
				// 使用统一的窗口开始时间计算逻辑（考虑窗口过期情况）
				startTime := accCopy.GetCurrentWindowStartTime()
				stats, err := h.accountUsageService.GetAccountWindowStats(gctx, accCopy.ID, startTime)
				if err == nil && stats != nil {
					mu.Lock()
					windowCosts[accCopy.ID] = stats.StandardCost // 使用标准费用
					mu.Unlock()
				}
				return nil // 不返回错误，允许部分失败
			})
		}
		_ = g.Wait()
	}

	// Build response with concurrency info
	result := make([]AccountWithConcurrency, len(accounts))
	for i := range accounts {
		acc := &accounts[i]
		item := AccountWithConcurrency{
			Account:            dto.AccountFromService(acc),
			CurrentConcurrency: concurrencyCounts[acc.ID],
		}

		// 添加窗口费用（仅当启用时）
		if windowCosts != nil {
			if cost, ok := windowCosts[acc.ID]; ok {
				item.CurrentWindowCost = &cost
			}
		}

		// 添加活跃会话数（仅当启用时）
		if activeSessions != nil {
			if count, ok := activeSessions[acc.ID]; ok {
				item.ActiveSessions = &count
			}
		}

		// 添加 RPM 计数（仅当启用时）
		if rpmCounts != nil {
			if rpm, ok := rpmCounts[acc.ID]; ok {
				item.CurrentRPM = &rpm
			}
		}

		result[i] = item
	}

	etag := buildAccountsListETag(result, total, page, pageSize, platform, accountType, status, search, lite)
	if etag != "" {
		c.Header("ETag", etag)
		c.Header("Vary", "If-None-Match")
		if ifNoneMatchMatched(c.GetHeader("If-None-Match"), etag) {
			c.Status(http.StatusNotModified)
			return
		}
	}

	response.Paginated(c, result, total, page, pageSize)
}

func buildAccountsListETag(
	items []AccountWithConcurrency,
	total int64,
	page, pageSize int,
	platform, accountType, status, search string,
	lite bool,
) string {
	payload := struct {
		Total       int64                    `json:"total"`
		Page        int                      `json:"page"`
		PageSize    int                      `json:"page_size"`
		Platform    string                   `json:"platform"`
		AccountType string                   `json:"type"`
		Status      string                   `json:"status"`
		Search      string                   `json:"search"`
		Lite        bool                     `json:"lite"`
		Items       []AccountWithConcurrency `json:"items"`
	}{
		Total:       total,
		Page:        page,
		PageSize:    pageSize,
		Platform:    platform,
		AccountType: accountType,
		Status:      status,
		Search:      search,
		Lite:        lite,
		Items:       items,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "\"" + hex.EncodeToString(sum[:]) + "\""
}

func ifNoneMatchMatched(ifNoneMatch, etag string) bool {
	if etag == "" || ifNoneMatch == "" {
		return false
	}
	for _, token := range strings.Split(ifNoneMatch, ",") {
		candidate := strings.TrimSpace(token)
		if candidate == "*" {
			return true
		}
		if candidate == etag {
			return true
		}
		if strings.HasPrefix(candidate, "W/") && strings.TrimPrefix(candidate, "W/") == etag {
			return true
		}
	}
	return false
}

// GetByID handles getting an account by ID
// GET /api/v1/admin/accounts/:id
func (h *AccountHandler) GetByID(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// CheckMixedChannel handles checking mixed channel risk for account-group binding.
// POST /api/v1/admin/accounts/check-mixed-channel
func (h *AccountHandler) CheckMixedChannel(c *gin.Context) {
	var req CheckMixedChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if len(req.GroupIDs) == 0 {
		response.Success(c, gin.H{"has_risk": false})
		return
	}

	accountID := int64(0)
	if req.AccountID != nil {
		accountID = *req.AccountID
	}

	err := h.adminService.CheckMixedChannelRisk(c.Request.Context(), accountID, req.Platform, req.GroupIDs)
	if err != nil {
		var mixedErr *service.MixedChannelError
		if errors.As(err, &mixedErr) {
			response.Success(c, gin.H{
				"has_risk": true,
				"error":    "mixed_channel_warning",
				"message":  mixedErr.Error(),
				"details": gin.H{
					"group_id":         mixedErr.GroupID,
					"group_name":       mixedErr.GroupName,
					"current_platform": mixedErr.CurrentPlatform,
					"other_platform":   mixedErr.OtherPlatform,
				},
			})
			return
		}

		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"has_risk": false})
}

// Create handles creating a new account
// POST /api/v1/admin/accounts
func (h *AccountHandler) Create(c *gin.Context) {
	var req CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}
	// base_rpm 输入校验：负值归零，超过 10000 截断
	sanitizeExtraBaseRPM(req.Extra)

	// 确定是否跳过混合渠道检查
	skipCheck := req.ConfirmMixedChannelRisk != nil && *req.ConfirmMixedChannelRisk

	// 捕获闭包内创建的账号引用，用于创建成功后触发异步探测。
	// 幂等重放时闭包不会执行 → createdAccount 为 nil → 不重复调度。
	var createdAccount *service.Account

	result, err := executeAdminIdempotent(c, "admin.accounts.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		account, execErr := h.adminService.CreateAccount(ctx, &service.CreateAccountInput{
			Name:                  req.Name,
			Notes:                 req.Notes,
			Platform:              req.Platform,
			Type:                  req.Type,
			Credentials:           req.Credentials,
			Extra:                 req.Extra,
			ProxyID:               req.ProxyID,
			Concurrency:           req.Concurrency,
			Priority:              req.Priority,
			RateMultiplier:        req.RateMultiplier,
			LoadFactor:            req.LoadFactor,
			GroupIDs:              req.GroupIDs,
			ExpiresAt:             req.ExpiresAt,
			AutoPauseOnExpired:    req.AutoPauseOnExpired,
			SkipMixedChannelCheck: skipCheck,
		})
		if execErr != nil {
			return nil, execErr
		}
		createdAccount = account
		// Antigravity OAuth: 新账号直接设置隐私
		h.adminService.ForceAntigravityPrivacy(ctx, account)
		// OpenAI OAuth: 新账号直接设置隐私
		h.adminService.ForceOpenAIPrivacy(ctx, account)
		return h.buildAccountResponseWithRuntime(ctx, account), nil
	})
	if err != nil {
		// 检查是否为混合渠道错误
		var mixedErr *service.MixedChannelError
		if errors.As(err, &mixedErr) {
			// 创建接口仅返回最小必要字段，详细信息由专门检查接口提供
			c.JSON(409, gin.H{
				"error":   "mixed_channel_warning",
				"message": mixedErr.Error(),
			})
			return
		}

		if retryAfter := service.RetryAfterSecondsFromError(err); retryAfter > 0 {
			c.Header("Retry-After", strconv.Itoa(retryAfter))
		}
		response.ErrorFrom(c, err)
		return
	}

	if result != nil && result.Replayed {
		c.Header("X-Idempotency-Replayed", "true")
	}
	// OpenAI APIKey 账号创建后异步探测上游 /v1/responses 能力。
	// 探测失败不影响账号创建响应。
	h.scheduleOpenAIResponsesProbe(createdAccount)
	response.Success(c, result.Data)
}

// Update handles updating an account
// PUT /api/v1/admin/accounts/:id
func (h *AccountHandler) Update(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	var req UpdateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}
	// base_rpm 输入校验：负值归零，超过 10000 截断
	sanitizeExtraBaseRPM(req.Extra)

	// 确定是否跳过混合渠道检查
	skipCheck := req.ConfirmMixedChannelRisk != nil && *req.ConfirmMixedChannelRisk

	account, err := h.adminService.UpdateAccount(c.Request.Context(), accountID, &service.UpdateAccountInput{
		Name:                  req.Name,
		Notes:                 req.Notes,
		Type:                  req.Type,
		Credentials:           req.Credentials,
		Extra:                 req.Extra,
		ProxyID:               req.ProxyID,
		Concurrency:           req.Concurrency, // 指针类型，nil 表示未提供
		Priority:              req.Priority,    // 指针类型，nil 表示未提供
		RateMultiplier:        req.RateMultiplier,
		LoadFactor:            req.LoadFactor,
		Status:                req.Status,
		GroupIDs:              req.GroupIDs,
		ExpiresAt:             req.ExpiresAt,
		AutoPauseOnExpired:    req.AutoPauseOnExpired,
		SkipMixedChannelCheck: skipCheck,
	})
	if err != nil {
		// 检查是否为混合渠道错误
		var mixedErr *service.MixedChannelError
		if errors.As(err, &mixedErr) {
			// 更新接口仅返回最小必要字段，详细信息由专门检查接口提供
			c.JSON(409, gin.H{
				"error":   "mixed_channel_warning",
				"message": mixedErr.Error(),
			})
			return
		}

		response.ErrorFrom(c, err)
		return
	}

	// OpenAI APIKey: credentials 修改后重新探测上游能力（base_url/api_key 可能变更）。
	// 异步执行，探测失败不影响账号更新响应。
	if len(req.Credentials) > 0 {
		h.scheduleOpenAIResponsesProbe(account)
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// scheduleOpenAIResponsesProbe 异步触发 OpenAI APIKey 账号的 Responses API 能力探测。
//
// 仅对 platform=openai && type=apikey 账号生效；其他账号无操作。
// 探测本身在 goroutine 中执行（会发一次 HTTP 请求到上游），不会阻塞
// 当前请求。探测错误仅记录日志，不向上下文传播：探测失败时标记保持缺失，
// 网关会按"现状即证据"默认走 Responses。
func (h *AccountHandler) scheduleOpenAIResponsesProbe(account *service.Account) {
	if account == nil || account.Platform != service.PlatformOpenAI || account.Type != service.AccountTypeAPIKey {
		return
	}
	if h.accountTestService == nil {
		return
	}
	accountID := account.ID
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("openai_responses_probe_panic", "account_id", accountID, "recover", r)
			}
		}()
		h.accountTestService.ProbeOpenAIAPIKeyResponsesSupport(context.Background(), accountID)
	}()
}

// Delete handles deleting an account
// DELETE /api/v1/admin/accounts/:id
func (h *AccountHandler) Delete(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	err = h.adminService.DeleteAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Account deleted successfully"})
}

// TestAccountRequest represents the request body for testing an account
type TestAccountRequest struct {
	ModelID string `json:"model_id"`
	Prompt  string `json:"prompt"`
	Mode    string `json:"mode"`
}

type SyncFromCRSRequest struct {
	BaseURL            string   `json:"base_url" binding:"required"`
	Username           string   `json:"username" binding:"required"`
	Password           string   `json:"password" binding:"required"`
	SyncProxies        *bool    `json:"sync_proxies"`
	SelectedAccountIDs []string `json:"selected_account_ids"`
}

type PreviewFromCRSRequest struct {
	BaseURL  string `json:"base_url" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Test handles testing account connectivity with SSE streaming
// POST /api/v1/admin/accounts/:id/test
func (h *AccountHandler) Test(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	var req TestAccountRequest
	// Allow empty body, model_id is optional
	_ = c.ShouldBindJSON(&req)

	// Use AccountTestService to test the account with SSE streaming
	if err := h.accountTestService.TestAccountConnection(c, accountID, req.ModelID, req.Prompt, req.Mode); err != nil {
		// 测试失败：持久化错误状态，前端 reload 时可以看到。
		// 使用 Background 上下文，避免 SSE 流关闭导致写库失败。
		bgCtx := context.Background()
		if setErr := h.adminService.SetAccountError(bgCtx, accountID, extractRefreshErrorMessage(err)); setErr != nil {
			log.Printf("[WARN] failed to persist test error for account %d: %v", accountID, setErr)
		}
		return
	}

	if h.rateLimitService != nil {
		if _, err := h.rateLimitService.RecoverAccountAfterSuccessfulTest(c.Request.Context(), accountID); err != nil {
			_ = c.Error(err)
		}
	}
}

// RecoverState handles unified recovery of recoverable account runtime state.
// POST /api/v1/admin/accounts/:id/recover-state
func (h *AccountHandler) RecoverState(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	if h.rateLimitService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Rate limit service unavailable")
		return
	}

	if _, err := h.rateLimitService.RecoverAccountState(c.Request.Context(), accountID, service.AccountRecoveryOptions{
		InvalidateToken: true,
	}); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// SyncFromCRS handles syncing accounts from claude-relay-service (CRS)
// POST /api/v1/admin/accounts/sync/crs
func (h *AccountHandler) SyncFromCRS(c *gin.Context) {
	var req SyncFromCRSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Default to syncing proxies (can be disabled by explicitly setting false)
	syncProxies := true
	if req.SyncProxies != nil {
		syncProxies = *req.SyncProxies
	}

	result, err := h.crsSyncService.SyncFromCRS(c.Request.Context(), service.SyncFromCRSInput{
		BaseURL:            req.BaseURL,
		Username:           req.Username,
		Password:           req.Password,
		SyncProxies:        syncProxies,
		SelectedAccountIDs: req.SelectedAccountIDs,
	})
	if err != nil {
		response.ErrorFrom(c, classifyCRSError(err))
		return
	}

	response.Success(c, result)
}

// PreviewFromCRS handles previewing accounts from CRS before sync
// POST /api/v1/admin/accounts/sync/crs/preview
func (h *AccountHandler) PreviewFromCRS(c *gin.Context) {
	var req PreviewFromCRSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.crsSyncService.PreviewFromCRS(c.Request.Context(), service.SyncFromCRSInput{
		BaseURL:  req.BaseURL,
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		response.ErrorFrom(c, classifyCRSError(err))
		return
	}

	response.Success(c, result)
}

// refreshSingleAccount refreshes credentials for a single OAuth account.
// Returns (updatedAccount, warning, error) where warning is used for Antigravity ProjectIDMissing scenario.
func (h *AccountHandler) refreshSingleAccount(ctx context.Context, account *service.Account) (*service.Account, string, error) {
	if !account.IsOAuth() {
		return nil, "", infraerrors.BadRequest("NOT_OAUTH", "cannot refresh non-OAuth account")
	}

	var newCredentials map[string]any

	if account.IsOpenAI() {
		tokenInfo, err := h.openaiOAuthService.RefreshAccountToken(ctx, account)
		if err != nil {
			// 刷新失败但 access_token 可能仍有效，尝试设置隐私
			h.adminService.EnsureOpenAIPrivacy(ctx, account)
			return nil, "", err
		}

		newCredentials = h.openaiOAuthService.BuildAccountCredentials(tokenInfo)
		for k, v := range account.Credentials {
			if _, exists := newCredentials[k]; !exists {
				newCredentials[k] = v
			}
		}
	} else if account.Platform == service.PlatformGemini {
		tokenInfo, err := h.geminiOAuthService.RefreshAccountToken(ctx, account)
		if err != nil {
			return nil, "", fmt.Errorf("failed to refresh credentials: %w", err)
		}

		newCredentials = h.geminiOAuthService.BuildAccountCredentials(tokenInfo)
		for k, v := range account.Credentials {
			if _, exists := newCredentials[k]; !exists {
				newCredentials[k] = v
			}
		}
	} else if account.Platform == service.PlatformAntigravity {
		tokenInfo, err := h.antigravityOAuthService.RefreshAccountToken(ctx, account)
		if err != nil {
			return nil, "", err
		}

		newCredentials = h.antigravityOAuthService.BuildAccountCredentials(tokenInfo)
		for k, v := range account.Credentials {
			if _, exists := newCredentials[k]; !exists {
				newCredentials[k] = v
			}
		}

		// 特殊处理 project_id：如果新值为空但旧值非空，保留旧值
		// 这确保了即使 LoadCodeAssist 失败，project_id 也不会丢失
		if newProjectID, _ := newCredentials["project_id"].(string); newProjectID == "" {
			if oldProjectID := strings.TrimSpace(account.GetCredential("project_id")); oldProjectID != "" {
				newCredentials["project_id"] = oldProjectID
			}
		}

		// 如果 project_id 获取失败，更新凭证但不标记为 error
		if tokenInfo.ProjectIDMissing {
			updatedAccount, updateErr := h.adminService.UpdateAccount(ctx, account.ID, &service.UpdateAccountInput{
				Credentials: newCredentials,
			})
			if updateErr != nil {
				return nil, "", fmt.Errorf("failed to update credentials: %w", updateErr)
			}
			h.adminService.EnsureAntigravityPrivacy(ctx, updatedAccount)
			return updatedAccount, "missing_project_id_temporary", nil
		}

		// 成功获取到 project_id，如果之前是 missing_project_id 错误则清除
		if account.Status == service.StatusError && strings.Contains(account.ErrorMessage, "missing_project_id:") {
			if _, clearErr := h.adminService.ClearAccountError(ctx, account.ID); clearErr != nil {
				return nil, "", fmt.Errorf("failed to clear account error: %w", clearErr)
			}
		}
	} else {
		// Use Anthropic/Claude OAuth service to refresh token
		tokenInfo, err := h.oauthService.RefreshAccountToken(ctx, account)
		if err != nil {
			return nil, "", err
		}

		// Copy existing credentials to preserve non-token settings (e.g., intercept_warmup_requests)
		newCredentials = make(map[string]any)
		for k, v := range account.Credentials {
			newCredentials[k] = v
		}

		// Update token-related fields
		newCredentials["access_token"] = tokenInfo.AccessToken
		newCredentials["token_type"] = tokenInfo.TokenType
		newCredentials["expires_in"] = strconv.FormatInt(tokenInfo.ExpiresIn, 10)
		newCredentials["expires_at"] = strconv.FormatInt(tokenInfo.ExpiresAt, 10)
		if strings.TrimSpace(tokenInfo.RefreshToken) != "" {
			newCredentials["refresh_token"] = tokenInfo.RefreshToken
		}
		if strings.TrimSpace(tokenInfo.Scope) != "" {
			newCredentials["scope"] = tokenInfo.Scope
		}
	}

	updatedAccount, err := h.adminService.UpdateAccount(ctx, account.ID, &service.UpdateAccountInput{
		Credentials: newCredentials,
	})
	if err != nil {
		return nil, "", err
	}

	// 刷新成功后，清除 token 缓存，确保下次请求使用新 token
	if h.tokenCacheInvalidator != nil {
		if invalidateErr := h.tokenCacheInvalidator.InvalidateToken(ctx, updatedAccount); invalidateErr != nil {
			log.Printf("[WARN] Failed to invalidate token cache for account %d: %v", updatedAccount.ID, invalidateErr)
		}
	}

	// OpenAI OAuth: 刷新成功后检查并设置 privacy_mode
	h.adminService.EnsureOpenAIPrivacy(ctx, updatedAccount)
	// Antigravity OAuth: 刷新成功后检查并设置 privacy_mode
	h.adminService.EnsureAntigravityPrivacy(ctx, updatedAccount)

	return updatedAccount, "", nil
}

// Refresh handles refreshing account credentials
// POST /api/v1/admin/accounts/:id/refresh
func (h *AccountHandler) Refresh(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	// Get account
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}

	updatedAccount, warning, err := h.refreshSingleAccount(c.Request.Context(), account)
	if err != nil {
		classified := classifyRefreshError(err)
		// 持久化错误状态，确保 UI reload 时能看到刷新失败而不是"无变化"。
		if setErr := h.adminService.SetAccountError(c.Request.Context(), accountID, extractRefreshErrorMessage(err)); setErr != nil {
			log.Printf("[WARN] failed to persist refresh error for account %d: %v", accountID, setErr)
		}
		response.ErrorFrom(c, classified)
		return
	}

	if warning == "missing_project_id_temporary" {
		response.Success(c, gin.H{
			"message": "Token refreshed successfully, but project_id could not be retrieved (will retry automatically)",
			"warning": "missing_project_id_temporary",
		})
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), updatedAccount))
}

// GetStats handles getting account statistics
// GET /api/v1/admin/accounts/:id/stats
func (h *AccountHandler) GetStats(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	// Parse days parameter (default 30)
	days := 30
	if daysStr := c.Query("days"); daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 90 {
			days = d
		}
	}

	// Calculate time range
	now := timezone.Now()
	endTime := timezone.StartOfDay(now.AddDate(0, 0, 1))
	startTime := timezone.StartOfDay(now.AddDate(0, 0, -days+1))

	stats, err := h.accountUsageService.GetAccountUsageStats(c.Request.Context(), accountID, startTime, endTime)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// ClearError handles clearing account error
// POST /api/v1/admin/accounts/:id/clear-error
func (h *AccountHandler) ClearError(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	account, err := h.adminService.ClearAccountError(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 清除错误后，同时清除 token 缓存，确保下次请求会获取最新的 token（触发刷新或从 DB 读取）
	// 这解决了管理员重置账号状态后，旧的失效 token 仍在缓存中导致立即再次 401 的问题
	if h.tokenCacheInvalidator != nil && account.IsOAuth() {
		if invalidateErr := h.tokenCacheInvalidator.InvalidateToken(c.Request.Context(), account); invalidateErr != nil {
			log.Printf("[WARN] Failed to invalidate token cache for account %d: %v", accountID, invalidateErr)
		}
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// BatchClearError handles batch clearing account errors
// POST /api/v1/admin/accounts/batch-clear-error
func (h *AccountHandler) BatchClearError(c *gin.Context) {
	var req struct {
		AccountIDs []int64                   `json:"account_ids"`
		Filters    *BulkUpdateAccountFilters `json:"filters"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	ctx := c.Request.Context()

	accountIDs := req.AccountIDs
	if len(accountIDs) == 0 && req.Filters != nil {
		resolved, err := h.adminService.ResolveAccountIDsByFilters(ctx, toServiceBulkUpdateAccountFilters(req.Filters))
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		accountIDs = resolved
	}
	if len(accountIDs) == 0 {
		response.BadRequest(c, "account_ids or filters is required")
		return
	}

	const maxConcurrency = 10
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	var mu sync.Mutex
	var successCount, failedCount int
	var errors []gin.H

	// 注意：所有 goroutine 必须 return nil，避免 errgroup cancel 其他并发任务
	for _, id := range accountIDs {
		accountID := id // 闭包捕获
		g.Go(func() error {
			account, err := h.adminService.ClearAccountError(gctx, accountID)
			if err != nil {
				mu.Lock()
				failedCount++
				errors = append(errors, gin.H{
					"account_id": accountID,
					"error":      err.Error(),
				})
				mu.Unlock()
				return nil
			}

			// 清除错误后，同时清除 token 缓存
			if h.tokenCacheInvalidator != nil && account.IsOAuth() {
				if invalidateErr := h.tokenCacheInvalidator.InvalidateToken(gctx, account); invalidateErr != nil {
					log.Printf("[WARN] Failed to invalidate token cache for account %d: %v", accountID, invalidateErr)
				}
			}

			mu.Lock()
			successCount++
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"total":   len(accountIDs),
		"success": successCount,
		"failed":  failedCount,
		"errors":  errors,
	})
}

// BatchRefresh handles batch refreshing account credentials
// POST /api/v1/admin/accounts/batch-refresh
func (h *AccountHandler) BatchRefresh(c *gin.Context) {
	var req struct {
		AccountIDs []int64                   `json:"account_ids"`
		Filters    *BulkUpdateAccountFilters `json:"filters"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	ctx := c.Request.Context()

	accountIDs := req.AccountIDs
	if len(accountIDs) == 0 && req.Filters != nil {
		resolved, err := h.adminService.ResolveAccountIDsByFilters(ctx, toServiceBulkUpdateAccountFilters(req.Filters))
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		accountIDs = resolved
	}
	if len(accountIDs) == 0 {
		response.BadRequest(c, "account_ids or filters is required")
		return
	}

	accounts, err := h.adminService.GetAccountsByIDs(ctx, accountIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 建立已获取账号的 ID 集合，检测缺失的 ID
	foundIDs := make(map[int64]bool, len(accounts))
	for _, acc := range accounts {
		if acc != nil {
			foundIDs[acc.ID] = true
		}
	}

	const maxConcurrency = 10
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	var mu sync.Mutex
	var successCount, failedCount int
	var errors []gin.H
	var warnings []gin.H

	// 将不存在的账号 ID 标记为失败
	for _, id := range accountIDs {
		if !foundIDs[id] {
			failedCount++
			errors = append(errors, gin.H{
				"account_id": id,
				"error":      "account not found",
			})
		}
	}

	// 注意：所有 goroutine 必须 return nil，避免 errgroup cancel 其他并发任务
	for _, account := range accounts {
		acc := account // 闭包捕获
		if acc == nil {
			continue
		}
		g.Go(func() error {
			_, warning, err := h.refreshSingleAccount(gctx, acc)
			mu.Lock()
			if err != nil {
				failedCount++
				classified := classifyRefreshError(err)
				errors = append(errors, gin.H{
					"account_id": acc.ID,
					"error":      classified.Message,
					"reason":     classified.Reason,
				})
				// 刷新失败时持久化错误状态，让前端在 reload 时观察到。
				if setErr := h.adminService.SetAccountError(gctx, acc.ID, extractRefreshErrorMessage(err)); setErr != nil {
					log.Printf("[WARN] failed to persist refresh error for account %d: %v", acc.ID, setErr)
				}
			} else {
				successCount++
				if warning != "" {
					warnings = append(warnings, gin.H{
						"account_id": acc.ID,
						"warning":    warning,
					})
				}
			}
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"total":    len(accountIDs),
		"success":  successCount,
		"failed":   failedCount,
		"errors":   errors,
		"warnings": warnings,
	})
}

// BatchCreate handles batch creating accounts
// POST /api/v1/admin/accounts/batch
func (h *AccountHandler) BatchCreate(c *gin.Context) {
	var req struct {
		Accounts []CreateAccountRequest `json:"accounts" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.batch_create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		success := 0
		failed := 0
		results := make([]gin.H, 0, len(req.Accounts))
		// 收集需要异步设置隐私的 OAuth 账号
		var antigravityPrivacyAccounts []*service.Account
		var openaiPrivacyAccounts []*service.Account

		for _, item := range req.Accounts {
			if item.RateMultiplier != nil && *item.RateMultiplier < 0 {
				failed++
				results = append(results, gin.H{
					"name":    item.Name,
					"success": false,
					"error":   "rate_multiplier must be >= 0",
				})
				continue
			}

			// base_rpm 输入校验：负值归零，超过 10000 截断
			sanitizeExtraBaseRPM(item.Extra)

			skipCheck := item.ConfirmMixedChannelRisk != nil && *item.ConfirmMixedChannelRisk

			account, err := h.adminService.CreateAccount(ctx, &service.CreateAccountInput{
				Name:                  item.Name,
				Notes:                 item.Notes,
				Platform:              item.Platform,
				Type:                  item.Type,
				Credentials:           item.Credentials,
				Extra:                 item.Extra,
				ProxyID:               item.ProxyID,
				Concurrency:           item.Concurrency,
				Priority:              item.Priority,
				RateMultiplier:        item.RateMultiplier,
				GroupIDs:              item.GroupIDs,
				ExpiresAt:             item.ExpiresAt,
				AutoPauseOnExpired:    item.AutoPauseOnExpired,
				SkipMixedChannelCheck: skipCheck,
			})
			if err != nil {
				failed++
				results = append(results, gin.H{
					"name":    item.Name,
					"success": false,
					"error":   err.Error(),
				})
				continue
			}
			// 收集需要异步设置隐私的 OAuth 账号
			if account.Type == service.AccountTypeOAuth {
				switch account.Platform {
				case service.PlatformAntigravity:
					antigravityPrivacyAccounts = append(antigravityPrivacyAccounts, account)
				case service.PlatformOpenAI:
					openaiPrivacyAccounts = append(openaiPrivacyAccounts, account)
				}
			}
			// OpenAI APIKey 账号异步探测 /v1/responses 能力。
			h.scheduleOpenAIResponsesProbe(account)
			success++
			results = append(results, gin.H{
				"name":    item.Name,
				"id":      account.ID,
				"success": true,
			})
		}

		// 异步设置隐私，避免批量创建时阻塞请求
		adminSvc := h.adminService
		if len(antigravityPrivacyAccounts) > 0 {
			accounts := antigravityPrivacyAccounts
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("batch_create_antigravity_privacy_panic", "recover", r)
					}
				}()
				bgCtx := context.Background()
				for _, acc := range accounts {
					adminSvc.ForceAntigravityPrivacy(bgCtx, acc)
				}
			}()
		}
		if len(openaiPrivacyAccounts) > 0 {
			accounts := openaiPrivacyAccounts
			go func() {
				defer func() {
					if r := recover(); r != nil {
						slog.Error("batch_create_openai_privacy_panic", "recover", r)
					}
				}()
				bgCtx := context.Background()
				for _, acc := range accounts {
					adminSvc.ForceOpenAIPrivacy(bgCtx, acc)
				}
			}()
		}

		return gin.H{
			"success": success,
			"failed":  failed,
			"results": results,
		}, nil
	})
}

// BatchUpdateCredentialsRequest represents batch credentials update request
type BatchUpdateCredentialsRequest struct {
	AccountIDs []int64 `json:"account_ids" binding:"required,min=1"`
	Field      string  `json:"field" binding:"required,oneof=account_uuid org_uuid intercept_warmup_requests"`
	Value      any     `json:"value"`
}

// BatchUpdateCredentials handles batch updating credentials fields
// POST /api/v1/admin/accounts/batch-update-credentials
func (h *AccountHandler) BatchUpdateCredentials(c *gin.Context) {
	var req BatchUpdateCredentialsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// Validate value type based on field
	if req.Field == "intercept_warmup_requests" {
		// Must be boolean
		if _, ok := req.Value.(bool); !ok {
			response.BadRequest(c, "intercept_warmup_requests must be boolean")
			return
		}
	} else {
		// account_uuid and org_uuid can be string or null
		if req.Value != nil {
			if _, ok := req.Value.(string); !ok {
				response.BadRequest(c, req.Field+" must be string or null")
				return
			}
		}
	}

	ctx := c.Request.Context()

	// 阶段一：预验证所有账号存在，收集 credentials
	type accountUpdate struct {
		ID          int64
		Credentials map[string]any
	}
	updates := make([]accountUpdate, 0, len(req.AccountIDs))
	for _, accountID := range req.AccountIDs {
		account, err := h.adminService.GetAccount(ctx, accountID)
		if err != nil {
			response.Error(c, 404, fmt.Sprintf("Account %d not found", accountID))
			return
		}
		if account.Credentials == nil {
			account.Credentials = make(map[string]any)
		}
		account.Credentials[req.Field] = req.Value
		updates = append(updates, accountUpdate{ID: accountID, Credentials: account.Credentials})
	}

	// 阶段二：依次更新，返回每个账号的成功/失败明细，便于调用方重试
	success := 0
	failed := 0
	successIDs := make([]int64, 0, len(updates))
	failedIDs := make([]int64, 0, len(updates))
	results := make([]gin.H, 0, len(updates))
	for _, u := range updates {
		updateInput := &service.UpdateAccountInput{Credentials: u.Credentials}
		if _, err := h.adminService.UpdateAccount(ctx, u.ID, updateInput); err != nil {
			failed++
			failedIDs = append(failedIDs, u.ID)
			results = append(results, gin.H{
				"account_id": u.ID,
				"success":    false,
				"error":      err.Error(),
			})
			continue
		}
		success++
		successIDs = append(successIDs, u.ID)
		results = append(results, gin.H{
			"account_id": u.ID,
			"success":    true,
		})
	}

	response.Success(c, gin.H{
		"success":     success,
		"failed":      failed,
		"success_ids": successIDs,
		"failed_ids":  failedIDs,
		"results":     results,
	})
}

// BulkUpdate handles bulk updating accounts with selected fields/credentials.
// POST /api/v1/admin/accounts/bulk-update
func (h *AccountHandler) BulkUpdate(c *gin.Context) {
	var req BulkUpdateAccountsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}
	if len(req.AccountIDs) == 0 && req.Filters == nil {
		response.BadRequest(c, "account_ids or filters is required")
		return
	}
	// base_rpm 输入校验：负值归零，超过 10000 截断
	sanitizeExtraBaseRPM(req.Extra)

	// 确定是否跳过混合渠道检查
	skipCheck := req.ConfirmMixedChannelRisk != nil && *req.ConfirmMixedChannelRisk

	hasUpdates := req.Name != "" ||
		req.ProxyID != nil ||
		req.Concurrency != nil ||
		req.Priority != nil ||
		req.RateMultiplier != nil ||
		req.LoadFactor != nil ||
		req.Status != "" ||
		req.Schedulable != nil ||
		req.GroupIDs != nil ||
		len(req.Credentials) > 0 ||
		len(req.Extra) > 0

	if !hasUpdates {
		response.BadRequest(c, "No updates provided")
		return
	}

	result, err := h.adminService.BulkUpdateAccounts(c.Request.Context(), &service.BulkUpdateAccountsInput{
		AccountIDs:            req.AccountIDs,
		Filters:               toServiceBulkUpdateAccountFilters(req.Filters),
		Name:                  req.Name,
		ProxyID:               req.ProxyID,
		Concurrency:           req.Concurrency,
		Priority:              req.Priority,
		RateMultiplier:        req.RateMultiplier,
		LoadFactor:            req.LoadFactor,
		Status:                req.Status,
		Schedulable:           req.Schedulable,
		GroupIDs:              req.GroupIDs,
		Credentials:           req.Credentials,
		Extra:                 req.Extra,
		SkipMixedChannelCheck: skipCheck,
	})
	if err != nil {
		var mixedErr *service.MixedChannelError
		if errors.As(err, &mixedErr) {
			c.JSON(409, gin.H{
				"error":   "mixed_channel_warning",
				"message": mixedErr.Error(),
				"details": gin.H{
					"group_id":         mixedErr.GroupID,
					"group_name":       mixedErr.GroupName,
					"current_platform": mixedErr.CurrentPlatform,
					"other_platform":   mixedErr.OtherPlatform,
				},
			})
			return
		}
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

func toServiceBulkUpdateAccountFilters(filters *BulkUpdateAccountFilters) *service.BulkUpdateAccountFilters {
	if filters == nil {
		return nil
	}
	return &service.BulkUpdateAccountFilters{
		Platform:    filters.Platform,
		Type:        filters.Type,
		Status:      filters.Status,
		Group:       filters.Group,
		Search:      filters.Search,
		PrivacyMode: filters.PrivacyMode,
	}
}

// ========== OAuth Handlers ==========

// GenerateAuthURLRequest represents the request for generating auth URL
type GenerateAuthURLRequest struct {
	ProxyID *int64 `json:"proxy_id"`
}

// GenerateAuthURL generates OAuth authorization URL with full scope
// POST /api/v1/admin/accounts/generate-auth-url
func (h *OAuthHandler) GenerateAuthURL(c *gin.Context) {
	var req GenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body
		req = GenerateAuthURLRequest{}
	}

	result, err := h.oauthService.GenerateAuthURL(c.Request.Context(), req.ProxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// GenerateSetupTokenURL generates OAuth authorization URL for setup token (inference only)
// POST /api/v1/admin/accounts/generate-setup-token-url
func (h *OAuthHandler) GenerateSetupTokenURL(c *gin.Context) {
	var req GenerateAuthURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body
		req = GenerateAuthURLRequest{}
	}

	result, err := h.oauthService.GenerateSetupTokenURL(c.Request.Context(), req.ProxyID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// ExchangeCodeRequest represents the request for exchanging auth code
type ExchangeCodeRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Code      string `json:"code" binding:"required"`
	ProxyID   *int64 `json:"proxy_id"`
}

// ExchangeCode exchanges authorization code for tokens
// POST /api/v1/admin/accounts/exchange-code
func (h *OAuthHandler) ExchangeCode(c *gin.Context) {
	var req ExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.ExchangeCode(c.Request.Context(), &service.ExchangeCodeInput{
		SessionID: req.SessionID,
		Code:      req.Code,
		ProxyID:   req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, tokenInfo)
}

// ExchangeSetupTokenCode exchanges authorization code for setup token
// POST /api/v1/admin/accounts/exchange-setup-token-code
func (h *OAuthHandler) ExchangeSetupTokenCode(c *gin.Context) {
	var req ExchangeCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.ExchangeCode(c.Request.Context(), &service.ExchangeCodeInput{
		SessionID: req.SessionID,
		Code:      req.Code,
		ProxyID:   req.ProxyID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, tokenInfo)
}

// CookieAuthRequest represents the request for cookie-based authentication
type CookieAuthRequest struct {
	SessionKey string `json:"code" binding:"required"` // Using 'code' field as sessionKey (frontend sends it this way)
	ProxyID    *int64 `json:"proxy_id"`
}

// CookieAuth performs OAuth using sessionKey (cookie-based auto-auth)
// POST /api/v1/admin/accounts/cookie-auth
func (h *OAuthHandler) CookieAuth(c *gin.Context) {
	var req CookieAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.CookieAuth(c.Request.Context(), &service.CookieAuthInput{
		SessionKey: req.SessionKey,
		ProxyID:    req.ProxyID,
		Scope:      "full",
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, tokenInfo)
}

// SetupTokenCookieAuth performs OAuth using sessionKey for setup token (inference only)
// POST /api/v1/admin/accounts/setup-token-cookie-auth
func (h *OAuthHandler) SetupTokenCookieAuth(c *gin.Context) {
	var req CookieAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	tokenInfo, err := h.oauthService.CookieAuth(c.Request.Context(), &service.CookieAuthInput{
		SessionKey: req.SessionKey,
		ProxyID:    req.ProxyID,
		Scope:      "inference",
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, tokenInfo)
}

// GetUsage handles getting account usage information
// GET /api/v1/admin/accounts/:id/usage?source=passive|active&force=true
func (h *AccountHandler) GetUsage(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	source := c.DefaultQuery("source", "active")
	force := c.Query("force") == "true"

	var usage *service.UsageInfo
	if source == "passive" {
		usage, err = h.accountUsageService.GetPassiveUsage(c.Request.Context(), accountID)
	} else {
		usage, err = h.accountUsageService.GetUsage(c.Request.Context(), accountID, force)
	}
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, usage)
}

// ClearRateLimit handles clearing account rate limit status
// POST /api/v1/admin/accounts/:id/clear-rate-limit
func (h *AccountHandler) ClearRateLimit(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	err = h.rateLimitService.ClearRateLimit(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// ResetQuota handles resetting account quota usage
// POST /api/v1/admin/accounts/:id/reset-quota
func (h *AccountHandler) ResetQuota(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	if err := h.adminService.ResetAccountQuota(c.Request.Context(), accountID); err != nil {
		response.InternalError(c, "Failed to reset account quota: "+err.Error())
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// GetTempUnschedulable handles getting temporary unschedulable status
// GET /api/v1/admin/accounts/:id/temp-unschedulable
func (h *AccountHandler) GetTempUnschedulable(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	state, err := h.rateLimitService.GetTempUnschedStatus(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	if state == nil || state.UntilUnix <= time.Now().Unix() {
		response.Success(c, gin.H{"active": false})
		return
	}

	response.Success(c, gin.H{
		"active": true,
		"state":  state,
	})
}

// ClearTempUnschedulable handles clearing temporary unschedulable status
// DELETE /api/v1/admin/accounts/:id/temp-unschedulable
func (h *AccountHandler) ClearTempUnschedulable(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	if err := h.rateLimitService.ClearTempUnschedulable(c.Request.Context(), accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Temp unschedulable cleared successfully"})
}

// GetTodayStats handles getting account today statistics
// GET /api/v1/admin/accounts/:id/today-stats
func (h *AccountHandler) GetTodayStats(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	stats, err := h.accountUsageService.GetTodayStats(c.Request.Context(), accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, stats)
}

// BatchTodayStatsRequest 批量今日统计请求体。
type BatchTodayStatsRequest struct {
	AccountIDs []int64 `json:"account_ids" binding:"required"`
}

// GetBatchTodayStats 批量获取多个账号的今日统计。
// POST /api/v1/admin/accounts/today-stats/batch
func (h *AccountHandler) GetBatchTodayStats(c *gin.Context) {
	var req BatchTodayStatsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	accountIDs := normalizeInt64IDList(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.Success(c, gin.H{"stats": map[string]any{}})
		return
	}

	cacheKey := buildAccountTodayStatsBatchCacheKey(accountIDs)
	if cached, ok := accountTodayStatsBatchCache.Get(cacheKey); ok {
		if cached.ETag != "" {
			c.Header("ETag", cached.ETag)
			c.Header("Vary", "If-None-Match")
			if ifNoneMatchMatched(c.GetHeader("If-None-Match"), cached.ETag) {
				c.Status(http.StatusNotModified)
				return
			}
		}
		c.Header("X-Snapshot-Cache", "hit")
		response.Success(c, cached.Payload)
		return
	}

	stats, err := h.accountUsageService.GetTodayStatsBatch(c.Request.Context(), accountIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	payload := gin.H{"stats": stats}
	cached := accountTodayStatsBatchCache.Set(cacheKey, payload)
	if cached.ETag != "" {
		c.Header("ETag", cached.ETag)
		c.Header("Vary", "If-None-Match")
	}
	c.Header("X-Snapshot-Cache", "miss")
	response.Success(c, payload)
}

// SetSchedulableRequest represents the request body for setting schedulable status
type SetSchedulableRequest struct {
	Schedulable bool `json:"schedulable"`
}

// SetSchedulable handles toggling account schedulable status
// POST /api/v1/admin/accounts/:id/schedulable
func (h *AccountHandler) SetSchedulable(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	var req SetSchedulableRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	account, err := h.adminService.SetAccountSchedulable(c.Request.Context(), accountID, req.Schedulable)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
}

// GetAvailableModels handles getting available models for an account
// GET /api/v1/admin/accounts/:id/models
func (h *AccountHandler) GetAvailableModels(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}

	// Handle OpenAI accounts
	if account.IsOpenAI() {
		// OpenAI 自动透传会绕过常规模型改写，测试/模型列表也应回落到默认模型集。
		if account.IsOpenAIPassthroughEnabled() {
			response.Success(c, openai.DefaultModels)
			return
		}

		mapping := account.GetModelMapping()
		if len(mapping) == 0 {
			response.Success(c, openai.DefaultModels)
			return
		}

		// Return mapped models
		var models []openai.Model
		for requestedModel := range mapping {
			var found bool
			for _, dm := range openai.DefaultModels {
				if dm.ID == requestedModel {
					models = append(models, dm)
					found = true
					break
				}
			}
			if !found {
				models = append(models, openai.Model{
					ID:          requestedModel,
					Object:      "model",
					Type:        "model",
					DisplayName: requestedModel,
				})
			}
		}
		response.Success(c, models)
		return
	}

	// Handle Gemini accounts
	if account.IsGemini() {
		// For OAuth accounts: return default Gemini models
		if account.IsOAuth() {
			response.Success(c, geminicli.DefaultModels)
			return
		}

		// For API Key accounts: return models based on model_mapping
		mapping := account.GetModelMapping()
		if len(mapping) == 0 {
			response.Success(c, geminicli.DefaultModels)
			return
		}

		var models []geminicli.Model
		for requestedModel := range mapping {
			var found bool
			for _, dm := range geminicli.DefaultModels {
				if dm.ID == requestedModel {
					models = append(models, dm)
					found = true
					break
				}
			}
			if !found {
				models = append(models, geminicli.Model{
					ID:          requestedModel,
					Type:        "model",
					DisplayName: requestedModel,
					CreatedAt:   "",
				})
			}
		}
		response.Success(c, models)
		return
	}

	// Handle Antigravity accounts: return Claude + Gemini models
	if account.Platform == service.PlatformAntigravity {
		// 直接复用 antigravity.DefaultModels()，与 /v1/models 端点保持同步
		response.Success(c, antigravity.DefaultModels())
		return
	}

	// Handle Claude/Anthropic accounts
	// For OAuth and Setup-Token accounts: return default models
	if account.IsOAuth() {
		response.Success(c, claude.DefaultModels)
		return
	}

	// For API Key accounts: return models based on model_mapping
	mapping := account.GetModelMapping()
	if len(mapping) == 0 {
		// No mapping configured, return default models
		response.Success(c, claude.DefaultModels)
		return
	}

	// Return mapped models (keys of the mapping are the available model IDs)
	var models []claude.Model
	for requestedModel := range mapping {
		// Try to find display info from default models
		var found bool
		for _, dm := range claude.DefaultModels {
			if dm.ID == requestedModel {
				models = append(models, dm)
				found = true
				break
			}
		}
		// If not found in defaults, create a basic entry
		if !found {
			models = append(models, claude.Model{
				ID:          requestedModel,
				Type:        "model",
				DisplayName: requestedModel,
				CreatedAt:   "",
			})
		}
	}

	response.Success(c, models)
}

// SyncUpstreamModels handles syncing live supported models from an account's upstream.
// POST /api/v1/admin/accounts/:id/models/sync-upstream
func (h *AccountHandler) SyncUpstreamModels(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}

	if h.accountTestService == nil {
		response.InternalError(c, "Account test service is not configured")
		return
	}

	models, err := h.accountTestService.FetchUpstreamSupportedModels(c.Request.Context(), account)
	if err != nil {
		var syncErr *service.UpstreamModelSyncError
		if errors.As(err, &syncErr) {
			switch syncErr.Kind {
			case service.UpstreamModelSyncErrorConfiguration, service.UpstreamModelSyncErrorUnsupported:
				response.BadRequest(c, syncErr.SafeMessage())
			default:
				slog.Warn("sync_upstream_models_failed", "account_id", accountID, "kind", syncErr.Kind)
				response.Error(c, http.StatusBadGateway, syncErr.SafeMessage())
			}
			return
		}

		slog.Warn("sync_upstream_models_failed", "account_id", accountID)
		response.Error(c, http.StatusBadGateway, "Failed to sync upstream models from upstream")
		return
	}

	response.Success(c, gin.H{"models": models})
}

// SetPrivacy handles setting privacy for a single OpenAI/Antigravity OAuth account
// POST /api/v1/admin/accounts/:id/set-privacy
func (h *AccountHandler) SetPrivacy(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}
	account, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}
	if account.Type != service.AccountTypeOAuth {
		response.BadRequest(c, "Only OAuth accounts support privacy setting")
		return
	}
	var mode string
	switch account.Platform {
	case service.PlatformOpenAI:
		mode = h.adminService.ForceOpenAIPrivacy(c.Request.Context(), account)
	case service.PlatformAntigravity:
		mode = h.adminService.ForceAntigravityPrivacy(c.Request.Context(), account)
	default:
		response.BadRequest(c, "Only OpenAI and Antigravity OAuth accounts support privacy setting")
		return
	}
	if mode == "" {
		response.BadRequest(c, "Cannot set privacy: missing access_token")
		return
	}
	// 从 DB 重新读取以确保返回最新状态
	updated, err := h.adminService.GetAccount(c.Request.Context(), accountID)
	if err != nil {
		// 隐私已设置成功但读取失败，回退到内存更新
		if account.Extra == nil {
			account.Extra = make(map[string]any)
		}
		account.Extra["privacy_mode"] = mode
		response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), account))
		return
	}
	response.Success(c, h.buildAccountResponseWithRuntime(c.Request.Context(), updated))
}

// RefreshTier handles refreshing Google One tier for a single account
// POST /api/v1/admin/accounts/:id/refresh-tier
func (h *AccountHandler) RefreshTier(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	ctx := c.Request.Context()
	account, err := h.adminService.GetAccount(ctx, accountID)
	if err != nil {
		response.NotFound(c, "Account not found")
		return
	}

	if account.Platform != service.PlatformGemini || account.Type != service.AccountTypeOAuth {
		response.BadRequest(c, "Only Gemini OAuth accounts support tier refresh")
		return
	}

	oauthType, _ := account.Credentials["oauth_type"].(string)
	if oauthType != "google_one" {
		response.BadRequest(c, "Only google_one OAuth accounts support tier refresh")
		return
	}

	tierID, extra, creds, err := h.geminiOAuthService.RefreshAccountGoogleOneTier(ctx, account)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	_, updateErr := h.adminService.UpdateAccount(ctx, accountID, &service.UpdateAccountInput{
		Credentials: creds,
		Extra:       extra,
	})
	if updateErr != nil {
		response.ErrorFrom(c, updateErr)
		return
	}

	response.Success(c, gin.H{
		"tier_id":             tierID,
		"storage_info":        extra,
		"drive_storage_limit": extra["drive_storage_limit"],
		"drive_storage_usage": extra["drive_storage_usage"],
		"updated_at":          extra["drive_tier_updated_at"],
	})
}

// BatchRefreshTierRequest represents batch tier refresh request
type BatchRefreshTierRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

// BatchRefreshTier handles batch refreshing Google One tier
// POST /api/v1/admin/accounts/batch-refresh-tier
func (h *AccountHandler) BatchRefreshTier(c *gin.Context) {
	var req BatchRefreshTierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req = BatchRefreshTierRequest{}
	}

	ctx := c.Request.Context()
	accounts := make([]*service.Account, 0)

	if len(req.AccountIDs) == 0 {
		allAccounts, _, err := h.adminService.ListAccounts(ctx, 1, 10000, "gemini", "oauth", "", "", 0, "", "name", "asc")
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		for i := range allAccounts {
			acc := &allAccounts[i]
			oauthType, _ := acc.Credentials["oauth_type"].(string)
			if oauthType == "google_one" {
				accounts = append(accounts, acc)
			}
		}
	} else {
		fetched, err := h.adminService.GetAccountsByIDs(ctx, req.AccountIDs)
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}

		for _, acc := range fetched {
			if acc == nil {
				continue
			}
			if acc.Platform != service.PlatformGemini || acc.Type != service.AccountTypeOAuth {
				continue
			}
			oauthType, _ := acc.Credentials["oauth_type"].(string)
			if oauthType != "google_one" {
				continue
			}
			accounts = append(accounts, acc)
		}
	}

	const maxConcurrency = 10
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	var mu sync.Mutex
	var successCount, failedCount int
	var errors []gin.H

	for _, account := range accounts {
		acc := account // 闭包捕获
		g.Go(func() error {
			_, extra, creds, err := h.geminiOAuthService.RefreshAccountGoogleOneTier(gctx, acc)
			if err != nil {
				mu.Lock()
				failedCount++
				errors = append(errors, gin.H{
					"account_id": acc.ID,
					"error":      err.Error(),
				})
				mu.Unlock()
				return nil
			}

			_, updateErr := h.adminService.UpdateAccount(gctx, acc.ID, &service.UpdateAccountInput{
				Credentials: creds,
				Extra:       extra,
			})

			mu.Lock()
			if updateErr != nil {
				failedCount++
				errors = append(errors, gin.H{
					"account_id": acc.ID,
					"error":      updateErr.Error(),
				})
			} else {
				successCount++
			}
			mu.Unlock()

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	results := gin.H{
		"total":   len(accounts),
		"success": successCount,
		"failed":  failedCount,
		"errors":  errors,
	}

	response.Success(c, results)
}

// GetAntigravityDefaultModelMapping 获取 Antigravity 平台的默认模型映射
// GET /api/v1/admin/accounts/antigravity/default-model-mapping
func (h *AccountHandler) GetAntigravityDefaultModelMapping(c *gin.Context) {
	response.Success(c, domain.DefaultAntigravityModelMapping)
}

// sanitizeExtraBaseRPM 对 extra map 中的 base_rpm 值进行范围校验和归一化。
// 负值归零，超过 10000 截断为 10000。extra 为 nil 或不含 base_rpm 时无操作。
func sanitizeExtraBaseRPM(extra map[string]any) {
	if extra == nil {
		return
	}
	raw, ok := extra["base_rpm"]
	if !ok {
		return
	}
	v := service.ParseExtraInt(raw)
	if v < 0 {
		v = 0
	} else if v > 10000 {
		v = 10000
	}
	extra["base_rpm"] = v
}

// BatchTestAccountsRequest 批量测试账号请求
// 不再接收模型列表——后端为每个账号独立从上游拉模型并自动挑选合适的模型。
type BatchTestAccountsRequest struct {
	AccountIDs  []int64 `json:"account_ids" binding:"required,min=1"`
	Concurrency int     `json:"concurrency"`
}

// BatchTestAccountsProgress SSE 进度事件
type BatchTestAccountsProgress struct {
	Type        string `json:"type"` // "result" | "done"
	AccountID   int64  `json:"account_id,omitempty"`
	Model       string `json:"model,omitempty"`
	ModelSource string `json:"model_source,omitempty"` // "upstream" / "fallback"，告诉前端模型是 live 拉的还是本地兜底
	Success     bool   `json:"success,omitempty"`
	// Outcome 区分三态，比 Success 布尔更细：
	//   - "success"     测试调通
	//   - "failed"      明确失败（鉴权/配置错，账号凭证被证伪，应暴露/可禁用）
	//   - "unavailable" 暂不可用（上游池子/网关临时问题，账号未被证伪，不应误判禁用）
	Outcome  string `json:"outcome,omitempty"`
	Error    string `json:"error,omitempty"`
	Current  int    `json:"current,omitempty"`
	Total    int    `json:"total,omitempty"`
	Attempts int    `json:"attempts,omitempty"`  // 实际尝试次数
	FellBack string `json:"fell_back,omitempty"` // 最终成功使用的回落模式（compact / 空）
}

// batchTestOutcome 三态结果。
const (
	batchTestOutcomeSuccess     = "success"
	batchTestOutcomeFailed      = "failed"
	batchTestOutcomeUnavailable = "unavailable"
)

// classifyBatchTestOutcome 把 runTestWithRetry 的最终错误归成三态。
//
//   - testErr == nil                                → success
//   - 候选模型不可用（model_not_found/无效模型 ID/Legacy）→ unavailable（模型下线，不是账号坏）
//   - 上游池子/网关临时不可用（没号/502/temporarily）   → unavailable（账号未被证伪）
//   - 明确鉴权/配置错（!isRetryableTestError）          → failed（账号凭证被证伪，应暴露）
//   - 其余（重试耗尽的瞬时错误，如反复 EOF/超时）         → unavailable（同样未证伪账号）
//
// 设计意图：只有真正能归咎到"这个账号配置/凭证"的错误才标 failed；其它一律 unavailable，
// 避免把"中转临时没号 / 网关 502 / 老模型下线（is not a valid model ID 等）"误报成账号失败、
// 进而被一键禁用。注意 model-unavailable / pool-unavailable 必须在 isRetryableTestError
// 之前判断——因为 "is not a valid model ID" 带 invalid_request_error 会被 isRetryableTestError
// 当成不可重试，若不先拦下会被错判成 failed。
func classifyBatchTestOutcome(testErr error) string {
	if testErr == nil {
		return batchTestOutcomeSuccess
	}
	// 模型本身不可用（候选全换完仍是这类）：上游下线/无渠道，不是账号凭证问题。
	if service.IsModelUnavailableTestError(testErr) {
		return batchTestOutcomeUnavailable
	}
	// 上游池子/网关临时不可用：没号、502/503/504、service temporarily unavailable 等。
	if service.IsUpstreamPoolUnavailableTestError(testErr) {
		return batchTestOutcomeUnavailable
	}
	// 明确的鉴权/配置错误：账号凭证/配置确实有问题，判失败。
	if !isRetryableTestError(testErr) {
		return batchTestOutcomeFailed
	}
	// 其余可重试错误走到这里说明重试/换模型都没救活：反复网络抖动等——未证伪账号本身。
	return batchTestOutcomeUnavailable
}

// isRetryableTestError 判断测试错误是否值得重试。
// - 网络层错误 / 5xx / 流式中断 → 可重试
// - 客户端错误 / 模型不支持 / 鉴权失败 → 不重试
func isRetryableTestError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// 明确不重试的错误模式
	nonRetryable := []string{
		"invalid_request_error",     // 上游说请求参数不对（比如模型不支持）
		"unauthorized",              // 401
		"forbidden",                 // 403
		"invalid_api_key",           // API key 不对
		"invalid refresh token",     // refresh token 失效
		"account not found",         // 账号不存在
		"no access token available", // OAuth 账号没 token
		"no api key available",      // apikey 账号没 key
		"missing_project_id",        // Antigravity 配置错
		"invalid base url",          // base_url 配置错
	}
	for _, needle := range nonRetryable {
		if strings.Contains(msg, needle) {
			return false
		}
	}

	// 默认可重试：网络错误、5xx、超时、流式中断等
	return true
}

// runTestWithRetry 带重试、模型切换和回落的测试执行。
//
// candidates 是按"测试优先级"排序的模型列表（最新模型在前，老/legacy 在后，见
// service.pickTestableModelCandidates）。逻辑：
//   - 依次尝试每个候选模型；
//   - 单个模型遇到"瞬时错误"（网络/5xx/流式中断）时，在该模型上做指数退避重试；
//   - 遇到"模型不可用"错误（model_not_found / No available channel / Bedrock Legacy / 非法模型 ID）时，
//     不在同一模型上空耗重试，直接切换到下一个候选模型；
//   - OpenAI 账号在 default 模式跑完仍失败时，对当前模型再试一次 compact 模式作为回落。
//
// 返回：最终错误（nil 表示成功）、实际尝试次数、回落到的模式（默认模式时为空）、最终使用的模型。
func (h *AccountHandler) runTestWithRetry(account *service.Account, accountID int64, candidates []string) (error, int, string, string) {
	const maxAttemptsPerModel = 3

	if len(candidates) == 0 {
		return fmt.Errorf("no testable model resolved for this account"), 0, "", ""
	}

	doOnce := func(model, mode string) error {
		dummyWriter := httptest.NewRecorder()
		dummyReq := httptest.NewRequest("POST", "/dummy", nil)
		testCtx, _ := gin.CreateTestContext(dummyWriter)
		testCtx.Request = dummyReq
		return h.accountTestService.TestAccountConnection(testCtx, accountID, model, "", mode)
	}

	totalAttempts := 0
	var lastErr error
	lastModel := candidates[0]

	for _, model := range candidates {
		lastModel = model
		var modelErr error

		// 阶段 1: 当前模型的 default 模式重试。
		for attempt := 1; attempt <= maxAttemptsPerModel; attempt++ {
			totalAttempts++
			err := doOnce(model, "")
			if err == nil {
				return nil, totalAttempts, "", model
			}
			modelErr = err
			lastErr = err

			// 模型本身不可用 → 不在同一模型上重试，直接换下一个候选。
			if service.IsModelUnavailableTestError(err) {
				break
			}
			// 其它不可重试错误（鉴权失败等）→ 整体放弃，换模型也没用。
			if !isRetryableTestError(err) {
				return lastErr, totalAttempts, "", model
			}
			if attempt < maxAttemptsPerModel {
				// 指数退避: 500ms, 1s, 2s
				time.Sleep(time.Duration(1<<(attempt-1)) * 500 * time.Millisecond)
			}
		}

		// 阶段 2: OpenAI 账号且当前模型 default 失败 → 尝试 compact 模式回落（同一模型）。
		// 仅当失败原因不是"模型不可用"时才值得试 compact（模型不存在时 compact 也没意义）。
		if account != nil && account.IsOpenAI() && !service.IsModelUnavailableTestError(modelErr) && isRetryableTestError(modelErr) {
			totalAttempts++
			compactErr := doOnce(model, service.AccountTestModeCompact)
			if compactErr == nil {
				return nil, totalAttempts, "compact", model
			}
			lastErr = compactErr
		}

		// 当前模型不可用 → 继续下一个候选；否则（已穷尽重试/compact）也继续兜底尝试下一个候选。
	}

	return lastErr, totalAttempts, "", lastModel
}

// maxTestModelCandidates batch-test 每个账号最多尝试的候选模型数。
// 取 3：覆盖"上游列表第一个是下线老模型"的情况（换 1-2 个新模型即可命中），
// 又不至于在一个账号上空耗太多上游请求。
const maxTestModelCandidates = 3

// pickTestModelCandidatesForAccount 为账号挑选一组按"测试优先级"排序的候选模型。
//
// 策略（见 service.pickTestableModelCandidates）：
//   - 最新模型在前（gpt-5.5 > gpt-5.4 > gpt-5.2；claude opus/sonnet 4.7 > 4.6 > 4.5），
//     明确老旧 / 多数上游已下线的模型（claude-2.x、claude-3-5-*-2024*、gpt-4*、gemini-1.x 等）降权排队尾。
//   - batch-test 依次尝试这些候选，遇到 model_not_found / No available channel / Legacy 就换下一个，
//     从根本上避免"测试挑到一个上游已经下线/无渠道的老模型"导致整片误判失败。
//
// 上游列表为空时回退到平台默认测试模型。
func pickTestModelCandidatesForAccount(account *service.Account, upstreamModels []string) []string {
	if account == nil {
		return nil
	}
	if candidates := service.PickTestableModelCandidates(upstreamModels, maxTestModelCandidates); len(candidates) > 0 {
		return candidates
	}
	// 兜底：用各平台的默认测试模型
	if account.IsOpenAI() {
		return []string{openai.DefaultTestModel}
	}
	if account.IsGemini() {
		return []string{geminicli.DefaultTestModel}
	}
	return []string{claude.DefaultTestModel}
}

// BatchTestAccounts 批量测试账号
// POST /api/v1/admin/accounts/batch-test
//
// 设计：
//   - 前端只传 account_ids（不传 model_ids）
//   - 后端为每个账号独立做：拉上游模型列表 → 挑一个合适的模型 → 执行测试
//   - 上游模型列表拉不到 → 该账号直接 failed，错误信息明确说"无法获取上游模型"
//   - 测试过程内部错误不会触发 gateway failover（这是后台运维操作，不是网关流量）
func (h *AccountHandler) BatchTestAccounts(c *gin.Context) {
	var req BatchTestAccountsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	// 默认并发 5
	if req.Concurrency <= 0 {
		req.Concurrency = 5
	}
	if req.Concurrency > 20 {
		req.Concurrency = 20
	}

	// 设置 SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ctx := c.Request.Context()
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.InternalError(c, "Streaming not supported")
		return
	}

	total := len(req.AccountIDs)
	current := 0
	var mu sync.Mutex

	// 写 SSE 事件
	emit := func(p BatchTestAccountsProgress) {
		data, _ := json.Marshal(p)
		fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		flusher.Flush()
	}

	// 并发执行测试
	sem := make(chan struct{}, req.Concurrency)
	var wg sync.WaitGroup

	for _, accountID := range req.AccountIDs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		wg.Add(1)
		go func(accID int64) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			// 1) 获取账号
			account, err := h.adminService.GetAccount(context.Background(), accID)

			report := func(model, modelSource, outcome, errMsg string, attempts int, fellBack string) {
				mu.Lock()
				current++
				progress := BatchTestAccountsProgress{
					Type:        "result",
					AccountID:   accID,
					Model:       model,
					ModelSource: modelSource,
					Success:     outcome == batchTestOutcomeSuccess,
					Outcome:     outcome,
					Error:       errMsg,
					Current:     current,
					Total:       total,
					Attempts:    attempts,
					FellBack:    fellBack,
				}
				mu.Unlock()
				emit(progress)
			}

			if err != nil || account == nil {
				report("", "", batchTestOutcomeFailed, "Account not found", 0, "")
				return
			}

			// 2) 拉模型列表：OAuth 账号在 live 不可用时自动回退到平台内置 known-good 列表，
			//    避免 OpenAI ChatGPT-OAuth / Claude OAuth (scope 无 models.read) 等账号被一刀切判失败。
			//    走到 fetchErr != nil 的，已是不予回退的错误（鉴权/配置/不支持），多半是真配错；
			//    但若上游列表请求本身是临时 5xx/网关问题没能回退，则标 unavailable 而非 failed。
			upstreamCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			testableModels, modelSource, fetchErr := h.accountTestService.FetchTestableModelsForAccount(upstreamCtx, account)
			cancel()
			if fetchErr != nil {
				fetchOutcome := batchTestOutcomeFailed
				if service.IsUpstreamPoolUnavailableTestError(fetchErr) {
					fetchOutcome = batchTestOutcomeUnavailable
				}
				report("", "", fetchOutcome, "Failed to fetch testable models: "+fetchErr.Error(), 0, "")
				return
			}
			if len(testableModels) == 0 {
				report("", "", batchTestOutcomeFailed, "No testable model resolved for this account", 0, "")
				return
			}

			// 3) 挑选一组按测试优先级排序的候选模型（最新在前，老/legacy 排队尾）。
			testCandidates := pickTestModelCandidatesForAccount(account, testableModels)
			if len(testCandidates) == 0 {
				report("", string(modelSource), batchTestOutcomeFailed, "No suitable test model found for this account", 0, "")
				return
			}

			// 4) 执行测试（候选模型逐个尝试 + 每模型重试 + OpenAI compact 回落）。
			//    遇到 model_not_found / No available channel / Bedrock Legacy 自动换下一个候选模型。
			testErr, attempts, fellBack, usedModel := h.runTestWithRetry(account, accID, testCandidates)

			errMsg := ""
			if testErr != nil {
				errMsg = testErr.Error()
			}
			report(usedModel, string(modelSource), classifyBatchTestOutcome(testErr), errMsg, attempts, fellBack)
		}(accountID)
	}

	wg.Wait()

	// 发送完成事件
	emit(BatchTestAccountsProgress{Type: "done", Current: total, Total: total})
}
