package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// accountIdentityIndex 是基于 service.AccountIdentityKeys 的判重索引。
// 任意一把身份 key 命中即视为同一账号（同 platform+type 内）。
//
// 用于导入去重：先用已有账号库构建索引，再在导入循环里 Add 本批已处理项，
// 这样既能与库内账号比，也能与本批前面的项比。
type accountIdentityIndex struct {
	// idByKey: 身份 key -> 已登记账号的 ID（库内账号为真实 ID；本批内项可用占位标识）。
	idByKey map[string]int64
	// nameByID: 账号 ID -> 名称，用于在跳过原因里显示"与已有账号「xxx」重复"。
	nameByID map[int64]string
}

func newAccountIdentityIndex() *accountIdentityIndex {
	return &accountIdentityIndex{
		idByKey:  make(map[string]int64),
		nameByID: make(map[int64]string),
	}
}

// AddExisting 把库内已有账号登记进索引。
func (idx *accountIdentityIndex) AddExisting(accounts []service.Account) {
	for i := range accounts {
		acc := accounts[i]
		keys := service.AccountIdentityKeys(&acc)
		if len(keys) == 0 {
			continue
		}
		idx.nameByID[acc.ID] = acc.Name
		for _, k := range keys {
			// 已有的不覆盖：保留第一个登记者（库内账号优先于后登记的同 key）。
			if _, ok := idx.idByKey[k]; !ok {
				idx.idByKey[k] = acc.ID
			}
		}
	}
}

// FindDuplicate 返回与给定账号判重命中的已登记账号 ID 与名称。
// found=false 表示无重复。id<=0 表示命中的是"本批内之前的项"（无真实库 ID）。
func (idx *accountIdentityIndex) FindDuplicate(acc *service.Account) (id int64, name string, found bool) {
	keys := service.AccountIdentityKeys(acc)
	for _, k := range keys {
		if existingID, ok := idx.idByKey[k]; ok {
			return existingID, idx.nameByID[existingID], true
		}
	}
	return 0, "", false
}

// AddBatchItem 把本批中"已处理（已创建或将要创建）"的项登记进索引，便于后续项与之判重。
// 传入的 id 对已成功创建的项是真实库 ID；对仅占位的项可传负数（如 -(index+1)）。
func (idx *accountIdentityIndex) AddBatchItem(acc *service.Account, id int64, name string) {
	keys := service.AccountIdentityKeys(acc)
	if len(keys) == 0 {
		return
	}
	if id != 0 {
		idx.nameByID[id] = name
	}
	for _, k := range keys {
		if _, ok := idx.idByKey[k]; !ok {
			idx.idByKey[k] = id
		}
	}
}
