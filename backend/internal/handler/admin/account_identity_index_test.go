package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// TestAccountIdentityIndex_FindDuplicate 核验导入去重索引：
// 与已登记账号（库内 / 本批先前项）任意身份 key 命中即判重复。
func TestAccountIdentityIndex_FindDuplicate(t *testing.T) {
	t.Parallel()

	idx := newAccountIdentityIndex()
	idx.AddExisting([]service.Account{
		{ID: 10, Name: "existing-openai", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-1", "base_url": "https://x.com/v1"}},
		{ID: 11, Name: "existing-claude", Platform: service.PlatformAnthropic, Type: service.AccountTypeOAuth, Credentials: map[string]any{"refresh_token": "rt-1"}},
	})

	// 命中库内账号（尾斜杠归一化后等价）。
	dup := &service.Account{Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-1", "base_url": "https://x.com/v1/"}}
	id, name, found := idx.FindDuplicate(dup)
	require.True(t, found)
	require.Equal(t, int64(10), id)
	require.Equal(t, "existing-openai", name)

	// 同 api_key 但跨平台不应命中（作用域隔离）。
	crossPlatform := &service.Account{Platform: service.PlatformAnthropic, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-1"}}
	_, _, found = idx.FindDuplicate(crossPlatform)
	require.False(t, found, "same api_key on different platform must not be a duplicate")

	// 全新账号：不命中。
	fresh := &service.Account{Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-new"}}
	_, _, found = idx.FindDuplicate(fresh)
	require.False(t, found)

	// 把本批内一个新项登记进索引，后续同 key 项应判与之重复（id<=0 表示本批内项）。
	idx.AddBatchItem(fresh, -1, "batch-item")
	again := &service.Account{Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "sk-new"}}
	gotID, _, found := idx.FindDuplicate(again)
	require.True(t, found, "second occurrence within the same batch must be detected as duplicate")
	require.LessOrEqual(t, gotID, int64(0), "batch-internal duplicate should carry a non-positive id")
}
