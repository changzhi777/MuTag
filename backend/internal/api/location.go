// 位置管理 API（0006）：两级位置树 CRUD，删除前校验子节点与资产引用。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/domain"
)

// locationReq 位置创建/编辑请求。
type locationReq struct {
	Name     string   `json:"name"`
	ParentID *int64   `json:"parent_id"` // NULL=一级区域，非空=二级点位
	Lat      *float64 `json:"lat"`
	Lng      *float64 `json:"lng"`
}

// listLocations GET /api/admin/locations —— 全量（体量小，客户端组树）。
func (d Deps) listLocations(c *gin.Context) {
	items, err := d.Store.Locations().ListAll(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []domain.Location{}
	}
	c.JSON(200, gin.H{"items": items})
}

// createLocation POST /api/admin/locations。
func (d Deps) createLocation(c *gin.Context) {
	var req locationReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(400, gin.H{"error": "name 必填"})
		return
	}
	if req.ParentID != nil { // 二级点位必须挂在一级下（parent 本身不能有 parent）
		parent, err := d.Store.Locations().Get(c.Request.Context(), *req.ParentID)
		if err != nil {
			c.JSON(400, gin.H{"error": "parent 不存在"})
			return
		}
		if parent.ParentID != nil {
			c.JSON(400, gin.H{"error": "位置层级最多两级（点位不能挂点位）"})
			return
		}
	}
	l := &domain.Location{Name: req.Name, ParentID: req.ParentID, Lat: req.Lat, Lng: req.Lng}
	id, err := d.Store.Locations().Create(c.Request.Context(), l)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"id": id})
}

// updateLocation PUT /api/admin/locations/:id。
func (d Deps) updateLocation(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "id 非法"})
		return
	}
	var req locationReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(400, gin.H{"error": "name 必填"})
		return
	}
	if req.ParentID != nil && *req.ParentID == id {
		c.JSON(400, gin.H{"error": "parent 不能是自身"})
		return
	}
	l := &domain.Location{ID: id, Name: req.Name, ParentID: req.ParentID, Lat: req.Lat, Lng: req.Lng}
	if err := d.Store.Locations().Update(c.Request.Context(), l); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// deleteLocation DELETE /api/admin/locations/:id —— 子节点或资产引用存在时拒绝。
func (d Deps) deleteLocation(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"error": "id 非法"})
		return
	}
	ctx := c.Request.Context()
	if n, _ := d.Store.Locations().CountChildren(ctx, id); n > 0 {
		c.JSON(400, gin.H{"error": "该位置下有二级点位，先删点位"})
		return
	}
	if n, _ := d.Store.Locations().CountAssets(ctx, id); n > 0 {
		c.JSON(400, gin.H{"error": "仍有资产引用该位置，先改资产的安装位置"})
		return
	}
	if err := d.Store.Locations().Delete(ctx, id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
