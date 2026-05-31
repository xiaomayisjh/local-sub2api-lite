package service

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// 本文件解决"批量测试挑了一个调不通的模型"问题。
//
// 背景：dedupeAndSortModelIDs 对上游模型列表做 sort.Strings（纯字典序），
// 导致 batch-test 取列表第一个时，挑到的往往是字典序靠前的老模型：
//   - claude-2.0          < claude-haiku-4-5
//   - claude-3-5-haiku-20241022 < claude-haiku-4-5-20251001
//   - gpt-5.2             < gpt-5.4 / gpt-5.5
// 这些老模型很多上游已经下线（Bedrock Legacy / model_not_found / 中转无渠道），
// 于是测试一上来就报 "is not a valid model ID" / "No available channel" / "Legacy"。
//
// 策略（与用户确认）：
//   - 对老模型"仅降权排队尾"，不剔除——仍支持它们的中转账号也能在新模型全失败后兜底测到。
//   - 优先测最新的（数字版本号大的）模型；同代里非 legacy 优先。
//   - batch-test 取"候选队列"前 N 个依次尝试，遇到 model_not_found 类错误就换下一个。

// rankTestableModels 把上游模型列表按"测试优先级"重排：新模型在前，老/legacy 模型排到队尾。
// 不丢弃任何模型，只改顺序。入参可以是任意顺序（含已 sort.Strings 的）。
func rankTestableModels(models []string) []string {
	cleaned := make([]string, 0, len(models))
	seen := make(map[string]struct{}, len(models))
	for _, m := range models {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		cleaned = append(cleaned, m)
	}

	sort.SliceStable(cleaned, func(i, j int) bool {
		si := scoreTestableModel(cleaned[i])
		sj := scoreTestableModel(cleaned[j])
		if si.legacy != sj.legacy {
			// 非 legacy 永远排在 legacy 前面
			return !si.legacy
		}
		if si.version != sj.version {
			// 版本号大的在前（gpt-5.5 > gpt-5.4 > gpt-5.2；4.7 > 4.6 > 4.5）
			return si.version > sj.version
		}
		if si.tierRank != sj.tierRank {
			// 同版本同 legacy 时，按能力档次：opus > sonnet > haiku（仅作为稳定排序的次级 key）
			return si.tierRank > sj.tierRank
		}
		// 最后用字典序兜底，保证确定性
		return cleaned[i] < cleaned[j]
	})

	return cleaned
}

// PickTestableModelCandidates 返回最多 maxCandidates 个"优先测试"的模型（已按 rankTestableModels 排序）。
// batch-test 会依次尝试这些候选，遇到模型不存在/无渠道就换下一个。
func PickTestableModelCandidates(models []string, maxCandidates int) []string {
	ranked := rankTestableModels(models)
	if maxCandidates > 0 && len(ranked) > maxCandidates {
		ranked = ranked[:maxCandidates]
	}
	return ranked
}

type testableModelScore struct {
	legacy   bool
	version  int // 版本号 *100，例如 5.4 -> 540、4.5 -> 450、3.5 -> 350；无版本号为 0
	tierRank int // opus=3 / sonnet=2 / pro=2 / flash/haiku/mini=1 / 其它=0
}

var (
	// 版本号：匹配 "5.4" / "4-5" / "4.6" / "3.5" 这类（点或连字符分隔的两段数字）。
	modelVersionRe = regexp.MustCompile(`(\d+)[.\-](\d+)`)
	// 单段大版本号兜底，例如 "gpt-4" / "claude-2"。
	modelMajorOnlyRe = regexp.MustCompile(`(?:^|[^0-9])(\d+)(?:[^0-9.\-]|$)`)
)

// legacyModelMatchers 命中即视为 legacy（降权排队尾，不剔除）。
// 仅列举"明确老旧 / 多数上游已下线"的模型，避免误伤当前主力模型。
var legacyModelMatchers = []*regexp.Regexp{
	// Claude 初代 / 3 / 3.5（含 2024 日期戳的 3.5）
	regexp.MustCompile(`(?i)^claude-(?:instant|v?[12])\b`),      // claude-2.0 / claude-2.1 / claude-instant
	regexp.MustCompile(`(?i)^claude-3-(?:haiku|sonnet|opus)\b`), // claude-3-haiku/sonnet/opus（初代 3）
	regexp.MustCompile(`(?i)^claude-3-5-`),                      // claude-3-5-haiku-20241022 / claude-3-5-sonnet-*
	regexp.MustCompile(`(?i)^claude-3-7-`),                      // claude-3-7-sonnet-*（较旧）
	// OpenAI 老系列
	regexp.MustCompile(`(?i)^gpt-3`),             // gpt-3.5-turbo 等
	regexp.MustCompile(`(?i)^gpt-4(?:[.\-o]|$)`), // gpt-4 / gpt-4o / gpt-4.1 / gpt-4-turbo（5.x 之前）
	regexp.MustCompile(`(?i)^(?:text|code|davinci|curie|babbage|ada)-`),
	regexp.MustCompile(`(?i)^o1(?:[.\-]|$)`), // o1 / o1-mini（早于 gpt-5 codex 线）
	// Gemini 老系列
	regexp.MustCompile(`(?i)^gemini-1`),     // gemini-1.0 / gemini-1.5
	regexp.MustCompile(`(?i)^gemini-pro\b`), // 裸 gemini-pro（初代）
	regexp.MustCompile(`(?i)^gemini-2-`),    // gemini-2-* 旧命名
}

func scoreTestableModel(model string) testableModelScore {
	lower := strings.ToLower(strings.TrimSpace(model))
	score := testableModelScore{
		legacy:   isLegacyTestableModel(lower),
		version:  parseModelVersion(lower),
		tierRank: modelTierRank(lower),
	}
	return score
}

func isLegacyTestableModel(lower string) bool {
	for _, re := range legacyModelMatchers {
		if re.MatchString(lower) {
			return true
		}
	}
	return false
}

// parseModelVersion 从模型 ID 抽取一个可比较的版本号（major*100+minor）。
// 例：gpt-5.4 -> 540；claude-opus-4-5-20251101 -> 450；gpt-4 -> 400；无版本 -> 0。
func parseModelVersion(lower string) int {
	if m := modelVersionRe.FindStringSubmatch(lower); m != nil {
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		if minor > 99 {
			minor = 99
		}
		return major*100 + minor
	}
	if m := modelMajorOnlyRe.FindStringSubmatch(lower); m != nil {
		major, _ := strconv.Atoi(m[1])
		return major * 100
	}
	return 0
}

func modelTierRank(lower string) int {
	switch {
	case strings.Contains(lower, "opus"):
		return 3
	case strings.Contains(lower, "sonnet"), strings.Contains(lower, "-pro"), strings.HasSuffix(lower, "pro"):
		return 2
	case strings.Contains(lower, "haiku"), strings.Contains(lower, "flash"), strings.Contains(lower, "mini"):
		return 1
	default:
		return 0
	}
}

// IsModelUnavailableTestError 判断测试错误是否为"这个模型本身不可用"（而非账号/网络问题）。
// 命中时 batch-test 应换下一个候选模型，而不是对同一个模型重试。
//
// 覆盖的上游报错形态（来自真实日志）：
//   - "No available channel for model X under group ..."   (new-api / voapi distributor)
//   - "model_not_found" / "is not a valid model ID"          (Anthropic / 中转)
//   - "ResourceNotFoundException: ... marked by provider as Legacy"  (AWS Bedrock)
//   - "unknown model" / "model not found"
func IsModelUnavailableTestError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	needles := []string{
		"no available channel for model",
		"model_not_found",
		"is not a valid model id",
		"marked by provider as legacy",
		"unknown model",
		"model not found",
		"does not exist",
		"no such model",
	}
	for _, n := range needles {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}

// IsUpstreamPoolUnavailableTestError 判断测试错误是否为"上游/中转池子临时不可用"，
// 即与"这个账号的凭证好不好"无关的临时状态：中转后端没号了、网关 5xx、服务临时不可用、
// 分组下暂时没有可用渠道等。
//
// 命中这类错误时，batch-test 不应把账号判成 ❌ 失败（会误导用户一键禁用好账号），
// 而应标成 ⚠️ "暂不可用"——账号凭证未被证伪，过一会儿/上游恢复后多半还能用。
//
// 覆盖的真实日志形态：
//   - "No available accounts: no available accounts"           (voapi 中转自己没号)
//   - "Service temporarily unavailable"                        (上游临时抖)
//   - "No available channel for model X under group Y"         (分组下暂时无渠道)
//   - "分组 X 下模型 Y 无可用渠道（distributor）"                  (同上，中文 new-api)
//   - 纯 "error code: 502" / HTTP 502/503/504                  (网关/上游短时故障)
//
// 注意：与 IsModelUnavailableTestError 的关系——"无渠道"既是换候选模型的信号（试别的模型
// 可能有渠道），也是池子问题；当所有候选都换完仍是这类错误时，最终判定走"暂不可用"。
func IsUpstreamPoolUnavailableTestError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// 明确的"池子/上游临时不可用"措辞。
	poolNeedles := []string{
		"no available accounts",
		"no available account",
		"service temporarily unavailable",
		"temporarily unavailable",
		"no available channel", // 分组下暂时无渠道（英文 new-api / voapi）
		"无可用渠道",                // 同上（中文 new-api）
		"无可用账号",                // 中转无可用账号（中文）
		"overloaded",           // 上游过载
		"please try again",     // 通用稍后重试
	}
	for _, n := range poolNeedles {
		if strings.Contains(msg, n) {
			return true
		}
	}

	// 纯网关 5xx（502/503/504），不含更具体的鉴权/参数语义时，视为上游短时故障。
	gatewayNeedles := []string{
		"error code: 502",
		"error code: 503",
		"error code: 504",
		"http 502", "http 503", "http 504",
		"bad gateway",
		"gateway timeout",
		"returned 502", "returned 503", "returned 504",
	}
	for _, n := range gatewayNeedles {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}
