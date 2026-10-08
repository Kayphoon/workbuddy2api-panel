package pool

import (
	"testing"
	"time"

	"workbuddy2api/internal/auth"
)

func TestModelPriorityRuleMatching(t *testing.T) {
	p := New("")
	p.SetModelPriority(map[string]ModelPriorityRule{
		"kimi-k3": {
			Realms:   []string{"cn", "global"},
			Accounts: []string{"天真有邪"},
		},
		"deepseek*": {
			Realms: []string{"global", "cn"},
		},
	})

	// 1. 精确匹配
	r1 := p.MatchingPriorityRule("kimi-k3")
	if r1 == nil || len(r1.Realms) != 2 || r1.Realms[0] != "cn" {
		t.Fatalf("expected kimi-k3 exact match, got %+v", r1)
	}

	// 2. 通配符匹配
	r2 := p.MatchingPriorityRule("deepseek-v4.1-flash")
	if r2 == nil || len(r2.Realms) != 2 || r2.Realms[0] != "global" {
		t.Fatalf("expected deepseek* wildcard match, got %+v", r2)
	}

	// 3. 未匹配
	r3 := p.MatchingPriorityRule("unknown-model")
	if r3 != nil {
		t.Fatalf("expected nil for unknown-model, got %+v", r3)
	}
}

func TestModelAccountPriorityPick(t *testing.T) {
	p := New("")
	a1 := &auth.Auth{UID: "uid-first", Nickname: "天真有邪", Site: auth.SiteCN}
	a2 := &auth.Auth{UID: "uid-second", Nickname: "提米", Site: auth.SiteCN}
	a3 := &auth.Auth{UID: "uid-normal", Nickname: "普通号", Site: auth.SiteCN}

	p.byUID["uid-first"] = &entry{a: a1, credits: 5000}
	p.byUID["uid-second"] = &entry{a: a2, credits: 5000}
	p.byUID["uid-normal"] = &entry{a: a3, credits: 5000}

	p.SetModelPriority(map[string]ModelPriorityRule{
		"kimi-k3": {
			Accounts: []string{"天真有邪", "提米"},
		},
	})

	// 1. 正常状态必选第一优先级（天真有邪）
	got1 := p.pick(nil, "kimi-k3", "")
	if got1 == nil || got1.UID != "uid-first" {
		t.Fatalf("expected first priority account uid-first, got %+v", got1)
	}

	// 2. 第一优先级冷却时，自动顺延到第二优先级（提米）
	p.Cooldown("uid-first", CoolSoft, time.Hour, "test cooldown")
	got2 := p.pick(nil, "kimi-k3", "")
	if got2 == nil || got2.UID != "uid-second" {
		t.Fatalf("expected second priority account uid-second, got %+v", got2)
	}

	// 3. 第一、第二优先级都冷却时，自动回落到普通号
	p.Cooldown("uid-second", CoolSoft, time.Hour, "test cooldown")
	got3 := p.pick(nil, "kimi-k3", "")
	if got3 == nil || got3.UID != "uid-normal" {
		t.Fatalf("expected fallback to normal account uid-normal, got %+v", got3)
	}
}
