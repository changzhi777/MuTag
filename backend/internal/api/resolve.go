package api

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/domain"
)

// errTagVoid 码已作废（对外表现为 404）。
var errTagVoid = errors.New("tag void")

// wechatUAFlag 微信内置浏览器 UA 标识。
const wechatUAFlag = "MicroMessenger"

// appUAFlag 青沐码 APK WebView UA 后缀标识（mobile/DetailScreen 追加）。
const appUAFlag = "QingMuApp"

// resolveTarget 活码分流决策（纯函数，便于单测）。
// 微信内扫码且已配置小程序跳转 → 小程序；其余一律 H5 只读详情。
// M2 起微信内目标切为小程序唤起链接，码本身无需重印。
func resolveTarget(userAgent, h5Path, miniJumpURL string) string {
	if miniJumpURL != "" && strings.Contains(userAgent, wechatUAFlag) {
		return miniJumpURL
	}
	return h5Path
}

// scanChannel 依 UA 归类扫码渠道（微信优先：APK 内 WebView 载入微信页场景仍归 weapp）。
func scanChannel(userAgent string) string {
	switch {
	case strings.Contains(userAgent, wechatUAFlag):
		return "weapp"
	case strings.Contains(userAgent, appUAFlag):
		return "app"
	default:
		return "h5"
	}
}

// resolve GET /t/:urlCode —— 活码 302 分流器。
// 缓存(60s) → 库兜底；不存在或已作废 → 404；每次写 scan_logs。
func resolve(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		urlCode := c.Param("urlCode")
		ua := c.GetHeader("User-Agent")
		ctx := c.Request.Context()

		tagID, err := lookupTagID(ctx, d, urlCode)
		if err != nil {
			c.String(http.StatusNotFound, "404: 码不存在或已作废")
			return
		}

		// 扫码留痕（失败不阻断分流，但必须留日志——0005 前渠道 CHECK 拒绝 app 曾静默丢数据一周）
		ch := scanChannel(ua)
		if err := d.Store.ScanLogs().Insert(ctx, tagID, "", ch, c.ClientIP(), ua); err != nil {
			log.Printf("[scan] insert scan_log failed: tagID=%s channel=%s err=%v", tagID, ch, err)
		}

		// M2 起配置 WEAPP_JUMP_URL 后，微信内扫码切小程序唤起
		target := resolveTarget(ua, "/h5/t/"+tagID, d.WeappJumpURL)
		c.Redirect(http.StatusFound, target)
	}
}

// lookupTagID 活码路径段 → tag_id：缓存 → 库兜底。
// 仅缓存 active 码；void/不存在每次落库校验（量小，防穿透靠 404 响应）。
func lookupTagID(ctx context.Context, d Deps, urlCode string) (string, error) {
	cacheKey := "reslv:" + urlCode
	if tagID, ok := d.Cache.Get(cacheKey); ok {
		return tagID, nil
	}

	tag, err := d.Store.Tags().GetByURLCode(ctx, urlCode)
	if err != nil {
		return "", err
	}
	if tag.Status != domain.TagActive {
		return "", errTagVoid
	}

	d.Cache.Set(cacheKey, tag.TagID)
	return tag.TagID, nil
}
