package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestValidateDataHeader_OptionalProxiesAndAccounts 核验 proxies/accounts 缺省不再报错。
func TestValidateDataHeader_OptionalProxiesAndAccounts(t *testing.T) {
	t.Parallel()

	// 二者皆缺（nil）：合法。
	require.NoError(t, validateDataHeader(DataPayload{}))

	// 仅含账号、缺 proxies：合法。
	require.NoError(t, validateDataHeader(DataPayload{
		Accounts: []DataAccount{{Name: "a"}},
	}))

	// 仅含代理、缺 accounts：合法。
	require.NoError(t, validateDataHeader(DataPayload{
		Proxies: []DataProxy{{Host: "h"}},
	}))

	// 非法 type / version 仍应被拒。
	require.Error(t, validateDataHeader(DataPayload{Type: "bogus"}))
	require.Error(t, validateDataHeader(DataPayload{Version: 999}))
}

// TestNormalizeAccountType 核验类型别名归一化（外部实例兼容）。
func TestNormalizeAccountType(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"api_key":         service.AccountTypeAPIKey,         // "apikey"
		"API_KEY":         service.AccountTypeAPIKey,         // 大小写
		"api-key":         service.AccountTypeAPIKey,         //
		"  apikey  ":      service.AccountTypeAPIKey,         // 去空白
		"setup_token":     service.AccountTypeSetupToken,     // "setup-token"
		"setuptoken":      service.AccountTypeSetupToken,     //
		"serviceaccount":  service.AccountTypeServiceAccount, // "service_account"
		"service-account": service.AccountTypeServiceAccount, //
		"service_account": service.AccountTypeServiceAccount, // 已规范
		"oauth":           service.AccountTypeOAuth,          // 原样
		"bedrock":         service.AccountTypeBedrock,        // 原样
		"unknown-xyz":     "unknown-xyz",                     // 无法识别原样返回（小写）
	}
	for in, want := range cases {
		require.Equalf(t, want, normalizeAccountType(in), "normalizeAccountType(%q)", in)
	}
}

// TestValidateDataAccount_AcceptsAllRealTypes 核验六种真实类型均通过校验，
// 且归一化后的别名（api_key→apikey）也能通过。
func TestValidateDataAccount_AcceptsAllRealTypes(t *testing.T) {
	t.Parallel()

	creds := map[string]any{"k": "v"}
	for _, typ := range []string{
		service.AccountTypeOAuth,
		service.AccountTypeSetupToken,
		service.AccountTypeAPIKey,
		service.AccountTypeUpstream,
		service.AccountTypeBedrock,
		service.AccountTypeServiceAccount,
	} {
		err := validateDataAccount(DataAccount{Name: "n", Platform: "anthropic", Type: typ, Credentials: creds})
		require.NoErrorf(t, err, "type %q must be valid", typ)
	}

	// 归一化后的别名（导入循环会先 normalize 再 validate）。
	require.NoError(t, validateDataAccount(DataAccount{
		Name: "n", Platform: "anthropic", Type: normalizeAccountType("api_key"), Credentials: creds,
	}))
	require.NoError(t, validateDataAccount(DataAccount{
		Name: "n", Platform: "gemini", Type: normalizeAccountType("serviceaccount"), Credentials: creds,
	}))

	// 真正非法的类型仍被拒。
	require.Error(t, validateDataAccount(DataAccount{
		Name: "n", Platform: "anthropic", Type: "totally-bogus", Credentials: creds,
	}))
}
