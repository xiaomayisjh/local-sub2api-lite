package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// hasOverlap 判断两个账号是否被判为重复（任意身份 key 命中）。
func hasOverlap(a, b *Account) bool {
	ka := AccountIdentityKeys(a)
	kb := AccountIdentityKeys(b)
	set := make(map[string]struct{}, len(ka))
	for _, k := range ka {
		set[k] = struct{}{}
	}
	for _, k := range kb {
		if _, ok := set[k]; ok {
			return true
		}
	}
	return false
}

// TestAccountIdentityKeys_ScopedByPlatformAndType 核验"仅同 platform+type 内判重"：
// 即便凭证内容一样，platform 或 type 不同也不应判为重复。
func TestAccountIdentityKeys_ScopedByPlatformAndType(t *testing.T) {
	t.Parallel()

	// 同一把 api_key，但一个 openai-apikey、一个 anthropic-apikey：不应重复。
	oa := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-same"}}
	an := &Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-same"}}
	require.False(t, hasOverlap(oa, an), "same api_key across different platforms must NOT be duplicate")

	// 同 platform 但不同 type（oauth vs apikey）：不应重复。
	oauthAcc := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "tok-x"}}
	apikeyAcc := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "tok-x"}}
	require.False(t, hasOverlap(oauthAcc, apikeyAcc), "oauth vs apikey on same platform must NOT be duplicate")
}

// TestAccountIdentityKeys_OpenAIOAuth 复刻 codex 身份：account_id / user_id / email / access_token。
func TestAccountIdentityKeys_OpenAIOAuth(t *testing.T) {
	t.Parallel()

	// account_id 相同即重复，即便其它字段不同。
	a := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{
		"chatgpt_account_id": "acc-1", "access_token": "tokA",
	}}
	b := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{
		"chatgpt_account_id": "acc-1", "access_token": "tokB-different",
	}}
	require.True(t, hasOverlap(a, b), "same chatgpt_account_id must be duplicate")

	// 无 account_id/user_id 时回落到 email。
	c := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{
		"email": "User@Example.com",
	}}
	d := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{
		"email": "user@example.com", // 大小写归一化
	}}
	require.True(t, hasOverlap(c, d), "same email (case-insensitive) must be duplicate")

	// 完全不同的人不应重复。
	e := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "acc-x"}}
	f := &Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "acc-y"}}
	require.False(t, hasOverlap(e, f))
}

// TestAccountIdentityKeys_AnthropicOAuthPrefersRefresh anthropic OAuth 用 refresh_token 判重（access 会轮换）。
func TestAccountIdentityKeys_AnthropicOAuthPrefersRefresh(t *testing.T) {
	t.Parallel()

	// 同 refresh_token、不同 access_token：仍应判重（access 轮换不影响身份）。
	a := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{
		"refresh_token": "rt-1", "access_token": "at-old",
	}}
	b := &Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{
		"refresh_token": "rt-1", "access_token": "at-new",
	}}
	require.True(t, hasOverlap(a, b), "same refresh_token must be duplicate even if access_token rotated")

	// setup-token 同样走 token 规则。
	st1 := &Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken, Credentials: map[string]any{"refresh_token": "rt-st"}}
	st2 := &Account{Platform: PlatformAnthropic, Type: AccountTypeSetupToken, Credentials: map[string]any{"refresh_token": "rt-st"}}
	require.True(t, hasOverlap(st1, st2))
}

// TestAccountIdentityKeys_APIKeyBaseURLNormalized api_key + 归一化 base_url。
func TestAccountIdentityKeys_APIKeyBaseURLNormalized(t *testing.T) {
	t.Parallel()

	// 同 api_key + base_url 仅尾斜杠/大小写差异：判重。
	a := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "sk-1", "base_url": "https://API.x.com/v1/",
	}}
	b := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "sk-1", "base_url": "https://api.x.com/v1",
	}}
	require.True(t, hasOverlap(a, b), "base_url trailing slash / case must be normalized")

	// 同 api_key 但不同 base_url（不同上游）：不应重复。
	c := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "sk-1", "base_url": "https://a.com/v1",
	}}
	d := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
		"api_key": "sk-1", "base_url": "https://b.com/v1",
	}}
	require.False(t, hasOverlap(c, d), "same key but different upstream must NOT be duplicate")

	// 没有 api_key 的脏数据：无身份 key，永不判重。
	empty := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{}}
	require.Empty(t, AccountIdentityKeys(empty))
}

// TestAccountIdentityKeys_Bedrock sigv4 用 AK+region；apikey 模式用 api_key。
func TestAccountIdentityKeys_Bedrock(t *testing.T) {
	t.Parallel()

	a := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{
		"auth_mode": "sigv4", "aws_access_key_id": "AKIA1", "aws_region": "us-east-1", "aws_secret_access_key": "sec",
	}}
	b := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{
		"auth_mode": "sigv4", "aws_access_key_id": "AKIA1", "aws_region": "us-east-1", "aws_secret_access_key": "sec",
	}}
	require.True(t, hasOverlap(a, b), "same AK+region+secret must be duplicate")

	// 不同 region 视为不同配置。
	c := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{
		"aws_access_key_id": "AKIA1", "aws_region": "us-west-2",
	}}
	require.False(t, hasOverlap(a, c), "different region must NOT be duplicate")

	// apikey 模式 Bedrock。
	k1 := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{"auth_mode": "apikey", "api_key": "bk-1"}}
	k2 := &Account{Platform: PlatformAnthropic, Type: AccountTypeBedrock, Credentials: map[string]any{"auth_mode": "apikey", "api_key": "bk-1"}}
	require.True(t, hasOverlap(k1, k2))
}

// TestAccountIdentityKeys_ServiceAccount project_id + private_key_id 唯一标识 SA 密钥。
func TestAccountIdentityKeys_ServiceAccount(t *testing.T) {
	t.Parallel()

	a := &Account{Platform: PlatformGemini, Type: AccountTypeServiceAccount, Credentials: map[string]any{
		"project_id": "proj-1", "private_key_id": "pk-abc", "client_email": "svc@proj.iam",
	}}
	b := &Account{Platform: PlatformGemini, Type: AccountTypeServiceAccount, Credentials: map[string]any{
		"project_id": "proj-1", "private_key_id": "pk-abc",
	}}
	require.True(t, hasOverlap(a, b), "same project+private_key_id must be duplicate")

	// 不同 SA 密钥。
	c := &Account{Platform: PlatformGemini, Type: AccountTypeServiceAccount, Credentials: map[string]any{
		"project_id": "proj-1", "private_key_id": "pk-zzz",
	}}
	require.False(t, hasOverlap(a, c))
}

// TestGroupDuplicateAccounts 核验多选去重的归组：含 >=2 才成组，最早创建排第一（由 handler 标 SuggestKeep）。
func TestGroupDuplicateAccounts(t *testing.T) {
	t.Parallel()

	accounts := []*Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-dup", "base_url": "https://x.com/v1"}},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-dup", "base_url": "https://x.com/v1/"}}, // 与 #1 重复（尾斜杠归一化）
		{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-unique"}},                               // 独一份
		{ID: 4, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{"refresh_token": "rt-1"}},
		{ID: 5, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{"refresh_token": "rt-1"}}, // 与 #4 重复
	}

	groups := GroupDuplicateAccounts(accounts)
	require.Len(t, groups, 2, "expect exactly two duplicate groups")

	// 收集每组的 ID 集合，断言 {1,2} 与 {4,5} 各成一组，#3 不在任何组。
	groupSets := make([]map[int64]bool, 0, len(groups))
	for _, g := range groups {
		set := map[int64]bool{}
		for _, id := range g.AccountIDs {
			set[id] = true
		}
		groupSets = append(groupSets, set)
	}

	hasGroup := func(ids ...int64) bool {
		for _, set := range groupSets {
			if len(set) != len(ids) {
				continue
			}
			all := true
			for _, id := range ids {
				if !set[id] {
					all = false
					break
				}
			}
			if all {
				return true
			}
		}
		return false
	}

	require.True(t, hasGroup(1, 2), "ids 1 and 2 must be grouped (normalized base_url): %+v", groups)
	require.True(t, hasGroup(4, 5), "ids 4 and 5 must be grouped (same refresh_token): %+v", groups)

	// #3 独一份，不应出现在任何组。
	for _, set := range groupSets {
		require.False(t, set[3], "unique account #3 must not be in any duplicate group")
	}
}

// TestGroupDuplicateAccounts_TransitiveUnion 三个账号通过不同 key 传递相连，应归为同一组。
func TestGroupDuplicateAccounts_TransitiveUnion(t *testing.T) {
	t.Parallel()

	// A 与 B 共享 account_id；B 与 C 共享 access_token；A 与 C 无直接共享 —— 但应传递归为一组。
	accounts := []*Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "acc-1", "access_token": "tok-A"}},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "acc-1", "access_token": "tok-B"}},
		{ID: 3, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"chatgpt_account_id": "acc-3", "access_token": "tok-B"}},
	}

	groups := GroupDuplicateAccounts(accounts)
	require.Len(t, groups, 1, "transitively connected accounts must form one group: %+v", groups)
	require.Len(t, groups[0].AccountIDs, 3)
}

// TestGroupDuplicateAccounts_NoDuplicates 全部互不重复时返回空。
func TestGroupDuplicateAccounts_NoDuplicates(t *testing.T) {
	t.Parallel()

	accounts := []*Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-a"}},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-b"}},
		{ID: 3, Platform: PlatformAnthropic, Type: AccountTypeOAuth, Credentials: map[string]any{"refresh_token": "rt-c"}},
	}
	groups := GroupDuplicateAccounts(accounts)
	require.Empty(t, groups)
}
