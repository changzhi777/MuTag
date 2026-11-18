package api

// QR 端点 e2e：/qr/:urlCode 服务端码图（渲染正确性 / 参数限幅 / 语义一致性）。
import (
	"encoding/binary"
	"net/http"
	"testing"
)

// pngWidth 解析 PNG IHDR 宽度（签名 8 + len 4 + "IHDR" 4 后的前 4 字节）。
func pngWidth(t *testing.T, png []byte) uint32 {
	t.Helper()
	if len(png) < 24 || string(png[12:16]) != "IHDR" {
		t.Fatalf("非合法 PNG：len=%d", len(png))
	}
	return binary.BigEndian.Uint32(png[16:20])
}

func TestE2EQRCode(t *testing.T) {
	gr := newTestRouter(t)

	// 1) active 码：200 + PNG + 缓存头
	w := gr.do(t, http.MethodGet, "/qr/MT-2026-000001", "")
	if w.Code != http.StatusOK {
		t.Fatalf("active 码出图应 200，got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/png" {
		t.Fatalf("Content-Type 应 image/png，got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc == "" {
		t.Fatalf("active 码应带 Cache-Control")
	}
	if string(w.Body.Bytes()[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("响应体不是 PNG")
	}

	// 2) HEAD 兼容（扫码枪纪律同款）
	if w := gr.do(t, http.MethodHead, "/qr/MT-2026-000001", ""); w.Code != http.StatusOK {
		t.Fatalf("HEAD 应 200，got %d", w.Code)
	}

	// 3) size 缺省 512
	if w := gr.do(t, http.MethodGet, "/qr/MT-2026-000001", ""); pngWidth(t, w.Body.Bytes()) != 512 {
		t.Fatalf("缺省尺寸应 512，got %d", pngWidth(t, w.Body.Bytes()))
	}

	// 4) size 限幅：超大钳到 1024
	w = gr.do(t, http.MethodGet, "/qr/MT-2026-000001?size=99999", "")
	if w.Code != http.StatusOK || pngWidth(t, w.Body.Bytes()) != 1024 {
		t.Fatalf("超大 size 应钳到 1024，got code=%d width=%d", w.Code, pngWidth(t, w.Body.Bytes()))
	}

	// 5) size 非法 → 400
	if w := gr.do(t, http.MethodGet, "/qr/MT-2026-000001?size=abc", ""); w.Code != http.StatusBadRequest {
		t.Fatalf("非法 size 应 400，got %d", w.Code)
	}

	// 6) 不存在码 → 404
	if w := gr.do(t, http.MethodGet, "/qr/MT-2026-999999", ""); w.Code != http.StatusNotFound {
		t.Fatalf("不存在码应 404，got %d", w.Code)
	}

	// 7) void 码 → 404（与活码分流语义一致）
	if w := gr.do(t, http.MethodPost, "/api/admin/tags/MT-2026-000002/void", ""); w.Code != http.StatusOK {
		t.Fatalf("void 前置失败：%d", w.Code)
	}
	if w := gr.do(t, http.MethodGet, "/qr/MT-2026-000002", ""); w.Code != http.StatusNotFound {
		t.Fatalf("void 码应 404，got %d", w.Code)
	}

	// 8) 默认带 logo（bytes > 纯码）；logo=0 出纯码
	withLogo := gr.do(t, http.MethodGet, "/qr/MT-2026-000003", "")
	if withLogo.Code != http.StatusOK {
		t.Fatalf("logo 版应 200，got %d", withLogo.Code)
	}
	noLogo := gr.do(t, http.MethodGet, "/qr/MT-2026-000003?logo=0", "")
	if noLogo.Code != http.StatusOK {
		t.Fatalf("纯码版应 200，got %d", noLogo.Code)
	}
	if withLogo.Body.Len() <= noLogo.Body.Len() {
		t.Fatalf("logo 版 bytes（%d）应大于纯码版（%d）", withLogo.Body.Len(), noLogo.Body.Len())
	}
}
