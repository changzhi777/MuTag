package h5

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"mutag/backend/internal/domain"
)

// TestRenderTemplates 模板渲染 smoke：语法/函数/字段引用错误在启动期即暴露。
func TestRenderTemplates(t *testing.T) {
	r, err := NewRenderer()
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}

	buy := time.Date(2026, 4, 23, 0, 0, 0, 0, time.Local)
	price := 12999.5
	keeper := "常智"
	unit := "个"
	asset := &domain.Asset{
		AssetID: 1, TagID: "MT-2026-000001", Name: "无线蓝牙罗技M720",
		BuyDate: &buy, Price: &price, Keeper: &keeper, Unit: &unit,
		Status: domain.AssetInUse,
	}
	events := []domain.AssetEvent{
		{EventID: 1, AssetID: 1, Type: domain.EventCreate, Actor: "seed",
			NewValue:  map[string]any{"name": asset.Name},
			Note:      ptr("来源采购行序号 1（第 1/1 件）"),
			CreatedAt: time.Now()},
	}

	var buf bytes.Buffer
	if err := r.tpl.ExecuteTemplate(&buf, "detail.html", map[string]any{
		"Asset": asset, "Events": events,
	}); err != nil {
		t.Fatalf("render detail.html: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"无线蓝牙罗技M720", "MT-2026-000001", "¥12,999.50", "资产履历", "入库", "在用"} {
		if !strings.Contains(out, want) {
			t.Errorf("detail.html 缺少 %q", want)
		}
	}
	// 空值字段应隐藏（Model/Sn/Location/Remark 均 nil）
	for _, hidden := range []string{">型号<", ">SN<", ">位置<", ">备注<"} {
		if strings.Contains(out, hidden) {
			t.Errorf("detail.html 空值字段 %q 未隐藏", hidden)
		}
	}

	buf.Reset()
	if err := r.tpl.ExecuteTemplate(&buf, "notfound.html", map[string]any{
		"Title": "二维码无效", "Desc": "该码号不存在，请核对后重试",
	}); err != nil {
		t.Fatalf("render notfound.html: %v", err)
	}
	if !strings.Contains(buf.String(), "二维码无效") {
		t.Error("notfound.html 缺少标题")
	}
}

func ptr(s string) *string { return &s }
