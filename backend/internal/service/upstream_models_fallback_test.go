package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

// TestFetchTestableModelsForAccount_OpenAIOAuthFallback OpenAI ChatGPT-OAuth 账号没有
// live 模型列表接口（buildOpenAIUpstreamModelsRequest 显式 unsupported），必须 fallback 到
// openai.DefaultModelIDs() 才能让批量测试不被一刀切判失败。
func TestFetchTestableModelsForAccount_OpenAIOAuthFallback(t *testing.T) {
	t.Parallel()

	svc := &AccountTestService{
		httpUpstream: &httpUpstreamRecorder{}, // 不会被调用——OpenAI OAuth 直接走 fallback
		cfg:          upstreamModelSyncTestConfig(),
	}
	account := &Account{
		ID:       101,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "oauth-access-token",
		},
	}

	models, source, err := svc.FetchTestableModelsForAccount(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, TestableModelsSourceFallback, source)
	require.NotEmpty(t, models, "OpenAI OAuth fallback must return known models")
	require.Equal(t, openai.DefaultModelIDs(), models)
}

// TestFetchTestableModelsForAccount_AnthropicOAuthFallbackOn401 Anthropic OAuth 在 claude-code scope 下
// 调 /v1/models 通常 401/403，旧逻辑直接判失败；新逻辑必须 fallback 到 claude.DefaultModelIDs()。
func TestFetchTestableModelsForAccount_AnthropicOAuthFallbackOn401(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"type":"forbidden","message":"oauth scope mismatch"}}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}
	account := &Account{
		ID:       102,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "anthropic-oauth-token",
		},
	}

	models, source, err := svc.FetchTestableModelsForAccount(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, TestableModelsSourceFallback, source)
	require.Equal(t, claude.DefaultModelIDs(), models)
}

// TestFetchTestableModelsForAccount_AnthropicOAuthUpstreamWhenAvailable
// 当 Anthropic OAuth 上游能正常返回模型列表时（部分服务端宽松的 OAuth scope），不应 fallback。
func TestFetchTestableModelsForAccount_AnthropicOAuthUpstreamWhenAvailable(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"data":[{"id":"claude-sonnet-4-5-20250929"},{"id":"claude-opus-4-7"}]}`,
		)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}
	account := &Account{
		ID:       103,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token": "anthropic-oauth-token",
		},
	}

	models, source, err := svc.FetchTestableModelsForAccount(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, TestableModelsSourceUpstream, source)
	require.Equal(t, []string{"claude-opus-4-7", "claude-sonnet-4-5-20250929"}, models)
}

// TestFetchTestableModelsForAccount_APIKeyNoFallback API-Key 账号 live 失败必须直接报错，
// 不允许 fallback——否则会掩盖 base_url / api_key 配置错误。
func TestFetchTestableModelsForAccount_APIKeyNoFallback(t *testing.T) {
	t.Parallel()

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusUnauthorized,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":"invalid api key"}`)),
	}}
	svc := &AccountTestService{
		httpUpstream: upstream,
		cfg:          upstreamModelSyncTestConfig(),
	}
	account := &Account{
		ID:       104,
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

// TestFallbackTestableModelIDs_PlatformCoverage 直接校验 fallback 表对各平台的覆盖。
func TestFallbackTestableModelIDs_PlatformCoverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		account   *Account
		wantEmpty bool
	}{
		{
			name:    "anthropic_oauth",
			account: &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth},
		},
		{
			name:    "anthropic_setup_token",
			account: &Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken},
		},
		{
			name:    "openai_oauth",
			account: &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth},
		},
		{
			name:    "gemini_oauth",
			account: &Account{Platform: PlatformGemini, Type: AccountTypeOAuth},
		},
		{
			name:    "antigravity_oauth",
			account: &Account{Platform: PlatformAntigravity, Type: AccountTypeOAuth},
		},
		{
			name:      "nil_account",
			account:   nil,
			wantEmpty: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := fallbackTestableModelIDs(tt.account)
			if tt.wantEmpty {
				require.Empty(t, got)
				return
			}
			require.NotEmpty(t, got, "fallback for %s must not be empty", tt.name)
			for _, id := range got {
				require.NotEmpty(t, strings.TrimSpace(id))
			}
		})
	}
}
