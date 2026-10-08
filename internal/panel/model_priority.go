package panel

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sort"

	"github.com/linguo2625469/workbuddy2api-panel/internal/pool"
)

// getModelPriority 获取当前模型优先级配置、账号简要信息、以及已知模型列表
func (p *Panel) getModelPriority(w http.ResponseWriter, r *http.Request) {
	rules := map[string]pool.ModelPriorityRule{}
	if p.cfg.Pool != nil {
		rules = p.cfg.Pool.ModelPrioritySnapshot()
	}

	type acctInfo struct {
		UID      string `json:"uid"`
		Nickname string `json:"nickname"`
		Realm    string `json:"realm"`
		Credits  int64  `json:"credits"`
		Cooling  bool   `json:"cooling"`
		Disabled bool   `json:"disabled"`
		Paused   bool   `json:"paused"`
	}
	var accts []acctInfo
	if p.cfg.Pool != nil {
		for _, st := range p.cfg.Pool.List() {
			accts = append(accts, acctInfo{
				UID:      st.UID,
				Nickname: st.Nickname,
				Realm:    st.Realm,
				Credits:  st.Credits,
				Cooling:  st.Cooling,
				Disabled: st.Disabled,
				Paused:   st.Paused,
			})
		}
	}
	sort.Slice(accts, func(i, j int) bool {
		return accts[i].Nickname < accts[j].Nickname
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"rules":    rules,
		"accounts": accts,
	})
}

// saveModelPriority 保存模型优先级配置：热更新 Pool + 写盘 config.json
func (p *Panel) saveModelPriority(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}

	var req struct {
		Rules map[string]pool.ModelPriorityRule `json:"rules"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return
	}

	// 1. 热应用到 Pool
	if p.cfg.Pool != nil {
		p.cfg.Pool.SetModelPriority(req.Rules)
	}

	// 2. 落盘写入 config.json（通过 SaveConfig 闭包）
	if p.cfg.SaveConfig != nil {
		patchObj := map[string]any{
			"pool": map[string]any{
				"model_priority": req.Rules,
			},
		}
		patchBytes, _ := json.Marshal(patchObj)
		if _, err := p.cfg.SaveConfig(patchBytes); err != nil {
			log.Printf("panel: 保存 model_priority 配置失败: %v", err)
			writeErr(w, http.StatusInternalServerError, "save config: "+err.Error())
			return
		}
	}

	var curRules map[string]pool.ModelPriorityRule
	if p.cfg.Pool != nil {
		curRules = p.cfg.Pool.ModelPrioritySnapshot()
	} else {
		curRules = req.Rules
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"rules": curRules,
	})
}
