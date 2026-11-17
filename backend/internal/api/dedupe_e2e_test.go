package api

// 导入去重 e2e：min/max 幂等语义（规划 P1 任务 2.5）。
// 场景：首次导入 → 二次全跳过 → partial 补差 → force 全建 → precheck verdict 分类。
import (
	"encoding/json"
	"net/http"
	"testing"
)

type importResp struct {
	Created    int    `json:"created"`
	Skipped    int    `json:"skipped"`
	LastTag    string `json:"last_tag"`
	Duplicates []map[string]any `json:"duplicates"`
}

type precheckResp struct {
	Groups  []struct {
		Key      string `json:"key"`
		Name     string `json:"name"`
		Incoming int    `json:"incoming"`
		Existing int    `json:"existing"`
		Verdict  string `json:"verdict"`
	} `json:"groups"`
	Summary struct {
		Create    int `json:"create"`
		SkipExact int `json:"skip_exact"`
		Suspect   int `json:"suspect"`
	} `json:"summary"`
}

const dedupeItems = `{"items":[{"seq":"1","name":"去重测试A","buy_date":"2026-06-01","qty":2,"unit":null,"keeper":"测试员"},
{"seq":"2","name":"去重测试B","buy_date":"2026-06-02","qty":1,"unit":null,"keeper":null}]}`

func TestE2EImportDedupe(t *testing.T) {
	gr := newTestRouter(t)

	// 1) 首次导入：A×2 + B×1 = created 3
	w := gr.do(t, http.MethodPost, "/api/admin/import", dedupeItems)
	if w.Code != http.StatusOK {
		t.Fatalf("首次导入应 200，got %d: %s", w.Code, w.Body.String())
	}
	var r1 importResp
	if err := json.Unmarshal(w.Body.Bytes(), &r1); err != nil {
		t.Fatalf("响应解析: %v", err)
	}
	if r1.Created != 3 || r1.Skipped != 0 {
		t.Fatalf("首次应 created=3 skipped=0，got created=%d skipped=%d", r1.Created, r1.Skipped)
	}

	// 2) 二次同 items：全跳过（幂等）
	w = gr.do(t, http.MethodPost, "/api/admin/import", dedupeItems)
	var r2 importResp
	_ = json.Unmarshal(w.Body.Bytes(), &r2)
	if r2.Created != 0 || r2.Skipped != 3 {
		t.Fatalf("二次应 created=0 skipped=3，got created=%d skipped=%d", r2.Created, r2.Skipped)
	}
	if len(r2.Duplicates) != 2 {
		t.Fatalf("重复明细应 2 组（A/B），got %d", len(r2.Duplicates))
	}

	// 3) partial 补差：A qty=3（库内 2）→ skip 2 create 1
	partial := `{"items":[{"seq":"1","name":"去重测试A","buy_date":"2026-06-01","qty":3,"unit":null,"keeper":"测试员"}]}`
	w = gr.do(t, http.MethodPost, "/api/admin/import", partial)
	var r3 importResp
	_ = json.Unmarshal(w.Body.Bytes(), &r3)
	if r3.Created != 1 || r3.Skipped != 2 {
		t.Fatalf("partial 应 created=1 skipped=2，got created=%d skipped=%d", r3.Created, r3.Skipped)
	}

	// 4) force（dedup=false）：A×1 全建不受库内 3 条影响
	w = gr.do(t, http.MethodPost, "/api/admin/import", `{"items":[{"seq":"1","name":"去重测试A","buy_date":"2026-06-01","qty":1,"unit":null,"keeper":"测试员"}]}`)
	var r4 importResp
	_ = json.Unmarshal(w.Body.Bytes(), &r4)
	if r4.Created != 0 || r4.Skipped != 1 {
		t.Fatalf("无 force 标志仍应去重，got created=%d skipped=%d", r4.Created, r4.Skipped)
	}
	// dedup=false 走顶层字段
	forceBody := `{"items":[{"seq":"1","name":"去重测试A","buy_date":"2026-06-01","qty":1,"unit":null,"keeper":"测试员"}],"dedup":false}`
	w = gr.do(t, http.MethodPost, "/api/admin/import", forceBody)
	var r5 importResp
	_ = json.Unmarshal(w.Body.Bytes(), &r5)
	if r5.Created != 1 || r5.Skipped != 0 {
		t.Fatalf("dedup=false 应 created=1 skipped=0，got created=%d skipped=%d", r5.Created, r5.Skipped)
	}

	// 5) precheck：对 A（库内 4）与全新 C 的 verdict 分类
	pre := `{"items":[{"seq":"1","name":"去重测试A","buy_date":"2026-06-01","qty":2,"unit":null,"keeper":"测试员"},
{"seq":"2","name":"去重测试C","buy_date":"2026-07-01","qty":1,"unit":null,"keeper":null}]}`
	w = gr.do(t, http.MethodPost, "/api/admin/import/precheck", pre)
	if w.Code != http.StatusOK {
		t.Fatalf("precheck 应 200，got %d: %s", w.Code, w.Body.String())
	}
	var pc precheckResp
	if err := json.Unmarshal(w.Body.Bytes(), &pc); err != nil {
		t.Fatalf("precheck 解析: %v", err)
	}
	if len(pc.Groups) != 2 {
		t.Fatalf("precheck 应 2 组，got %d", len(pc.Groups))
	}
	verdicts := make(map[string]string)
	for _, g := range pc.Groups {
		verdicts[g.Name] = g.Verdict
	}
	if verdicts["去重测试A"] != "exact" {
		t.Errorf("A 应 exact（incoming2 ≤ existing4），got %s", verdicts["去重测试A"])
	}
	if verdicts["去重测试C"] != "new" {
		t.Errorf("C 应 new，got %s", verdicts["去重测试C"])
	}
	if pc.Summary.SkipExact != 2 || pc.Summary.Create != 1 {
		t.Errorf("summary 应 skip=2 create=1，got %+v", pc.Summary)
	}

	// 6) 键归一化：大小写/空白差异视为同键
	norm := `{"items":[{"seq":"1","name":"  去重测试a ","buy_date":"2026-06-01","qty":1,"unit":null,"keeper":" 测试员 "}]}`
	w = gr.do(t, http.MethodPost, "/api/admin/import", norm)
	var r6 importResp
	_ = json.Unmarshal(w.Body.Bytes(), &r6)
	if r6.Created != 0 || r6.Skipped != 1 {
		t.Errorf("归一化后应视为同键跳过，got created=%d skipped=%d", r6.Created, r6.Skipped)
	}
}
