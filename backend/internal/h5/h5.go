// Package h5 服务端渲染的只读详情页（html/template + embed，零外部依赖）。
// CSS 全内联：一次请求完成渲染，无外链资源。
package h5

import (
	"embed"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/domain"
	"mutag/backend/internal/repo"
)

//go:embed templates/*.html
var files embed.FS

func mustServe(c *gin.Context, status int) {
	c.Status(status)
}

// DetailHandler GET /h5/t/:tagID —— 只读详情页。
// 语义：码不存在 → 404「二维码无效」；码已作废(void) → 200「该资产码已作废」。
func (r *Renderer) DetailHandler(tags *repo.TagRepo, assets *repo.AssetRepo, events *repo.EventRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tagID := c.Param("tagID")

		tag, err := tags.GetByTagID(ctx, tagID)
		if err != nil {
			mustServe(c, http.StatusNotFound)
			_ = r.tpl.ExecuteTemplate(c.Writer, "notfound.html", map[string]any{
				"Title": "二维码无效",
				"Desc":  "该码号不存在，请核对后重试",
			})
			return
		}
		if tag.Status != domain.TagActive {
			mustServe(c, http.StatusOK)
			_ = r.tpl.ExecuteTemplate(c.Writer, "notfound.html", map[string]any{
				"Title": "该资产码已作废",
				"Desc":  "此标签已注销，如有疑问请联系资产管理员",
			})
			return
		}

		asset, err := assets.GetByTagID(ctx, tagID)
		if err != nil {
			mustServe(c, http.StatusNotFound)
			_ = r.tpl.ExecuteTemplate(c.Writer, "notfound.html", map[string]any{
				"Title": "二维码无效",
				"Desc":  "该码号不存在，请核对后重试",
			})
			return
		}

		eventList, err := events.ListByAsset(ctx, asset.AssetID, 50)
		if err != nil {
			eventList = nil // 履历查询失败不阻断详情渲染
		}

		mustServe(c, http.StatusOK)
		_ = r.tpl.ExecuteTemplate(c.Writer, "detail.html", map[string]any{
			"Asset":  asset,
			"Events": eventList,
		})
	}
}

// Renderer H5 渲染器（模板 + 设计系统 FuncMap）。
type Renderer struct {
	tpl *template.Template
}

// NewRenderer 解析内嵌模板并挂载格式化函数。
func NewRenderer() (*Renderer, error) {
	tpl, err := template.New("h5").Funcs(template.FuncMap{
		"statusClass": statusClass,
		"eventClass":  eventClass,
		"eventLabel":  eventLabel,
		"statusLabel": statusLabel,
		"fmtDateT":    fmtDateT,
		"fmtMoney":    fmtMoney,
		"fmtEventT":   fmtEventT,
		"fmtRFC3339":  fmtRFC3339,
		"nz":          nz,
	}).ParseFS(files, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Renderer{tpl: tpl}, nil
}

// nz 指针字符串非 nil 且非空白（KV 空值行隐藏判定）。
func nz(s *string) bool {
	return s != nil && strings.TrimSpace(*s) != ""
}

// statusClass 资产状态 → CSS 类名（设计 token：st-*）。
func statusClass(s domain.AssetStatus) string {
	switch s {
	case domain.AssetInUse:
		return "st-active"
	case domain.AssetIdle:
		return "st-idle"
	case domain.AssetRepair:
		return "st-repair"
	case domain.AssetBorrow:
		return "st-lent"
	case domain.AssetScrap:
		return "st-scrap"
	}
	return "st-scrap"
}

func statusLabel(s domain.AssetStatus) string {
	switch s {
	case domain.AssetInUse:
		return "在用"
	case domain.AssetIdle:
		return "闲置"
	case domain.AssetRepair:
		return "维修中"
	case domain.AssetBorrow:
		return "借出"
	case domain.AssetScrap:
		return "已报废"
	}
	return string(s)
}

// eventClass 履历类型 → CSS 类名（设计 token：ev-*）。
func eventClass(e domain.EventType) string {
	switch e {
	case domain.EventCreate:
		return "ev-inbound"
	case domain.EventReceive:
		return "ev-claim"
	case domain.EventReturn:
		return "ev-return"
	case domain.EventRepair:
		return "ev-repair"
	case domain.EventScrap:
		return "ev-scrap"
	case domain.EventTransfer:
		return "ev-move"
	}
	return "ev-other"
}

func eventLabel(e domain.EventType) string {
	switch e {
	case domain.EventCreate:
		return "入库"
	case domain.EventReceive:
		return "领用"
	case domain.EventReturn:
		return "归还"
	case domain.EventRepair:
		return "维修"
	case domain.EventScrap:
		return "报废"
	case domain.EventTransfer:
		return "调拨"
	case domain.EventUpdate:
		return "信息变更"
	}
	return string(e)
}

// fmtDateT 资产采购日期 → YYYY-MM-DD。
func fmtDateT(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// fmtMoney 金额 → ¥千分位两位小数。
func fmtMoney(p *float64) string {
	if p == nil {
		return ""
	}
	neg := *p < 0
	v := math.Abs(*p)
	s := fmt.Sprintf("%.2f", v)
	intPart := s[:len(s)-3]
	dec := s[len(s)-3:]
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	out := "¥" + b.String() + dec
	if neg {
		return "-" + out
	}
	return out
}

// fmtRFC3339 履历时间的机器可读格式（<time datetime> 用）。
func fmtRFC3339(t time.Time) string {
	return t.Format(time.RFC3339)
}

// fmtEventT 履历时间：当年 MM-DD HH:mm，跨年完整日期。
func fmtEventT(t time.Time) string {
	if t.Year() == time.Now().Year() {
		return t.Format("01-02 15:04")
	}
	return t.Format("2006-01-02 15:04")
}
