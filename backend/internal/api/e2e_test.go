package api

// 进程内全链集成测试（等价于 deploy/verify.sh 的 13 连检）：
// 临时 SQLite → 迁移 → 造数 → gin router.ServeHTTP 零端口走全链路。
// 背景：本机沙箱禁本地端口绑定，httptest.Server 起不来，故不经网络直接调 router。

import (
	"strconv"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/cache"
	"mutag/backend/internal/db"
	"mutag/backend/internal/domain"
	"mutag/backend/internal/h5"
	"mutag/backend/internal/repo"
)

// ginRouter 进程内路由包装（零端口零网络）。
type ginRouter struct {
	r    *gin.Engine
	deps Deps
}

func testCtx() context.Context {
	return context.Background()
}

func tagIDf(seq int) string {
	return fmt.Sprintf("MT-2026-%06d", seq)
}

func newTestRouterWithPassword(t *testing.T, password string) *ginRouter {
	gr := newTestRouter(t)
	gr.deps.AdminPassword = password
	gr.r = NewRouter(gr.deps)
	return gr
}

// newTestDeps 测试基建：临时库 + 迁移 + 造数 3 条 + 默认 deps（限流关闭）。
// 双形态实证开关：MUTAG_TEST_PG_DSN 有值时全套 e2e 跑在真 PG（如 Supabase）上——
// 双方言同一套测试，是"切换点收敛"的最终实证；无值默认 SQLite（TempDir 隔离）。
func newTestDeps(t *testing.T) (Deps, *sql.DB) {
	t.Helper()
	var dialect db.Dialect
	var database *sql.DB
	var err error

	if dsn := os.Getenv("MUTAG_TEST_PG_DSN"); dsn != "" {
		dialect = db.DialectPostgres
		database, err = db.OpenPostgres(dsn)
		if err != nil {
			t.Fatalf("open pg: %v", err)
		}
		// 测试库独占：清空 public schema 防上次运行残留
		if _, err := database.Exec(`DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
			t.Fatalf("reset pg schema: %v", err)
		}
	} else {
		dialect = db.DialectSQLite
		database, err = db.OpenSQLite(filepath.Join(t.TempDir(), "test.db"))
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
	}
	t.Cleanup(func() { database.Close() })

	if err := db.Migrate(testCtx(), database, dialect); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	renderer, err := h5.NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	store := repo.NewStore(database)

	// 造数：3 条（复用管理 API 同款链路）
	tags := store.Tags()
	for i := 1; i <= 3; i++ {
		tagID := tagIDf(i)
		if err := tags.Insert(testCtx(), &domain.Tag{
			TagID: tagID, QrScene: "t:" + tagID, UrlCode: tagID, Status: domain.TagActive,
		}); err != nil {
			t.Fatalf("seed tag: %v", err)
		}
		id, err := store.Assets().Create(testCtx(), &domain.Asset{
			TagID: tagID, Name: "测试资产" + tagID, Status: domain.AssetInUse,
		})
		if err != nil {
			t.Fatalf("seed asset: %v", err)
		}
		note := "e2e seed"
		_ = store.Events().Create(testCtx(), &domain.AssetEvent{
			AssetID: id, Type: domain.EventCreate, Actor: "test", Note: &note,
		})
	}

	deps := Deps{
		DB:    database,
		Store: store,
		Cache: cache.New(60 * time.Second),
		H5:    renderer,
	}
	return deps, database
}

func newTestRouter(t *testing.T) *ginRouter {
	t.Helper()
	deps, _ := newTestDeps(t)
	return &ginRouter{r: NewRouter(deps), deps: deps}
}

func (gr *ginRouter) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	gr.r.ServeHTTP(w, req)
	return w
}

func TestE2EFullChain(t *testing.T) {
	gr := newTestRouter(t)

	t.Run("活码302", func(t *testing.T) {
		w := gr.do(t, http.MethodGet, "/t/MT-2026-000001", "")
		if w.Code != http.StatusFound || !strings.Contains(w.Header().Get("Location"), "/h5/t/MT-2026-000001") {
			t.Fatalf("code=%d location=%q", w.Code, w.Header().Get("Location"))
		}
	})

	t.Run("活码HEAD", func(t *testing.T) {
		if w := gr.do(t, http.MethodHead, "/t/MT-2026-000001", ""); w.Code != http.StatusFound {
			t.Fatalf("HEAD code=%d", w.Code)
		}
	})

	t.Run("H5详情", func(t *testing.T) {
		w := gr.do(t, http.MethodGet, "/h5/t/MT-2026-000001", "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "测试资产MT-2026-000001") {
			t.Fatalf("code=%d body含资产名失败", w.Code)
		}
	})

	t.Run("JSON_API含履历", func(t *testing.T) {
		w := gr.do(t, http.MethodGet, "/api/v1/assets/MT-2026-000001", "")
		var body struct {
			Asset  domain.Asset `json:"asset"`
			Events []struct {
				Type domain.EventType `json:"type"`
			} `json:"events"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Asset.TagID != "MT-2026-000001" {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
		if len(body.Events) == 0 || body.Events[0].Type != domain.EventCreate {
			t.Fatalf("履历缺失: %+v", body.Events)
		}
	})

	t.Run("未知码404", func(t *testing.T) {
		if w := gr.do(t, http.MethodGet, "/t/MT-2026-999999", ""); w.Code != http.StatusNotFound {
			t.Fatalf("code=%d", w.Code)
		}
	})

	t.Run("健康探针", func(t *testing.T) {
		w := gr.do(t, http.MethodGet, "/healthz", "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"ok"`) {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("免鉴权登录open", func(t *testing.T) {
		w := gr.do(t, http.MethodPost, "/api/admin/login", `{"password":"x"}`)
		if !strings.Contains(w.Body.String(), `"open":true`) {
			t.Fatalf("body=%s", w.Body.String())
		}
	})

	t.Run("新建自动配码004", func(t *testing.T) {
		w := gr.do(t, http.MethodPost, "/api/admin/assets", `{"name":"验收测试资产","unit":"个","keeper":"admin"}`)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "MT-2026-000004") {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("编辑写履历", func(t *testing.T) {
		w := gr.do(t, http.MethodPut, "/api/admin/assets/MT-2026-000004",
			`{"name":"验收测试资产（改名）","keeper":"admin","status":"in_use"}`)
		// name 与 keeper 相对 seed 值均有变化 → changed=2
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"changed":2`) {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("作废后扫码已作废", func(t *testing.T) {
		if w := gr.do(t, http.MethodPost, "/api/admin/tags/MT-2026-000004/void", ""); w.Code != http.StatusOK {
			t.Fatalf("void code=%d", w.Code)
		}
		w := gr.do(t, http.MethodGet, "/h5/t/MT-2026-000004", "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "已作废") {
			t.Fatalf("code=%d body含「已作废」失败", w.Code)
		}
	})

	t.Run("统计", func(t *testing.T) {
		w := gr.do(t, http.MethodGet, "/api/admin/stats", "")
		if !strings.Contains(w.Body.String(), `"by_status"`) {
			t.Fatalf("body=%s", w.Body.String())
		}
	})

	t.Run("SPA_fallback", func(t *testing.T) {
		w := gr.do(t, http.MethodGet, "/admin/codes", "")
		if w.Code != http.StatusOK || !strings.HasPrefix(w.Body.String(), "<!DOCTYPE html>") {
			t.Fatalf("code=%d body=%q", w.Code, w.Body.String()[:min(60, w.Body.Len())])
		}
	})
}

// M2 员工端链路：dev 登录 → 快捷操作四动作 → 履历落库 → 我的上报
func TestE2EEmployeeChain(t *testing.T) {
	gr := newTestRouter(t)

	t.Run("dev登录", func(t *testing.T) {
		w := gr.do(t, http.MethodPost, "/api/v1/auth/login", `{"code":"testcode"}`)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"role":"staff"`) {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
	})

	t.Run("未登录快捷操作401", func(t *testing.T) {
		if w := gr.do(t, http.MethodPost, "/api/v1/assets/MT-2026-000001/actions", `{"action":"receive"}`); w.Code != http.StatusUnauthorized {
			t.Fatalf("got %d", w.Code)
		}
	})

	// 登录拿 token
	w := gr.do(t, http.MethodPost, "/api/v1/auth/login", `{"code":"emp1"}`)
	var body struct {
		Token string `json:"token"`
		User  struct {
			OpenID string `json:"openid"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Token == "" {
		t.Fatalf("login: %s", w.Body.String())
	}
	authed := func(method, path, payload string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+body.Token)
		rec := httptest.NewRecorder()
		gr.r.ServeHTTP(rec, req)
		return rec
	}

	t.Run("领用", func(t *testing.T) {
		if r := authed(http.MethodPost, "/api/v1/assets/MT-2026-000001/actions", `{"action":"receive","note":"e2e领用"}`); r.Code != http.StatusOK {
			t.Fatalf("receive: %d %s", r.Code, r.Body.String())
		}
		a, err := gr.deps.Store.Assets().GetByTagID(testCtx(), "MT-2026-000001")
		if err != nil || a.Keeper == nil || *a.Keeper != body.User.OpenID {
			t.Fatalf("keeper 未变更: %v %+v", err, a.Keeper)
		}
	})

	t.Run("报修", func(t *testing.T) {
		if r := authed(http.MethodPost, "/api/v1/assets/MT-2026-000002/actions", `{"action":"repair","note":"e2e报修"}`); r.Code != http.StatusOK {
			t.Fatalf("repair: %d %s", r.Code, r.Body.String())
		}
		a, _ := gr.deps.Store.Assets().GetByTagID(testCtx(), "MT-2026-000002")
		if a.Status != domain.AssetRepair {
			t.Fatalf("status=%s", a.Status)
		}
	})

	t.Run("位置变更", func(t *testing.T) {
		if r := authed(http.MethodPost, "/api/v1/assets/MT-2026-000003/actions", `{"action":"transfer","location":"B栋1层"}`); r.Code != http.StatusOK {
			t.Fatalf("transfer: %d %s", r.Code, r.Body.String())
		}
	})

	t.Run("我的上报含履历", func(t *testing.T) {
		r := authed(http.MethodGet, "/api/v1/auth/my-events", "")
		var myBody struct {
			Events []struct {
				Actor string `json:"actor"`
				Type  string `json:"type"`
			} `json:"events"`
		}
		if err := json.Unmarshal(r.Body.Bytes(), &myBody); err != nil || len(myBody.Events) < 2 {
			t.Fatalf("events 不足: %d %s", len(myBody.Events), r.Body.String())
		}
		for _, e := range myBody.Events {
			if e.Actor != body.User.OpenID {
				t.Fatalf("actor 不属于本人: %s", e.Actor)
			}
		}
	})
}

// 带密码鉴权形态：错误密码 401 → 正确密码发 token → 带 token 可用
func TestE2EAuthMode(t *testing.T) {
	gr := newTestRouterWithPassword(t, "s3cret")

	if w := gr.do(t, http.MethodGet, "/api/admin/stats", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("无 token 应 401, got %d", w.Code)
	}
	w := gr.do(t, http.MethodPost, "/api/admin/login", `{"password":"wrong"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("错密码应 401, got %d", w.Code)
	}
	w = gr.do(t, http.MethodPost, "/api/admin/login", `{"password":"s3cret"}`)
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Token == "" {
		t.Fatalf("登录失败: %s", w.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+body.Token)
	rec := httptest.NewRecorder()
	gr.r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("带 token 应 200, got %d", rec.Code)
	}
}

// M4 财务链路：内置模板播种 → 导出（映射转换+审计）→ 导入预览（差异）→ 确认入库 → 按批次回滚
func TestE2EFinanceChain(t *testing.T) {
	gr := newTestRouter(t)

	// 给 seed 资产 001 配 SN + 财务字段（导入更新的匹配锚点）
	if w := gr.do(t, http.MethodPut, "/api/admin/assets/MT-2026-000001",
		`{"name":"测试资产MT-2026-000001","sn":"SN-FIX-001","price":990.5,"buy_date":"2026-01-10","dept_id":"D01","status":"in_use"}`); w.Code != http.StatusOK {
		t.Fatalf("配置锚点资产失败: %d %s", w.Code, w.Body.String())
	}

	t.Run("内置模板已播种", func(t *testing.T) {
		w := gr.do(t, http.MethodGet, "/api/admin/fin/mappings", "")
		var body struct {
			Items []struct {
				MappingID string `json:"mapping_id"`
				Version   int64  `json:"version"`
			} `json:"items"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || len(body.Items) < 4 {
			t.Fatalf("模板应≥4: %d %s", len(body.Items), w.Body.String())
		}
		kinds := map[string]bool{}
		for _, m := range body.Items {
			kinds[m.MappingID] = true
		}
		for _, want := range []string{"yonyou_u8", "kingdee", "guanjiapo", "generic"} {
			if !kinds[want] {
				t.Fatalf("缺内置模板 %s", want)
			}
		}
	})

	t.Run("导出映射转换与审计", func(t *testing.T) {
		w := gr.do(t, http.MethodPost, "/api/admin/fin/export", `{"mapping_id":"yonyou_u8","dept":"D01"}`)
		var body struct {
			JobID    string `json:"job_id"`
			RowCount int    `json:"row_count"`
			Columns  []struct {
				Name  string `json:"name"`
				Field string `json:"field"`
			} `json:"columns"`
			Rows [][]string `json:"rows"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.RowCount != 1 {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
		if body.JobID == "" || len(body.Columns) != 10 || len(body.Rows[0]) != len(body.Columns) {
			t.Fatalf("列行不对齐: cols=%d row=%d", len(body.Columns), len(body.Rows[0]))
		}
		// 映射转换：金额两位小数 + 状态中文 + 日期格式
		joined := strings.Join(body.Rows[0], "|")
		if !strings.Contains(joined, "990.50") || !strings.Contains(joined, "在用") || !strings.Contains(joined, "2026-01-10") {
			t.Fatalf("映射转换缺失: %s", joined)
		}
		// 审计留痕可查
		w2 := gr.do(t, http.MethodGet, "/api/admin/fin/export/jobs", "")
		if !strings.Contains(w2.Body.String(), body.JobID) {
			t.Fatalf("导出未留痕: %s", w2.Body.String())
		}
	})

	// 四行混合（generic 模板，含「序列号」列）：新增 / 更新（SN-FIX-001 改名改价）/ 错误（金额非数字）/ 文件内 SN 重复
	previewPayload := `{"mapping_id":"generic","file_name":"财务台账.xlsx","rows":[
		{"名称":"财务导入资产A","原值":"1234.5","购置日期":"2026-01-05","序列号":"SN-A"},
		{"名称":"测试资产MT-2026-000001（财务修正）","序列号":"SN-FIX-001","原值":"1200"},
		{"名称":"坏行","原值":"abc"},
		{"序列号":"SN-A","名称":"文件内重复SN行"}
	]}`

	var batchID string
	t.Run("预览差异", func(t *testing.T) {
		w := gr.do(t, http.MethodPost, "/api/admin/fin/import/preview", previewPayload)
		var body struct {
			BatchID string `json:"batch_id"`
			Summary struct {
				Creates int `json:"creates"`
				Updates int `json:"updates"`
				Errors  int `json:"errors"`
			} `json:"summary"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
		if body.Summary.Creates != 1 || body.Summary.Updates != 1 || body.Summary.Errors != 2 {
			t.Fatalf("差异计数不符: %+v body=%s", body.Summary, w.Body.String())
		}
		batchID = body.BatchID
	})

	t.Run("确认入库", func(t *testing.T) {
		w := gr.do(t, http.MethodPost, "/api/admin/fin/import/batches/"+batchID+"/confirm", "")
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"created":1`) {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
		// 新增资产落库（新码号 MT-2026-000004）+ 锚点资产已按导入改名改价
		if found, err := gr.deps.Store.Assets().GetBySN(testCtx(), "SN-A"); err != nil || len(found) != 1 {
			t.Fatalf("新增资产未落库: %v %+v", err, found)
		}
		a, _ := gr.deps.Store.Assets().GetByTagID(testCtx(), "MT-2026-000001")
		if a.Name != "测试资产MT-2026-000001（财务修正）" || a.Price == nil || *a.Price != 1200 {
			t.Fatalf("更新未生效: %s %.2f", a.Name, derefPrice(a.Price))
		}
	})

	t.Run("回滚恢复", func(t *testing.T) {
		w := gr.do(t, http.MethodPost, "/api/admin/fin/import/batches/"+batchID+"/rollback", "")
		if w.Code != http.StatusOK {
			t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
		}
		// 新增行：资产删除、码作废（码号已消耗不复用）
		created, err := gr.deps.Store.Assets().GetBySN(testCtx(), "SN-A")
		if err != nil || len(created) != 0 {
			t.Fatalf("回滚后新增资产应删除: %v %+v", err, created)
		}
		tag, err := gr.deps.Store.Tags().GetByTagID(testCtx(), "MT-2026-000004")
		if err != nil || tag.Status != domain.TagVoid {
			t.Fatalf("回滚后码应置 void: %v %+v", err, tag)
		}
		// 更新行：恢复旧值
		a, _ := gr.deps.Store.Assets().GetByTagID(testCtx(), "MT-2026-000001")
		if a.Name != "测试资产MT-2026-000001" || a.Price == nil || *a.Price != 990.5 {
			t.Fatalf("回滚未恢复旧值: %s", a.Name)
		}
		// 状态机：rolled_back 不可再回滚、不可再确认
		if w := gr.do(t, http.MethodPost, "/api/admin/fin/import/batches/"+batchID+"/rollback", ""); w.Code != http.StatusBadRequest {
			t.Fatalf("二次回滚应 400, got %d", w.Code)
		}
		if w := gr.do(t, http.MethodPost, "/api/admin/fin/import/batches/"+batchID+"/confirm", ""); w.Code != http.StatusBadRequest {
			t.Fatalf("rolled_back 再确认应 400, got %d", w.Code)
		}
	})
}

// 网格编辑通道：_tag_id 直指已有资产（不走模板列名反查，SN 缺失也能精确匹配）
func TestE2EGridEdit(t *testing.T) {
	gr := newTestRouter(t)

	w := gr.do(t, http.MethodPost, "/api/admin/fin/import/preview", `{"mapping_id":"generic","rows":[
		{"_tag_id":"MT-2026-000002","name":"网格改名资产","price":"50.5","status":"idle"},
		{"_tag_id":"MT-2026-999999","name":"不存在的码"},
		{"_tag_id":"MT-2026-000003","price":"abc"}
	]}`)
	var body struct {
		BatchID string `json:"batch_id"`
		Summary struct {
			Creates int `json:"creates"`
			Updates int `json:"updates"`
			Errors  int `json:"errors"`
		} `json:"summary"`
		Updates []struct {
			TagID string            `json:"tag_id"`
			New   map[string]string `json:"new"`
			Old   map[string]string `json:"old"`
		} `json:"updates"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if body.Summary.Updates != 1 || body.Summary.Errors != 2 {
		t.Fatalf("网格差异计数不符: %+v body=%s", body.Summary, w.Body.String())
	}
	if len(body.Updates) == 1 && body.Updates[0].TagID != "MT-2026-000002" {
		t.Fatalf("更新应命中 002: %+v", body.Updates)
	}

	// 确认 → 库内生效（改名 + 状态 idle + 价格 50.50）
	if w := gr.do(t, http.MethodPost, "/api/admin/fin/import/batches/"+body.BatchID+"/confirm", ""); w.Code != http.StatusOK {
		t.Fatalf("confirm: %d %s", w.Code, w.Body.String())
	}
	a, _ := gr.deps.Store.Assets().GetByTagID(testCtx(), "MT-2026-000002")
	if a.Name != "网格改名资产" || a.Status != domain.AssetIdle || a.Price == nil || *a.Price != 50.5 {
		t.Fatalf("网格编辑未生效: %s %s %.2f", a.Name, a.Status, derefPrice(a.Price))
	}
}

// derefPrice *float64 → 文本（断言辅助）。
func derefPrice(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

// 扫码限流（纪律 4）：容量 2 的桶 → 前两次 302，第三次 429；管理端 API 不受限流影响
func TestE2ERateLimit(t *testing.T) {
	deps, _ := newTestDeps(t)
	deps.Limiter = cache.NewLimiter(2, 0.001) // 极慢回填，测试窗口内不恢复
	gr := &ginRouter{r: NewRouter(deps), deps: deps}

	for i := 0; i < 2; i++ {
		if w := gr.do(t, http.MethodGet, "/t/MT-2026-000001", ""); w.Code != http.StatusFound {
			t.Fatalf("第 %d 次扫码应 302, got %d", i+1, w.Code)
		}
	}
	if w := gr.do(t, http.MethodGet, "/t/MT-2026-000001", ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("桶空应 429, got %d", w.Code)
	}
	// H5 直打同桶
	if w := gr.do(t, http.MethodGet, "/h5/t/MT-2026-000001", ""); w.Code != http.StatusTooManyRequests {
		t.Fatalf("H5 同桶应 429, got %d", w.Code)
	}
	// 管理端不受扫码限流影响
	if w := gr.do(t, http.MethodGet, "/api/admin/stats", ""); w.Code != http.StatusOK {
		t.Fatalf("管理端不应被限流, got %d", w.Code)
	}
}

// 码扫码统计：scan_logs 有数据的码返回次数与最近扫码
func TestE2EScanStats(t *testing.T) {
	gr := newTestRouter(t)
	// 扫码两次（不同码号）
	gr.do(t, http.MethodGet, "/t/MT-2026-000001", "")
	gr.do(t, http.MethodGet, "/t/MT-2026-000001", "")
	gr.do(t, http.MethodGet, "/t/MT-2026-000002", "")
	w := gr.do(t, http.MethodGet, "/api/admin/tags/scan-stats", "")
	var body struct {
		Items []struct {
			TagID string `json:"tag_id"`
			Count int64  `json:"count"`
			Last  string `json:"last"`
		} `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || len(body.Items) < 2 {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	byTag := map[string]int64{}
	for _, it := range body.Items {
		byTag[it.TagID] = it.Count
		if it.Last == "" {
			t.Fatalf("last 为空: %+v", it)
		}
	}
	if byTag["MT-2026-000001"] != 2 || byTag["MT-2026-000002"] != 1 {
		t.Fatalf("扫码计数不符: %+v", byTag)
	}
}

// 活码对外域名下发：服务端 BASE_URL 单一事实源（码内容不随后台入口域名漂移）
func TestE2EAppConfig(t *testing.T) {
	deps, _ := newTestDeps(t)
	deps.BaseURL = "https://mutag.qingmulife.com"
	gr := &ginRouter{r: NewRouter(deps), deps: deps}
	w := gr.do(t, http.MethodGet, "/api/admin/config", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"base_url":"https://mutag.qingmulife.com"`) {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
}

// 0006 资产模型增强全链：两级位置树 → 设备（挂位置）→ 部件（挂设备+数量）→
// H5 详情（位置路径/所属设备/部件清单）→ 删除保护。
func TestE2EAssetHierarchy(t *testing.T) {
	gr := newTestRouter(t)

	// 位置：建一级区域
	w := gr.do(t, http.MethodPost, "/api/admin/locations", `{"name":"研发机房"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("建区域 code=%d body=%s", w.Code, w.Body.String())
	}
	var zone struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &zone)
	if zone.ID == 0 {
		t.Fatal("区域 id 为空")
	}

	// 二级点位
	w = gr.do(t, http.MethodPost, "/api/admin/locations",
		`{"name":"机柜A-3","parent_id":`+strconv.FormatInt(zone.ID, 10)+`}`)
	if w.Code != http.StatusOK {
		t.Fatalf("建点位 code=%d body=%s", w.Code, w.Body.String())
	}
	var spot struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &spot)

	// 三级挂点位 → 400
	w = gr.do(t, http.MethodPost, "/api/admin/locations",
		`{"name":"非法三级","parent_id":`+strconv.FormatInt(spot.ID, 10)+`}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("三级应 400: code=%d", w.Code)
	}

	// 列表含两节点
	w = gr.do(t, http.MethodGet, "/api/admin/locations", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "研发机房") ||
		!strings.Contains(w.Body.String(), "机柜A-3") {
		t.Fatalf("位置列表不符: %s", w.Body.String())
	}

	// 设备资产：挂区域位置
	w = gr.do(t, http.MethodPost, "/api/admin/assets",
		`{"name":"测试机柜设备","location_id":`+strconv.FormatInt(zone.ID, 10)+`,"brand":"APC"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("建设备失败: code=%d body=%s", w.Code, w.Body.String())
	}
	deviceTag := tagIDf(4) // 造数 3 条后下一码 000004

	// 部件资产：挂设备 + qty=4
	w = gr.do(t, http.MethodPost, "/api/admin/assets",
		`{"name":"测试硬盘","parent_tag_id":"`+deviceTag+`","qty":4,"brand":"Seagate"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("建部件失败: code=%d body=%s", w.Code, w.Body.String())
	}
	partTag := tagIDf(5)

	// 部件 H5：所属设备名 + 位置路径继承检查（部件未挂位置 → 无安装位置行）
	w = gr.do(t, http.MethodGet, "/h5/t/"+partTag, "")
	if w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), "所属设备") ||
		!strings.Contains(w.Body.String(), "测试机柜设备") ||
		!strings.Contains(w.Body.String(), "数量") {
		t.Fatalf("部件 H5 缺挂载信息: %s", w.Body.String())
	}

	// 设备 H5：部件清单 + 位置路径（区域 → 点位，设备挂的是区域所以路径=研发机房）
	w = gr.do(t, http.MethodGet, "/h5/t/"+deviceTag, "")
	if w.Code != http.StatusOK ||
		!strings.Contains(w.Body.String(), "部件清单") ||
		!strings.Contains(w.Body.String(), "测试硬盘") ||
		!strings.Contains(w.Body.String(), "研发机房") {
		t.Fatalf("设备 H5 缺部件清单/位置: %s", w.Body.String())
	}

	// 删除保护：区域被设备引用 → 400
	w = gr.do(t, http.MethodDelete, "/api/admin/locations/"+strconv.FormatInt(zone.ID, 10), "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("区域有引用应 400: code=%d body=%s", w.Code, w.Body.String())
	}

	// 点位无引用可删（先建后删自洽）
	w = gr.do(t, http.MethodPost, "/api/admin/locations", `{"name":"临时点位","parent_id":`+strconv.FormatInt(zone.ID, 10)+`}`)
	var tmp struct {
		ID int64 `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &tmp)
	w = gr.do(t, http.MethodDelete, "/api/admin/locations/"+strconv.FormatInt(tmp.ID, 10), "")
	if w.Code != http.StatusOK {
		t.Fatalf("删空点位应 200: code=%d", w.Code)
	}
}

