// Package api HTTP 路由与处理器：活码 302 分流、H5 详情、JSON API、健康检查。
package api

import (
	"context"
	"database/sql"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/adminui"
	"mutag/backend/internal/cache"
	"mutag/backend/internal/h5"
	"mutag/backend/internal/repo"
)

// Deps API 层依赖集合。
type Deps struct {
	DB            *sql.DB
	Store         *repo.Store
	Cache         *cache.TTLCache
	Limiter       *cache.Limiter // 扫码限流（nil = 关闭；纪律 4）
	H5            *h5.Renderer
	AdminPassword string // 管理后台密码（空 = 免鉴权）
	WechatAppID   string // 小程序 AppID（空 = dev 模式登录）
	WechatSecret  string // 小程序 AppSecret
	TokenSecret   string // 小程序会话 token 签名密钥
	WeappJumpURL  string // 活码 302 微信内跳转目标（配置后启用小程序唤起）
}

// rateLimit 扫码限流中间件：按 ClientIP 令牌桶（活码/H5 匿名扫码无 openid）。
func rateLimit(l *cache.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if l == nil || l.Allow(c.ClientIP()) {
			c.Next()
			return
		}
		c.String(http.StatusTooManyRequests, "429: 扫码过于频繁，请稍后再试")
		c.Abort()
	}
}

// NewRouter 注册全部路由。
func NewRouter(d Deps) *gin.Engine {
	// 内置财务模板幂等播种（生产启动 / e2e 建路由均触发，保证模板可用）
	if d.DB != nil && d.Store != nil {
		if err := repo.EnsureFinDefaults(context.Background(), d.DB); err != nil {
			log.Printf("seed fin defaults: %v", err)
		}
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/healthz", health(d))
	r.HEAD("/healthz", health(d))

	// 活码分流器：码不变，目标可换（HEAD 兼容扫码枪探测）；限流防爬（纪律 4）
	r.GET("/t/:urlCode", rateLimit(d.Limiter), resolve(d))
	r.HEAD("/t/:urlCode", rateLimit(d.Limiter), resolve(d))

	// H5 只读详情页（同一限流桶——爬虫可绕过 302 直打 H5）
	r.GET("/h5/t/:tagID", rateLimit(d.Limiter), d.H5.DetailHandler(d.Store.Tags(), d.Store.Assets(), d.Store.Events()))

	// JSON API（小程序数据源）
	r.GET("/api/v1/assets/:tagID", assetDetail(d))

	// 小程序员工端（登录/身份/快捷操作/我的上报）
	mp := r.Group("/api/v1")
	{
		mp.POST("/auth/login", d.mpLogin)
		authed := mp.Group("", userAuth(d.TokenSecret))
		authed.GET("/auth/me", d.mpMe)
		authed.GET("/auth/my-events", d.mpMyEvents)
		authed.POST("/assets/:tagID/actions", d.mpAction)
	}

	// 管理后台 API（鉴权除 login 外全覆盖）
	admin := r.Group("/api/admin")
	{
		admin.POST("/login", adminLogin(d.AdminPassword))
		authed := admin.Group("", adminAuth(d.AdminPassword))
		authed.GET("/assets", d.listAssets)
		authed.POST("/assets", d.createAsset)
		authed.PUT("/assets/:tagID", d.updateAsset)
		authed.GET("/tags", d.listTags)
		authed.POST("/tags/:tagID/void", d.voidTag)
		authed.POST("/import", d.importAssets)
		authed.GET("/stats", d.stats)

		// M4 财务对接：模板 / 导出审计 / 导入预览确认回滚
		fin := authed.Group("/fin")
		{
			fin.GET("/mappings", d.listMappings)
			fin.PUT("/mappings/:mappingID", d.updateMapping)
			fin.POST("/export", d.exportRows)
			fin.GET("/export/jobs", d.listExportJobs)
			fin.POST("/import/preview", d.importPreview)
			fin.GET("/import/batches", d.listBatches)
			fin.POST("/import/batches/:batchID/confirm", d.confirmBatch)
			fin.POST("/import/batches/:batchID/rollback", d.rollbackBatch)
		}
	}

	// 管理后台 SPA（构建产物内嵌）
	adminui.Register(r)

	return r
}
