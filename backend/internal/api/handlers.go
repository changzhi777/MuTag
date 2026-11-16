package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/domain"
)

// assetDetail GET /api/v1/assets/:tagID —— 小程序/H5 共用的 JSON 数据源。
// 返回资产基本信息 + 履历（新→旧，上限 50 条）。
func assetDetail(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		tagID := c.Param("tagID")
		ctx := c.Request.Context()

		asset, err := d.Store.Assets().GetByTagID(ctx, tagID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "asset not found"})
			return
		}

		events, err := d.Store.Events().ListByAsset(ctx, asset.AssetID, 50)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "query events failed"})
			return
		}
		if len(events) == 0 {
			events = make([]domain.AssetEvent, 0)
		}

		c.JSON(http.StatusOK, gin.H{
			"asset":  asset,
			"events": events,
		})
	}
}

// health GET /healthz —— 存活探针（compose healthcheck / verify.sh 用）。
func health(d Deps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		status := gin.H{"status": "ok", "db": "up"}
		if err := d.DB.PingContext(ctx); err != nil {
			status["db"] = "down"
			status["status"] = "degraded"
		}
		c.JSON(http.StatusOK, status)
	}
}
