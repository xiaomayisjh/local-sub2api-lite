package admin

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestClassifyBatchTestOutcome 核验批量测试的三态归类，直接复刻真实日志里的报错串。
//
// 关键诉求（修复"误报"）：只有能归咎到账号凭证/配置的错误才判 failed；
// 上游池子临时问题、网关 5xx、老模型下线一律 unavailable，避免被一键禁用误伤好账号。
func TestClassifyBatchTestOutcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil_is_success",
			err:  nil,
			want: batchTestOutcomeSuccess,
		},
		// —— 上游池子 / 网关临时问题：unavailable ——
		{
			name: "no_available_accounts_is_unavailable",
			err:  errors.New(`API returned 503: {"error":{"message":"No available accounts: no available accounts"},"type":"voapi_error"}`),
			want: batchTestOutcomeUnavailable,
		},
		{
			name: "service_temporarily_unavailable_is_unavailable",
			err:  errors.New(`API returned 503: {"error":{"type":"<nil>","message":"Service temporarily unavailable (request id: x)"}}`),
			want: batchTestOutcomeUnavailable,
		},
		{
			name: "gateway_502_is_unavailable",
			err:  errors.New(`API returned 502: error code: 502`),
			want: batchTestOutcomeUnavailable,
		},
		// —— 模型下线 / 无渠道：unavailable（不是账号坏）——
		{
			name: "no_channel_for_model_is_unavailable",
			err:  errors.New(`API returned 503: {"error":{"type":"model_not_found","message":"No available channel for model claude-haiku-4-5 under group WindSurf (distributor)"}}`),
			want: batchTestOutcomeUnavailable,
		},
		{
			name: "chinese_no_channel_is_unavailable",
			err:  errors.New(`API returned 503: {"error":{"type":"model_not_found","message":"分组 awsy 下模型 claude-3-5-haiku-20241022 无可用渠道（distributor）"}}`),
			want: batchTestOutcomeUnavailable,
		},
		{
			// 关键回归点：is not a valid model ID 带 invalid_request_error，
			// 若不先于 isRetryableTestError 判 model-unavailable 会被错判成 failed。
			name: "invalid_model_id_is_unavailable_not_failed",
			err:  errors.New(`API returned 400: {"type":"error","error":{"type":"invalid_request_error","message":"model: claude-3-5-haiku-20241022 is not a valid model ID."}}`),
			want: batchTestOutcomeUnavailable,
		},
		{
			name: "bedrock_legacy_is_unavailable",
			err:  errors.New(`API returned 404: ResourceNotFoundException: This Model is marked by provider as Legacy`),
			want: batchTestOutcomeUnavailable,
		},
		// —— 账号凭证/配置错误：failed（应暴露、可禁用）——
		{
			name: "unauthorized_is_failed",
			err:  errors.New(`API returned 401: unauthorized`),
			want: batchTestOutcomeFailed,
		},
		{
			name: "forbidden_is_failed",
			err:  errors.New(`API returned 403: forbidden`),
			want: batchTestOutcomeFailed,
		},
		{
			name: "invalid_api_key_is_failed",
			err:  errors.New(`API returned 401: {"error":{"message":"invalid_api_key"}}`),
			want: batchTestOutcomeFailed,
		},
		{
			name: "invalid_base_url_is_failed",
			err:  errors.New(`invalid base url: missing scheme`),
			want: batchTestOutcomeFailed,
		},
		{
			name: "no_api_key_available_is_failed",
			err:  errors.New(`no api key available for this account`),
			want: batchTestOutcomeFailed,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, classifyBatchTestOutcome(tt.err), "err=%v", tt.err)
		})
	}
}
