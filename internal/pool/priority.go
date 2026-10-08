package pool

import (
	"strings"
	"sync"
)

// ModelPriorityRule 针对单一模型（精确名或通配符）的调度偏好规则
type ModelPriorityRule struct {
	Realms   []string `json:"realms,omitempty"`   // 域优先级顺序，如 ["global", "cn"] 或 ["cn", "global"]
	Accounts []string `json:"accounts,omitempty"` // 优先账号列表（UID 或 Nickname 列表，排在前的优先级最高）
}

// SetModelPriority 注入模型调度优先级配置表。
func (p *Pool) SetModelPriority(m map[string]ModelPriorityRule) {
	norm := make(map[string]ModelPriorityRule, len(m))
	for k, r := range m {
		mk := strings.TrimSpace(k)
		if mk == "" {
			continue
		}
		var cleanRealms []string
		for _, realm := range r.Realms {
			rm := strings.ToLower(strings.TrimSpace(realm))
			if rm == "cn" || rm == "global" {
				cleanRealms = append(cleanRealms, rm)
			}
		}
		var cleanAccts []string
		for _, acct := range r.Accounts {
			a := strings.TrimSpace(acct)
			if a != "" {
				cleanAccts = append(cleanAccts, a)
			}
		}
		norm[mk] = ModelPriorityRule{
			Realms:   cleanRealms,
			Accounts: cleanAccts,
		}
	}
	p.modelPriorityMu.Lock()
	defer p.modelPriorityMu.Unlock()
	p.modelPriority = norm
}

// ModelPrioritySnapshot 返回当前生效的模型调度优先级配置快照。
func (p *Pool) ModelPrioritySnapshot() map[string]ModelPriorityRule {
	p.modelPriorityMu.RLock()
	defer p.modelPriorityMu.RUnlock()
	out := make(map[string]ModelPriorityRule, len(p.modelPriority))
	for k, v := range p.modelPriority {
		realmsCopy := append([]string(nil), v.Realms...)
		acctsCopy := append([]string(nil), v.Accounts...)
		out[k] = ModelPriorityRule{
			Realms:   realmsCopy,
			Accounts: acctsCopy,
		}
	}
	return out
}

// MatchingPriorityRule 匹配模型适用的优先级规则。精确匹配 > 最长前缀通配符（如 "kimi*"）。
func (p *Pool) MatchingPriorityRule(model string) *ModelPriorityRule {
	if model == "" {
		return nil
	}
	p.modelPriorityMu.RLock()
	defer p.modelPriorityMu.RUnlock()
	if len(p.modelPriority) == 0 {
		return nil
	}

	// 1. 精确匹配（大小写不敏感）
	for k, r := range p.modelPriority {
		if strings.EqualFold(k, model) {
			res := r
			return &res
		}
	}

	// 2. 最长前缀通配符匹配（形如 "prefix*"）
	bestLen := -1
	var bestRule *ModelPriorityRule
	for k, r := range p.modelPriority {
		if len(k) > 1 && strings.HasSuffix(k, "*") {
			pre := k[:len(k)-1]
			if strings.HasPrefix(strings.ToLower(model), strings.ToLower(pre)) && len(pre) > bestLen {
				bestLen = len(pre)
				res := r
				bestRule = &res
			}
		}
	}
	return bestRule
}

// FindEntryByUIDOrNicknameLocked 根据 UID、前缀或昵称查找 entry。调用方须持读锁或写锁。
func (p *Pool) FindEntryByUIDOrNicknameLocked(target string) *entry {
	if target == "" {
		return nil
	}
	if e, ok := p.byUID[target]; ok {
		return e
	}
	for _, e := range p.byUID {
		if e.a != nil && (e.a.Nickname == target || strings.HasPrefix(e.a.UID, target)) {
			return e
		}
	}
	return nil
}
