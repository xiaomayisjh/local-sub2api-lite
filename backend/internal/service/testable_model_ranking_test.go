package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

// TestRankTestableModels_PrefersNewestNonLegacy 核验"新模型在前、老/legacy 排队尾"，
// 直接复刻用户日志里挑错模型的场景。
func TestRankTestableModels_PrefersNewestNonLegacy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input []string
		// wantFirst 断言重排后的第一个候选（batch-test 优先测的模型）。
		wantFirst string
		// wantLastContains 断言这些 legacy 模型被排到队尾（在 wantFirst 之后）。
		wantLastContains []string
	}{
		{
			// gpt-5.2 字典序在 gpt-5.4/5.5 之前 —— 旧逻辑会挑 5.2（日志里 "No available channel for gpt-5.2"）。
			name:      "openai_picks_newest_not_alphabetical_first",
			input:     []string{"gpt-5.2", "gpt-5.4", "gpt-5.5", "gpt-5.3-codex"},
			wantFirst: "gpt-5.5",
		},
		{
			// claude-2.0 / claude-3-5-haiku-20241022 字典序最靠前 —— 旧逻辑挑到下线老模型。
			name:             "claude_legacy_sinks_to_tail",
			input:            []string{"claude-2.0", "claude-3-5-haiku-20241022", "claude-haiku-4-5-20251001", "claude-opus-4-7"},
			wantFirst:        "claude-opus-4-7",
			wantLastContains: []string{"claude-2.0", "claude-3-5-haiku-20241022"},
		},
		{
			name:             "gpt4_legacy_after_gpt5",
			input:            []string{"gpt-4o", "gpt-4", "gpt-5.4", "gpt-3.5-turbo"},
			wantFirst:        "gpt-5.4",
			wantLastContains: []string{"gpt-4o", "gpt-4", "gpt-3.5-turbo"},
		},
		{
			name:      "gemini_newest_first",
			input:     []string{"gemini-1.5-flash", "gemini-2.5-pro", "gemini-3-pro-preview", "gemini-3.1-pro-preview"},
			wantFirst: "gemini-3.1-pro-preview",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ranked := rankTestableModels(tt.input)
			require.NotEmpty(t, ranked)
			require.Equal(t, tt.wantFirst, ranked[0], "ranked=%v", ranked)

			// legacy 模型不应被丢弃（仅降权），且必须排在 wantFirst 之后。
			firstIdx := indexOf(ranked, tt.wantFirst)
			for _, legacy := range tt.wantLastContains {
				idx := indexOf(ranked, legacy)
				require.GreaterOrEqual(t, idx, 0, "legacy model %q must be kept, not dropped: %v", legacy, ranked)
				require.Greater(t, idx, firstIdx, "legacy model %q must rank after %q: %v", legacy, tt.wantFirst, ranked)
			}
		})
	}
}

// TestRankTestableModels_DoesNotDropModels 确认重排只换顺序，不增不减。
func TestRankTestableModels_DoesNotDropModels(t *testing.T) {
	t.Parallel()

	input := []string{"claude-2.0", "gpt-5.4", "gemini-1.5-pro", "claude-opus-4-7", "gpt-4o"}
	ranked := rankTestableModels(input)
	require.ElementsMatch(t, input, ranked)
}

// TestPickTestableModelCandidates_LimitsCount 候选数量被截断到上限，且仍是最新优先。
func TestPickTestableModelCandidates_LimitsCount(t *testing.T) {
	t.Parallel()

	input := []string{"claude-2.0", "claude-3-5-haiku-20241022", "claude-haiku-4-5-20251001", "claude-sonnet-4-6", "claude-opus-4-7"}
	got := PickTestableModelCandidates(input, 3)
	require.Len(t, got, 3)
	require.Equal(t, "claude-opus-4-7", got[0])
	// 截断后的 3 个里不应包含最老的 claude-2.0（它被排到队尾、超出 maxCandidates）。
	require.NotContains(t, got, "claude-2.0")
}

// TestIsModelUnavailableTestError_RealWorldMessages 用真实日志里的报错串验证模型不可用识别。
func TestIsModelUnavailableTestError_RealWorldMessages(t *testing.T) {
	t.Parallel()

	unavailable := []string{
		`API returned 503: {"error":{"code":"model_not_found","message":"No available channel for model gpt-5.2-openai-compact under group gpt-plus"}}`,
		`API returned 400: {"type":"error","error":{"type":"invalid_request_error","message":"model: claude-3-5-haiku-20241022 is not a valid model ID."}}`,
		`API returned 404: {"error":{"message":"ResourceNotFoundException: Access denied. This Model is marked by provider as Legacy"}}`,
		`{"error":{"message":"unknown model gpt-5.4-openai-compact"}}`,
	}
	for _, msg := range unavailable {
		require.True(t, IsModelUnavailableTestError(errors.New(msg)), "should detect model-unavailable: %s", msg)
	}

	notUnavailable := []string{
		`API returned 502: error code: 502`,
		`API returned 503: {"error":{"message":"Service temporarily unavailable"}}`,
		`Failed to request upstream model list: context deadline exceeded`,
		`API returned 401: unauthorized`,
		``,
	}
	for _, msg := range notUnavailable {
		require.False(t, IsModelUnavailableTestError(errorOrNil(msg)), "should NOT be model-unavailable: %q", msg)
	}
}

// TestIsUpstreamPoolUnavailableTestError_RealWorldMessages 验证"上游池子临时不可用"识别，
// 直接复刻新日志里被误报成失败的那批报错。
func TestIsUpstreamPoolUnavailableTestError_RealWorldMessages(t *testing.T) {
	t.Parallel()

	poolUnavailable := []string{
		`API returned 503: {"error":{"message":"No available accounts: no available accounts"},"type":"voapi_error","rid":"2061034542302498816"}`,
		`API returned 503: {"error":{"type":"<nil>","message":"Service temporarily unavailable (request id: 202605311038385991325978268d9d6KKcyaKpM)"},"type":"error"}`,
		`API returned 502: error code: 502`,
		`API returned 503: {"error":{"type":"model_not_found","message":"分组 awsy 下模型 claude-3-5-haiku-20241022 无可用渠道（distributor）"},"type":"error"}`,
		`API returned 503: {"error":{"type":"model_not_found","message":"No available channel for model claude-haiku-4-5 under group Claude逆向-WindSurf (distributor)"},"type":"error"}`,
	}
	for _, msg := range poolUnavailable {
		require.True(t, IsUpstreamPoolUnavailableTestError(errors.New(msg)), "should detect pool-unavailable: %s", msg)
	}

	notPool := []string{
		// 明确鉴权/参数错误：账号配置问题，不是池子临时不可用。
		`API returned 400: {"type":"error","error":{"type":"invalid_request_error","message":"model: claude-3-5-haiku-20241022 is not a valid model ID."}}`,
		`API returned 401: unauthorized`,
		`API returned 403: forbidden`,
		``,
	}
	for _, msg := range notPool {
		require.False(t, IsUpstreamPoolUnavailableTestError(errorOrNil(msg)), "should NOT be pool-unavailable: %q", msg)
	}
}

// TestIsTransientUpstreamModelError 核验 API-Key fallback 的"瞬时 vs 配置错误"分类。
func TestIsTransientUpstreamModelError(t *testing.T) {
	t.Parallel()

	transient := []error{
		newUpstreamModelSyncUpstreamError("Upstream model list request failed with HTTP 503", fmt.Errorf("upstream model list returned HTTP 503")),
		newUpstreamModelSyncUpstreamError("Upstream model list request failed with HTTP 502", fmt.Errorf("HTTP 502")),
		newUpstreamModelSyncUpstreamError("Upstream model list request failed with HTTP 500", fmt.Errorf("HTTP 500")),
		// HTTP 404：中转没有 /v1/models 端点（Deepseek 等）。仍 fallback 到平台默认模型去测。
		newUpstreamModelSyncUpstreamError("Upstream model list request failed with HTTP 404", fmt.Errorf("upstream model list returned HTTP 404")),
		newUpstreamModelSyncUpstreamError("Failed to request upstream model list", fmt.Errorf("context deadline exceeded")),
		newUpstreamModelSyncUpstreamError("Failed to request upstream model list", fmt.Errorf(`Get "https://x/v1/models": EOF`)),
		newUpstreamModelSyncUpstreamError("Failed to request upstream model list", fmt.Errorf("dial tcp: lookup body-stripper-ai01: no such host")),
	}
	for _, err := range transient {
		require.True(t, isTransientUpstreamModelError(err), "should be transient: %v", err)
	}

	nonTransient := []error{
		newUpstreamModelSyncUpstreamError("Upstream model list request failed with HTTP 401", fmt.Errorf("HTTP 401")),
		newUpstreamModelSyncUpstreamError("Upstream model list request failed with HTTP 403", fmt.Errorf("HTTP 403")),
		newUpstreamModelSyncConfigError("Invalid OpenAI base URL", fmt.Errorf("bad url")),
		newUpstreamModelSyncConfigError("No OpenAI API key is available", nil),
		newUpstreamModelSyncUnsupportedError("Unsupported platform for upstream model sync: x", nil),
		nil,
	}
	for _, err := range nonTransient {
		require.False(t, isTransientUpstreamModelError(err), "should NOT be transient: %v", err)
	}
}

// TestFetchTestableModelsForAccount_APIKeyTransientFallback API-Key 账号在 5xx 时应 fallback 到默认模型，
// 让批量测试有机会用真实模型跑通，而不是直接 "Failed to fetch testable models"。
func TestFetchTestableModelsForAccount_APIKeyTransientFallback(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"upstream busy"}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}
	account := &Account{
		ID:       201,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-real",
			"base_url": "https://openai.example.com/v1",
		},
	}

	models, source, err := svc.FetchTestableModelsForAccount(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, TestableModelsSourceFallback, source)
	require.Equal(t, openai.DefaultModelIDs(), models)
}

// TestFetchTestableModelsForAccount_APIKeyAuthErrorNoFallback API-Key 账号 401/403 仍硬失败（不掩盖配错）。
func TestFetchTestableModelsForAccount_APIKeyAuthErrorNoFallback(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"forbidden"}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}
	account := &Account{
		ID:       202,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-bad",
			"base_url": "https://openai.example.com/v1",
		},
	}

	_, source, err := svc.FetchTestableModelsForAccount(context.Background(), account)
	require.Error(t, err)
	require.Equal(t, TestableModelsSource(""), source)
}

// TestFetchTestableModelsForAccount_APIKeyNetworkErrorFallback 网络层错误（EOF/超时/DNS）也应 fallback。
func TestFetchTestableModelsForAccount_APIKeyNetworkErrorFallback(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{err: fmt.Errorf(`Get "https://x/v1/models": EOF`)}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}
	account := &Account{
		ID:       203,
		Platform: PlatformAnthropic,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-real",
			"base_url": "https://claude.example.com",
		},
	}

	models, source, err := svc.FetchTestableModelsForAccount(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, TestableModelsSourceFallback, source)
	require.Equal(t, claude.DefaultModelIDs(), models)
}

// TestFetchTestableModelsForAccount_APIKey404FallbackPlatformCorrect 复刻 Deepseek：
// 上游没有 /v1/models 端点返回 404。API-Key 账号应 fallback，且用账号实际 platform 的默认模型
// （OpenAI 协议中转 → gpt-*，不会错用 claude-*）。
func TestFetchTestableModelsForAccount_APIKey404FallbackPlatformCorrect(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"not found"}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}
	account := &Account{
		ID:       204,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-deepseek",
			"base_url": "https://deepseek.example.com/v1",
		},
	}

	models, source, err := svc.FetchTestableModelsForAccount(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, TestableModelsSourceFallback, source)
	require.Equal(t, openai.DefaultModelIDs(), models, "404 fallback must use the account's own platform defaults, not claude")
}

func indexOf(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}

func errorOrNil(msg string) error {
	if msg == "" {
		return nil
	}
	return errors.New(msg)
}
