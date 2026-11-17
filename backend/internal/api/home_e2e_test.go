package api

// 根路径落地页 + NoRoute 兜底 e2e：/ 与未知路径的 HTML 语义、/api/* 的 JSON 语义、SPA fallback 不受影响。
import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestE2EHomeAndNoRoute(t *testing.T) {
	gr := newTestRouter(t)

	// 1) 根路径落地页：200 + 品牌 + 后台入口
	w := gr.do(t, http.MethodGet, "/", "")
	if w.Code != http.StatusOK {
		t.Fatalf("根路径应 200，got %d", w.Code)
	}
	out := w.Body.String()
	for _, want := range []string{"MuTag 动态码", "管理后台", "青沐科技"} {
		if !strings.Contains(out, want) {
			t.Errorf("落地页缺少 %q", want)
		}
	}

	// 2) 未知路径：404 + HTML 友好页（非 gin 纯文本）
	w = gr.do(t, http.MethodGet, "/no-such-page", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("未知路径应 404，got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "页面不存在") {
		t.Error("未知路径应返回 HTML 兜底页")
	}

	// 3) API 未知路径：404 + JSON 语义
	w = gr.do(t, http.MethodGet, "/api/v1/no-such", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("API 未知路径应 404，got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("API 未知路径应 JSON，got %q", ct)
	}

	// 4) SPA fallback 不受 NoRoute 影响：/admin/history-route 回 index.html
	req := httptest.NewRequest(http.MethodGet, "/admin/assets/MT-2026-000001", nil)
	rec := httptest.NewRecorder()
	gr.r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("SPA history 路由应 200 回 index.html，got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<script") {
		t.Error("SPA fallback 应返回 index.html")
	}
}
