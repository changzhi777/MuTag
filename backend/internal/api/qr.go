// 活码 QR 图端点：GET/HEAD /qr/:urlCode —— 服务端渲染活码 URL 的 PNG 码图。
// 公开可拉（码内容即公开活码 URL）；复用 lookupTagID 校验（缓存 + void 语义与分流一致）；
// 限流同活码桶（纪律 4）；active 码响应长缓存（tagId 永不 recycle 纪律支撑）。
// 默认中心嵌青沐 logo（High 级纠错盖遮蔽）；?logo=0 出纯码。
package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/qr"
)

// qrPNG GET/HEAD /qr/:urlCode?size=512&logo=1 —— 活码 URL 的 PNG 码图。
func (d Deps) qrPNG(c *gin.Context) {
	urlCode := c.Param("urlCode")
	ctx := c.Request.Context()

	// 存在性校验：复用分流缓存通道（active 才出码，void/不存在 → 404 与活码语义一致）
	if _, err := lookupTagID(ctx, d, urlCode); err != nil {
		c.String(http.StatusNotFound, "404: 码不存在或已作废")
		return
	}

	size, err := strconv.Atoi(c.DefaultQuery("size", "512"))
	if err != nil {
		c.String(http.StatusBadRequest, "400: size 应为整数像素")
		return
	}
	withLogo := c.DefaultQuery("logo", "1") != "0"

	png, err := qr.Render(d.BaseURL+"/t/"+urlCode, size, withLogo)
	if err != nil {
		c.String(http.StatusBadRequest, "400: %v", err)
		return
	}

	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, "image/png", png)
}
