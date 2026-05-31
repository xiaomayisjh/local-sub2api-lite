package admin

import (
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// FindDuplicatesRequest 多选去重的入参：要比对的账号 ID 列表。
type FindDuplicatesRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

// DuplicateAccountInfo 重复组里单个账号的展示信息（不含任何明文凭证）。
type DuplicateAccountInfo struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Platform   string     `json:"platform"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	// SuggestKeep 标记本组里"建议保留"的那一个（最早创建者）。前端默认勾选其余为待删。
	SuggestKeep bool `json:"suggest_keep"`
}

// DuplicateGroupResult 一组互为重复的账号。
type DuplicateGroupResult struct {
	Accounts []DuplicateAccountInfo `json:"accounts"`
}

// FindDuplicatesResult 多选去重结果。
type FindDuplicatesResult struct {
	// Groups 仅含">=2 个账号"的重复组；为空表示选中的账号里没有重复。
	Groups []DuplicateGroupResult `json:"groups"`
	// TotalDuplicates 所有重复组里"超出 1 个的账号数"之和——即按建议删除后会清掉的数量。
	TotalDuplicates int `json:"total_duplicates"`
}

// FindDuplicates 在选中的账号里按身份指纹找出重复组（同 platform+type 内比）。
// 只归组、不删除：删谁由前端让用户手动勾选，调用既有的 DELETE 接口完成。
func (h *AccountHandler) FindDuplicates(c *gin.Context) {
	var req FindDuplicatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if len(req.AccountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return
	}

	ctx := c.Request.Context()
	accountsPtr, err := h.adminService.GetAccountsByIDs(ctx, req.AccountIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// 建立 ID -> Account 映射，便于按组回填展示信息。
	byID := make(map[int64]*service.Account, len(accountsPtr))
	accounts := make([]*service.Account, 0, len(accountsPtr))
	for _, acc := range accountsPtr {
		if acc == nil {
			continue
		}
		byID[acc.ID] = acc
		accounts = append(accounts, acc)
	}

	groups := service.GroupDuplicateAccounts(accounts)

	result := FindDuplicatesResult{Groups: make([]DuplicateGroupResult, 0, len(groups))}
	for _, g := range groups {
		infos := make([]DuplicateAccountInfo, 0, len(g.AccountIDs))
		for _, id := range g.AccountIDs {
			acc, ok := byID[id]
			if !ok {
				continue
			}
			infos = append(infos, DuplicateAccountInfo{
				ID:         acc.ID,
				Name:       acc.Name,
				Platform:   acc.Platform,
				Type:       acc.Type,
				Status:     acc.Status,
				CreatedAt:  acc.CreatedAt,
				LastUsedAt: acc.LastUsedAt,
			})
		}
		if len(infos) < 2 {
			continue
		}
		// 组内按创建时间升序：最早创建的排第一，标记为"建议保留"。
		sort.SliceStable(infos, func(i, j int) bool {
			return infos[i].CreatedAt.Before(infos[j].CreatedAt)
		})
		infos[0].SuggestKeep = true
		result.TotalDuplicates += len(infos) - 1
		result.Groups = append(result.Groups, DuplicateGroupResult{Accounts: infos})
	}

	response.Success(c, result)
}
