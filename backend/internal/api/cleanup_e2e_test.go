package api

// 存量清洗 e2e：scan 分组/action 预标 → execute 混合清理（快照）→ restore 回滚 → 号不复用实证。
// 规划：.claude/plan/db-import-clean-dedupe.md 任务 3.5。
import (
	"encoding/json"
	"net/http"
	"testing"

	"mutag/backend/internal/domain"
)

type scanResp struct {
	Groups []struct {
		Key      string `json:"key"`
		Name     string `json:"name"`
		Clusters int    `json:"clusters"`
		Members  []struct {
			TagID  string `json:"tag_id"`
			Events int64  `json:"events"`
			Scans  int64  `json:"scans"`
			Keep   bool   `json:"keep"`
			Action string `json:"action"`
		} `json:"members"`
	} `json:"groups"`
	Summary struct {
		Delete int `json:"delete"`
		Void   int `json:"void"`
		Keep   int `json:"keep"`
	} `json:"summary"`
}

type execResp struct {
	JobID   string `json:"job_id"`
	Deleted int64  `json:"deleted"`
	Voided  int64  `json:"voided"`
}

func TestE2ECleanupChain(t *testing.T) {
	gr := newTestRouter(t)

	// seed：A×3（垃圾导入）+ B×2；给 A 的一条追加业务履历（→ 应预标 void 而非 delete）
	seed := `{"items":[{"seq":"1","name":"清洗测试A","buy_date":"2026-08-01","qty":3,"unit":null,"keeper":null},
{"seq":"2","name":"清洗测试B","buy_date":"2026-08-02","qty":2,"unit":null,"keeper":null}]}`
	if w := gr.do(t, http.MethodPost, "/api/admin/import", seed); w.Code != http.StatusOK {
		t.Fatalf("seed 导入失败: %d %s", w.Code, w.Body.String())
	}
	// A 组 tag 列表（按导入顺序 3 条）与追加履历
	var seeded struct {
		Items []struct {
			TagID string `json:"tag_id"`
			Name  string `json:"name"`
		} `json:"items"`
		Total int `json:"total"`
	}
	w := gr.do(t, http.MethodGet, "/api/admin/assets?page=1&size=50&keyword=清洗测试", "")
	if err := json.Unmarshal(w.Body.Bytes(), &seeded); err != nil {
		t.Fatalf("seed 读取: %v", err)
	}
	if seeded.Total != 5 {
		t.Fatalf("seed 应 5 条，got %d", seeded.Total)
	}
	var aTags []string
	var aFirst string // 给它追加履历（sort 后最早的保留——append 履历到任一条，验证 action=void 预标）
	for _, it := range seeded.Items {
		if it.Name == "清洗测试A" {
			aTags = append(aTags, it.TagID)
			if aFirst == "" {
				aFirst = it.TagID
			}
		}
	}
	if len(aTags) != 3 {
		t.Fatalf("A 组应 3 条，got %d", len(aTags))
	}
	// 追加业务履历： Events=2 → 预标 void
	if a, err := gr.deps.Store.Assets().GetByTagID(testCtx(), aFirst); err == nil {
		note := "e2e 业务履历"
		_ = gr.deps.Store.Events().Create(testCtx(), &domain.AssetEvent{
			AssetID: a.AssetID, Type: domain.EventReceive, Actor: "tester",
			NewValue: map[string]any{"keeper": "tester"}, Note: &note,
		})
	}

	// 1) scan：两组重复 + action 预标
	w = gr.do(t, http.MethodPost, "/api/admin/cleanup/scan", `{"mode":"all","keep_strategy":"earliest_1"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("scan 应 200，got %d: %s", w.Code, w.Body.String())
	}
	var sc scanResp
	if err := json.Unmarshal(w.Body.Bytes(), &sc); err != nil {
		t.Fatalf("scan 解析: %v", err)
	}
	if len(sc.Groups) != 2 {
		t.Fatalf("应 2 重复组，got %d", len(sc.Groups))
	}
	var deleteIDs, voidIDs []string
	for _, g := range sc.Groups {
		kept, removed := 0, 0
		for _, m := range g.Members {
			if m.Keep {
				kept++
				continue
			}
			removed++
			switch m.Action {
			case "delete":
				deleteIDs = append(deleteIDs, m.TagID)
			case "void":
				voidIDs = append(voidIDs, m.TagID)
			}
		}
		if kept != 1 || removed != len(g.Members)-1 {
			t.Errorf("组 %s 应 keep=1 remove=%d-1，got keep=%d remove=%d", g.Name, len(g.Members), kept, removed)
		}
	}
	// A 组的履历行必须预标 void（不是 delete）
	foundVoidWithHistory := false
	for _, g := range sc.Groups {
		if g.Name != "清洗测试A" {
			continue
		}
		for _, m := range g.Members {
			if m.Events > 1 && m.Action == "void" {
				foundVoidWithHistory = true
			}
		}
	}
	if !foundVoidWithHistory {
		t.Fatal("有履历的成员应预标 void")
	}
	if sc.Summary.Delete != len(deleteIDs) || sc.Summary.Void != len(voidIDs) {
		t.Fatalf("summary 与明细不符: delete %d/%d void %d/%d", sc.Summary.Delete, len(deleteIDs), sc.Summary.Void, len(voidIDs))
	}

	// 2) confirm_count 不符 → 400
	bad := `{"mode":"all","keep_strategy":"earliest_1","delete_tag_ids":["MT-2026-000001"],"void_tag_ids":[],"confirm_count":5}`
	if w := gr.do(t, http.MethodPost, "/api/admin/cleanup/execute", bad); w.Code != http.StatusBadRequest {
		t.Fatalf("confirm_count 不符应 400，got %d", w.Code)
	}

	// 3) execute 混合清理
	exBody, _ := json.Marshal(map[string]any{
		"mode": "all", "keep_strategy": "earliest_1",
		"delete_tag_ids": deleteIDs, "void_tag_ids": voidIDs,
		"confirm_count": len(deleteIDs) + len(voidIDs),
	})
	w = gr.do(t, http.MethodPost, "/api/admin/cleanup/execute", string(exBody))
	if w.Code != http.StatusOK {
		t.Fatalf("execute 应 200，got %d: %s", w.Code, w.Body.String())
	}
	var ex execResp
	if err := json.Unmarshal(w.Body.Bytes(), &ex); err != nil {
		t.Fatalf("execute 解析: %v", err)
	}
	if ex.Deleted != int64(len(deleteIDs)) {
		t.Errorf("deleted 应 %d，got %d", len(deleteIDs), ex.Deleted)
	}
	// 终态断言：delete 行的 assets 无、tags void 保留
	for _, tag := range deleteIDs {
		if _, err := gr.deps.Store.Assets().GetByTagID(testCtx(), tag); err == nil {
			t.Errorf("delete 行 %s 的资产应已删除", tag)
		}
	}
	for _, tag := range deleteIDs {
		tag_, err := gr.deps.Store.Tags().GetByTagID(testCtx(), tag)
		if err != nil {
			t.Errorf("delete 行 %s 的 tags 应保留（号簿），got err %v", tag, err)
		} else if tag_.Status != domain.TagVoid {
			t.Errorf("delete 行 %s 的 tags 应 void，got %s", tag, tag_.Status)
		}
	}

	// 4) 号不复用：NextTagID 不得落进已删号区间（tags void 保留 → MAX 含 void 号）
	next, err := gr.deps.Store.Tags().NextTagID(testCtx())
	if err != nil {
		t.Fatalf("NextTagID: %v", err)
	}
	for _, tag := range deleteIDs {
		if next >= tag && tag >= "MT-2026-000000" && next <= tag+"z" {
			t.Errorf("取号 %s 回池到已删号 %s——违反纪律 1", next, tag)
			break
		}
	}

	// 5) restore：回补 + tags 状态恢复
	w = gr.do(t, http.MethodPost, "/api/admin/cleanup/jobs/"+ex.JobID+"/restore", "")
	if w.Code != http.StatusOK {
		t.Fatalf("restore 应 200，got %d: %s", w.Code, w.Body.String())
	}
	var rr struct {
		Restored int64 `json:"restored"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &rr)
	if rr.Restored != int64(len(deleteIDs)) {
		t.Errorf("restore 应回补 %d，got %d", len(deleteIDs), rr.Restored)
	}
	for _, tag := range deleteIDs {
		if _, err := gr.deps.Store.Assets().GetByTagID(testCtx(), tag); err != nil {
			t.Errorf("restore 后 %s 资产应回来", tag)
		}
		if tag_, err := gr.deps.Store.Tags().GetByTagID(testCtx(), tag); err != nil || tag_.Status != domain.TagActive {
			t.Errorf("restore 后 %s tags 应恢复 active", tag)
		}
	}
}
