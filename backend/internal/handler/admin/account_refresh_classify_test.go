package admin

import (
	"errors"
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// TestClassifyRefreshError 覆盖 OAuth refresh 失败的所有已知分支，确保
// handler 把上游/网络/配置错误翻译成正确的 HTTP code + reason，
// 前端能根据 reason 查 i18n 给出明确的"重新授权" vs "稍后再试" 提示。
func TestClassifyRefreshError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantReason string
	}{
		{
			name:       "invalid_grant_in_upstream_body",
			err:        errors.New(`token refresh failed: status 400, body: {"error": "invalid_grant", "error_description": "Refresh token not found or invalid"}`),
			wantCode:   http.StatusConflict,
			wantReason: refreshReasonRTRevoked,
		},
		{
			name:       "openai_invalid_refresh_token",
			err:        errors.New("openai oauth returned invalid_refresh_token"),
			wantCode:   http.StatusConflict,
			wantReason: refreshReasonRTRevoked,
		},
		{
			name:       "refresh_token_reused",
			err:        errors.New("upstream: refresh_token_reused"),
			wantCode:   http.StatusConflict,
			wantReason: refreshReasonRTReused,
		},
		{
			name:       "no_refresh_token_available",
			err:        errors.New("no refresh token available for account"),
			wantCode:   http.StatusBadRequest,
			wantReason: refreshReasonNoRefreshToken,
		},
		{
			name:       "missing_project_id",
			err:        errors.New("missing_project_id: cannot refresh gemini account"),
			wantCode:   http.StatusBadRequest,
			wantReason: refreshReasonMissingProjectID,
		},
		{
			name:       "invalid_client",
			err:        errors.New(`body: {"error":"invalid_client"}`),
			wantCode:   http.StatusConflict,
			wantReason: refreshReasonInvalidClient,
		},
		{
			name:       "access_denied",
			err:        errors.New("oauth returned access_denied"),
			wantCode:   http.StatusForbidden,
			wantReason: refreshReasonAccessDenied,
		},
		{
			name:       "network_unexpected_eof",
			err:        errors.New(`Post "https://auth.openai.com/oauth/token": unexpected EOF`),
			wantCode:   http.StatusBadGateway,
			wantReason: refreshReasonUpstreamUnavail,
		},
		{
			name:       "network_connection_refused",
			err:        errors.New("dial tcp: connection refused"),
			wantCode:   http.StatusBadGateway,
			wantReason: refreshReasonUpstreamUnavail,
		},
		{
			name:       "network_timeout",
			err:        errors.New("context deadline exceeded"),
			wantCode:   http.StatusBadGateway,
			wantReason: refreshReasonUpstreamUnavail,
		},
		{
			name: "openai_proxy_required_passthrough",
			err: infraerrors.New(http.StatusBadGateway, "OPENAI_OAUTH_PROXY_REQUIRED",
				"proxy required to reach OpenAI"),
			wantCode:   http.StatusBadGateway,
			wantReason: "OPENAI_OAUTH_PROXY_REQUIRED",
		},
		{
			name: "openai_oauth_request_failed_passthrough",
			err: infraerrors.New(http.StatusBadGateway, "OPENAI_OAUTH_REQUEST_FAILED",
				"upstream unreachable"),
			wantCode:   http.StatusBadGateway,
			wantReason: "OPENAI_OAUTH_REQUEST_FAILED",
		},
		{
			name:       "unknown_error_falls_back_to_bad_gateway",
			err:        errors.New("some weird unrecognized failure"),
			wantCode:   http.StatusBadGateway,
			wantReason: refreshReasonUnknown,
		},
		{
			name: "upstream_4xx_without_known_keyword",
			err: infraerrors.New(http.StatusBadRequest, "",
				"upstream replied 400 with unknown payload"),
			wantCode:   http.StatusBadRequest,
			wantReason: refreshReasonUpstreamRejected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyRefreshError(tt.err)
			require.NotNil(t, got, "classified error must not be nil")
			require.Equal(t, int32(tt.wantCode), got.Code, "HTTP code")
			require.Equal(t, tt.wantReason, got.Reason, "reason code")
			require.NotEmpty(t, got.Message, "message must not be empty (UI fallback)")
		})
	}
}

// TestClassifyRefreshError_NilInput 边界：nil 进 → nil 出，避免 handler 误把 nil 当成失败。
func TestClassifyRefreshError_NilInput(t *testing.T) {
	require.Nil(t, classifyRefreshError(nil))
}

// TestClassifyRefreshError_MessagePreservesUpstreamBody 回归：早先 extractRefreshErrorMessage
// 在 err 非 ApplicationError 时会被 FromError 套上 UnknownMessage="internal error" 的壳，
// 导致上游 JSON 错误体的真实 message 丢失。这里覆盖典型场景，确保 .Message 携带详细信息。
func TestClassifyRefreshError_MessagePreservesUpstreamBody(t *testing.T) {
	err := errors.New(`token refresh failed: status 400, body: {"error":{"message":"Refresh token not found or invalid","code":"invalid_grant"}}`)
	classified := classifyRefreshError(err)
	require.NotNil(t, classified)
	// 必须含有上游真实 message，而不是 infraerrors.UnknownMessage("internal error")
	require.Contains(t, classified.Message, "Refresh token not found or invalid")
	require.NotEqual(t, "internal error", classified.Message)
}

// TestClassifyCRSError 覆盖 CRS 同步/预览常见失败：账密错 vs 网络问题 vs 配置错，
// 确保返回正确 HTTP code + reason，让 admin UI 能给出准确指引。
func TestClassifyCRSError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantReason string
	}{
		{
			name:       "invalid_base_url",
			err:        errors.New("invalid base_url: missing scheme"),
			wantCode:   http.StatusBadRequest,
			wantReason: crsReasonInvalidBaseURL,
		},
		{
			name:       "missing_credentials",
			err:        errors.New("username and password are required"),
			wantCode:   http.StatusBadRequest,
			wantReason: crsReasonMissingCredentials,
		},
		{
			name:       "crs_auth_failed_4xx",
			err:        errors.New("crs login failed: status=401 body=Invalid credentials"),
			wantCode:   http.StatusUnauthorized,
			wantReason: crsReasonAuthFailed,
		},
		{
			name:       "crs_auth_failed_message",
			err:        errors.New("crs login failed: Invalid username or password"),
			wantCode:   http.StatusUnauthorized,
			wantReason: crsReasonAuthFailed,
		},
		{
			name:       "network_timeout",
			err:        errors.New("Get \"https://crs.example.com\": context deadline exceeded"),
			wantCode:   http.StatusBadGateway,
			wantReason: crsReasonUpstreamUnavail,
		},
		{
			name:       "create_http_client_failed",
			err:        errors.New("create http client failed: invalid proxy"),
			wantCode:   http.StatusBadGateway,
			wantReason: crsReasonUpstreamUnavail,
		},
		{
			name:       "fallback_unknown",
			err:        errors.New("weird unmapped error"),
			wantCode:   http.StatusBadGateway,
			wantReason: crsReasonUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyCRSError(tt.err)
			require.NotNil(t, got)
			require.Equal(t, int32(tt.wantCode), got.Code)
			require.Equal(t, tt.wantReason, got.Reason)
			require.NotEmpty(t, got.Message)
		})
	}
	require.Nil(t, classifyCRSError(nil))
}
