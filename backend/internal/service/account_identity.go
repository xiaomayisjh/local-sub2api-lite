package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// 本文件提供"账号判重"的统一身份指纹规则，供两处复用：
//   1) 导入 JSON / 批量创建时自动去重（与已有账号库 + 本批已处理项比对）。
//   2) 多选去重：把选中的账号按身份归组，找出重复组。
//
// 设计要点（与用户确认）：
//   - 判重比"凭证身份"，不比 name（name 用户随手起、可重复、可改，不能当依据）。
//   - 仅在"同 platform + 同 type"范围内比较——openai-oauth 与 openai-apikey 即使同一个人也视为不同账号，
//     避免跨接入方式误判。
//   - 凭证是 AES-GCM 加密存储、nonce 随机，密文必然不同；判重只能在解密后的明文 Credentials 上做。
//     调用方（handler/service）拿到的 Account.Credentials 已是解密后的 map[string]any。
//   - 一个账号产出"一组"身份 key，任意一把 key 撞上即算重复——可抵抗"导出时缺了某个字段"。
//   - token / api_key 一律取 SHA256 指纹，不在日志/返回里暴露明文，且长 token 比较高效。

// AccountIdentityKeys 为一个账号生成其判重身份 key 集合（已带 "platform|type|" 作用域前缀）。
//
// 返回的每个 key 都形如 "anthropic|oauth|refresh:<fingerprint>"，因此：
//   - 不同 platform/type 的账号天然不会互相命中（作用域已写进前缀）。
//   - 同一账号的多把 key（如 OpenAI 的 account_id 与 user_id）任意一把命中即判重复。
//
// 若账号缺乏任何可判别的稳定凭证（极少见，多半是脏数据），返回空切片——调用方应将其视为"无法判重"，
// 即永远不与他人重复（宁可漏判，不可误判）。
func AccountIdentityKeys(account *Account) []string {
	if account == nil {
		return nil
	}
	platform := strings.ToLower(strings.TrimSpace(account.Platform))
	typ := strings.ToLower(strings.TrimSpace(account.Type))
	scope := platform + "|" + typ + "|"

	raw := accountIdentityRawKeys(platform, typ, account.Credentials)
	if len(raw) == 0 {
		return nil
	}
	keys := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, k := range raw {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		full := scope + k
		if _, ok := seen[full]; ok {
			continue
		}
		seen[full] = struct{}{}
		keys = append(keys, full)
	}
	return keys
}

// accountIdentityRawKeys 按 platform+type 派生"未加作用域前缀"的身份 key（如 "refresh:<fp>"）。
func accountIdentityRawKeys(platform, typ string, creds map[string]any) []string {
	switch typ {
	case AccountTypeAPIKey:
		// API Key 账号：base_url（归一化）+ api_key 指纹。
		// base_url 可空（官方直连），此时仅凭 api_key。
		return apiKeyIdentityKeys(creds)

	case AccountTypeUpstream:
		// 上游透传：base_url 必有意义，叠加 api_key。
		return apiKeyIdentityKeys(creds)

	case AccountTypeBedrock:
		return bedrockIdentityKeys(creds)

	case AccountTypeServiceAccount:
		return serviceAccountIdentityKeys(creds)

	case AccountTypeOAuth, AccountTypeSetupToken:
		switch platform {
		case PlatformOpenAI:
			return openaiOAuthIdentityKeys(creds)
		case PlatformGemini:
			return geminiOAuthIdentityKeys(creds)
		default:
			// anthropic / antigravity / 其它 OAuth：refresh_token 最稳（access 会轮换），兜底 access。
			return tokenOAuthIdentityKeys(creds)
		}

	default:
		// 未知类型：尽量用通用凭证兜底，能比就比，比不了就返回空（视为不可判重）。
		return genericIdentityKeys(creds)
	}
}

// openaiOAuthIdentityKeys 复刻 codex import 的身份规则：
// chatgpt_account_id → chatgpt_user_id 优先；二者皆无时用 email；最后叠加 access_token 指纹兜底。
func openaiOAuthIdentityKeys(creds map[string]any) []string {
	accountID := credString(creds, "chatgpt_account_id")
	userID := credString(creds, "chatgpt_user_id")
	email := credString(creds, "email")
	accessToken := credString(creds, "access_token")

	keys := make([]string, 0, 4)
	if accountID != "" {
		keys = append(keys, "account:"+accountID)
	}
	if userID != "" {
		keys = append(keys, "user:"+userID)
	}
	if accountID == "" && userID == "" {
		if email = strings.ToLower(strings.TrimSpace(email)); email != "" {
			keys = append(keys, "email:"+email)
		}
	}
	if accessToken != "" {
		keys = append(keys, "access:"+tokenFingerprint(accessToken))
	}
	return keys
}

// tokenOAuthIdentityKeys 用于 anthropic / antigravity 等 OAuth：
// refresh_token 比 access_token 稳定（access 频繁轮换、过期），优先用 refresh，兜底 access。
func tokenOAuthIdentityKeys(creds map[string]any) []string {
	keys := make([]string, 0, 2)
	if rt := credString(creds, "refresh_token"); rt != "" {
		keys = append(keys, "refresh:"+tokenFingerprint(rt))
	}
	if at := credString(creds, "access_token"); at != "" {
		keys = append(keys, "access:"+tokenFingerprint(at))
	}
	return keys
}

// geminiOAuthIdentityKeys Gemini/Vertex OAuth：project_id + access_token 指纹。
func geminiOAuthIdentityKeys(creds map[string]any) []string {
	project := strings.ToLower(credString(creds, "project_id"))
	keys := make([]string, 0, 2)
	if rt := credString(creds, "refresh_token"); rt != "" {
		keys = append(keys, "proj-refresh:"+project+":"+tokenFingerprint(rt))
	}
	if at := credString(creds, "access_token"); at != "" {
		keys = append(keys, "proj-access:"+project+":"+tokenFingerprint(at))
	}
	return keys
}

// apiKeyIdentityKeys API Key / upstream：归一化 base_url + api_key 指纹。
func apiKeyIdentityKeys(creds map[string]any) []string {
	apiKey := credString(creds, "api_key")
	if apiKey == "" {
		// 没有 api_key 无法判别（脏数据）。
		return nil
	}
	base := normalizeBaseURLForIdentity(credString(creds, "base_url"))
	return []string{"apikey:" + base + "|" + tokenFingerprint(apiKey)}
}

// bedrockIdentityKeys AWS Bedrock：按 auth_mode 区分。
//   - apikey 模式：api_key 指纹。
//   - sigv4 模式：aws_access_key_id + aws_region（同一组 AK 在同一 region 即同一账号配置）。
func bedrockIdentityKeys(creds map[string]any) []string {
	if apiKey := credString(creds, "api_key"); apiKey != "" {
		return []string{"bedrock-apikey:" + tokenFingerprint(apiKey)}
	}
	ak := credString(creds, "aws_access_key_id")
	region := strings.ToLower(credString(creds, "aws_region"))
	if ak == "" {
		return nil
	}
	// secret 不进 key（同一 AK 不应有两个 secret），但若提供则一并指纹化以防 AK 复用。
	secretFP := ""
	if sk := credString(creds, "aws_secret_access_key"); sk != "" {
		secretFP = tokenFingerprint(sk)
	}
	return []string{"bedrock-sigv4:" + ak + "|" + region + "|" + secretFP}
}

// serviceAccountIdentityKeys Google Service Account：project_id + private_key_id 唯一标识一把 SA 密钥。
// 兜底用 client_email；再兜底整把 private_key 指纹。
func serviceAccountIdentityKeys(creds map[string]any) []string {
	project := strings.ToLower(credString(creds, "project_id"))
	if pkID := credString(creds, "private_key_id"); pkID != "" {
		return []string{"sa-key:" + project + "|" + pkID}
	}
	if email := strings.ToLower(credString(creds, "client_email")); email != "" {
		return []string{"sa-email:" + project + "|" + email}
	}
	if pk := credString(creds, "private_key"); pk != "" {
		return []string{"sa-pk:" + project + "|" + tokenFingerprint(pk)}
	}
	return nil
}

// genericIdentityKeys 未知 type 的兜底：尝试常见凭证字段，能比就比。
func genericIdentityKeys(creds map[string]any) []string {
	if apiKey := credString(creds, "api_key"); apiKey != "" {
		base := normalizeBaseURLForIdentity(credString(creds, "base_url"))
		return []string{"apikey:" + base + "|" + tokenFingerprint(apiKey)}
	}
	if rt := credString(creds, "refresh_token"); rt != "" {
		return []string{"refresh:" + tokenFingerprint(rt)}
	}
	if at := credString(creds, "access_token"); at != "" {
		return []string{"access:" + tokenFingerprint(at)}
	}
	return nil
}

// credString 安全地从凭证 map 取字符串值（trim），非字符串或缺失返回 ""。
func credString(creds map[string]any, key string) string {
	if creds == nil {
		return ""
	}
	v, ok := creds[key]
	if !ok {
		return ""
	}
	switch s := v.(type) {
	case string:
		return strings.TrimSpace(s)
	case fmt.Stringer:
		return strings.TrimSpace(s.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

// normalizeBaseURLForIdentity 归一化 base_url 仅用于判重比较：转小写、去首尾空白、去尾部斜杠。
// 这样 "https://api.x.com/v1" 与 "https://api.x.com/v1/" 视为同一上游。
// 注意：这与 crs_sync_service.go 的 normalizeBaseURL（做 SSRF 校验）用途不同，故另起名字。
func normalizeBaseURLForIdentity(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimRight(s, "/")
	return s
}

// tokenFingerprint 对 token/key 取 SHA256 十六进制指纹（与 codex import 的 codexTokenFingerprint 等价）。
func tokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

// AccountDuplicateGroup 一组互为重复的账号（多选去重用）。
type AccountDuplicateGroup struct {
	// Key 是这组共享命中的代表性身份 key（用于调试/展示，不含明文凭证）。
	Key string `json:"key"`
	// AccountIDs 组内所有账号 ID（>=2 才构成"重复组"）。
	AccountIDs []int64 `json:"account_ids"`
}

// GroupDuplicateAccounts 把一批账号按身份归组，返回所有"含 >=2 个账号"的重复组。
//
// 用于多选去重：仅做归组与高亮，不做任何删除（删谁由用户手动决定）。
// 分组采用并查集语义的简化版：任意共享一把身份 key 的账号归入同一组
// （即使它们各自的 key 集合只部分重叠，只要存在传递性连接也算同组）。
func GroupDuplicateAccounts(accounts []*Account) []AccountDuplicateGroup {
	// 并查集：account index -> root
	parent := make([]int, len(accounts))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	// key -> 第一个见到该 key 的账号 index，用于把后续命中同 key 的账号 union 起来。
	keyOwner := make(map[string]int)
	// 预先算好每个账号的身份 key（避免重复计算指纹），repKey 取首个 key 作展示。
	allKeys := make([][]string, len(accounts))
	repKey := make([]string, len(accounts))

	for i, acc := range accounts {
		keys := AccountIdentityKeys(acc)
		allKeys[i] = keys
		if len(keys) > 0 {
			repKey[i] = keys[0]
		}
		for _, k := range keys {
			if owner, ok := keyOwner[k]; ok {
				union(i, owner)
			} else {
				keyOwner[k] = i
			}
		}
	}

	// 收集每个 root 下的成员。
	members := make(map[int][]int64)
	rootRepKey := make(map[int]string)
	for i, acc := range accounts {
		// 无身份 key 的账号不参与任何组（永不判重）。
		if len(allKeys[i]) == 0 {
			continue
		}
		r := find(i)
		members[r] = append(members[r], acc.ID)
		if _, ok := rootRepKey[r]; !ok && repKey[i] != "" {
			rootRepKey[r] = repKey[i]
		}
	}

	groups := make([]AccountDuplicateGroup, 0)
	for r, ids := range members {
		if len(ids) < 2 {
			continue
		}
		groups = append(groups, AccountDuplicateGroup{
			Key:        rootRepKey[r],
			AccountIDs: ids,
		})
	}
	return groups
}
