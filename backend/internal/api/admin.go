// 管理后台 API：台账 CRUD、码管理、批量导入、统计。
// 鉴权由 adminAuth 中间件统一处理；履历纪律：create/update/scrap 全写 asset_events。
package api

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/domain"
	"mutag/backend/internal/repo"
)

// assetReq 管理端资产编辑请求（tagID 不可改，由码表绑定）。
type assetReq struct {
	Name        string   `json:"name"`
	Category    *string  `json:"category"`
	Model       *string  `json:"model"`
	Sn          *string  `json:"sn"`
	BuyDate     *string  `json:"buy_date"` // YYYY-MM-DD
	Price       *float64 `json:"price"`
	DeptID      *string  `json:"dept_id"`
	Keeper      *string  `json:"keeper"`
	Location    *string  `json:"location"`
	Status      *string  `json:"status"`
	Unit        *string  `json:"unit"`
	WarrantyEnd *string  `json:"warranty_end"`
	Remark      *string  `json:"remark"`
}

func (r *assetReq) toAsset() (*domain.Asset, error) {
	a := &domain.Asset{
		Name:     r.Name,
		Category: r.Category,
		Model:    r.Model,
		Sn:       r.Sn,
		Price:    r.Price,
		DeptID:   r.DeptID,
		Keeper:   r.Keeper,
		Location: r.Location,
		Unit:     r.Unit,
		Remark:   r.Remark,
	}
	var err error
	if a.BuyDate, err = parseDay(r.BuyDate); err != nil {
		return nil, err
	}
	if a.WarrantyEnd, err = parseDay(r.WarrantyEnd); err != nil {
		return nil, err
	}
	a.Status = domain.AssetInUse
	if r.Status != nil {
		a.Status = domain.AssetStatus(*r.Status)
	}
	return a, nil
}

func parseDay(s *string) (*time.Time, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", *s, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// listAssets GET /api/admin/assets?keyword&status&page&size
func (d Deps) listAssets(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	items, total, err := d.Store.Assets().List(c.Request.Context(), repo.AssetFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Status:  c.Query("status"),
		Page:    page, Size: size,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []domain.Asset{}
	}
	c.JSON(200, gin.H{"total": total, "items": items})
}

// createAsset POST /api/admin/assets —— 自动配码 + create 履历。
func (d Deps) createAsset(c *gin.Context) {
	var req assetReq
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(400, gin.H{"error": "name 必填"})
		return
	}
	ctx := c.Request.Context()

	tagID, err := d.Store.Tags().NextTagID(ctx)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	asset, err := req.toAsset()
	if err != nil {
		c.JSON(400, gin.H{"error": "日期格式应为 YYYY-MM-DD"})
		return
	}
	asset.TagID = tagID

	if err := d.Store.Tags().Insert(ctx, &domain.Tag{
		TagID: tagID, QrScene: "t:" + tagID, UrlCode: tagID, Status: domain.TagActive,
	}); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	assetID, err := d.Store.Assets().Create(ctx, asset)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	note := "管理后台新建"
	_ = d.Store.Events().Create(ctx, &domain.AssetEvent{
		AssetID: assetID, Type: domain.EventCreate, Actor: "admin",
		NewValue: map[string]any{"name": asset.Name, "keeper": deref(asset.Keeper), "status": string(asset.Status)},
		Note:     &note,
	})
	c.JSON(200, gin.H{"tag_id": tagID, "asset_id": assetID})
}

// updateAsset PUT /api/admin/assets/:tagID —— diff 快照写 update 履历。
func (d Deps) updateAsset(c *gin.Context) {
	tagID := c.Param("tagID")
	var req assetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()

	old, err := d.Store.Assets().GetByTagID(ctx, tagID)
	if err != nil {
		c.JSON(404, gin.H{"error": "asset not found"})
		return
	}
	updated, err := req.toAsset()
	if err != nil {
		c.JSON(400, gin.H{"error": "日期格式应为 YYYY-MM-DD"})
		return
	}
	updated.TagID = tagID
	updated.AssetID = old.AssetID

	// 字段级 diff 快照（无旁路纪律）
	changed := map[string]any{}
	oldMap := map[string]any{
		"name": old.Name, "keeper": deref(old.Keeper), "location": deref(old.Location),
		"sn": deref(old.Sn), "model": deref(old.Model), "unit": deref(old.Unit),
		"remark": deref(old.Remark), "status": string(old.Status),
	}
	newMap := map[string]any{
		"name": updated.Name, "keeper": deref(updated.Keeper), "location": deref(updated.Location),
		"sn": deref(updated.Sn), "model": deref(updated.Model), "unit": deref(updated.Unit),
		"remark": deref(updated.Remark), "status": string(updated.Status),
	}
	for k, ov := range oldMap {
		if nv := newMap[k]; nv != ov {
			changed[k] = gin.H{"old": ov, "new": nv}
		}
	}

	if err := d.Store.Assets().Update(ctx, updated); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if len(changed) > 0 {
		_ = d.Store.Events().Create(ctx, &domain.AssetEvent{
			AssetID: old.AssetID, Type: domain.EventUpdate, Actor: "admin",
			OldValue: oldMap, NewValue: newMap,
			Note: noteOf(changed),
		})
	}
	c.JSON(200, gin.H{"ok": true, "changed": len(changed)})
}

// voidTag POST /api/admin/tags/:tagID/void —— 码作废 + 资产报废 + scrap 履历。
func (d Deps) voidTag(c *gin.Context) {
	tagID := c.Param("tagID")
	ctx := c.Request.Context()

	asset, err := d.Store.Assets().GetByTagID(ctx, tagID)
	if err != nil {
		c.JSON(404, gin.H{"error": "asset not found"})
		return
	}
	if err := d.Store.Tags().SetStatus(ctx, tagID, domain.TagVoid); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if err := d.Store.Assets().SetStatus(ctx, tagID, domain.AssetScrap); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	note := "管理后台作废码并报废资产"
	_ = d.Store.Events().Create(ctx, &domain.AssetEvent{
		AssetID: asset.AssetID, Type: domain.EventScrap, Actor: "admin",
		OldValue: map[string]any{"tag_status": "active", "status": string(asset.Status)},
		NewValue: map[string]any{"tag_status": "void", "status": "scrapped"},
		Note:     &note,
	})
	c.JSON(200, gin.H{"ok": true})
}

// listTags GET /api/admin/tags?status&page&size
func (d Deps) listTags(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	tags, total, err := d.Store.Tags().List(c.Request.Context(), c.Query("status"), page, size)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if tags == nil {
		tags = []domain.Tag{}
	}
	c.JSON(200, gin.H{"total": total, "items": tags})
}

// importItem 批量导入条目（前端 SheetJS 解析后的 JSON）。
type importItem struct {
	Seq     string  `json:"seq"`
	Name    string  `json:"name"`
	BuyDate *string `json:"buy_date"`
	Qty     int     `json:"qty"`
	Unit    *string `json:"unit"`
	Keeper  *string `json:"keeper"`
}

// importAssets POST /api/admin/import —— 一物一码拆分入库（同 seed 纪律）。
func (d Deps) importAssets(c *gin.Context) {
	var req struct {
		Items []importItem `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		c.JSON(400, gin.H{"error": "items 为空"})
		return
	}
	ctx := c.Request.Context()

	created := 0
	var lastTag string
	for _, rec := range req.Items {
		if strings.TrimSpace(rec.Name) == "" {
			continue
		}
		qty := rec.Qty
		if qty < 1 {
			qty = 1
		}
		for j := 0; j < qty; j++ {
			tagID, err := d.Store.Tags().NextTagID(ctx)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error(), "created": created})
				return
			}
			if err := d.Store.Tags().Insert(ctx, &domain.Tag{
				TagID: tagID, QrScene: "t:" + tagID, UrlCode: tagID, Status: domain.TagActive,
			}); err != nil {
				c.JSON(500, gin.H{"error": err.Error(), "created": created})
				return
			}
			var buyDate *time.Time
			if rec.BuyDate != nil && *rec.BuyDate != "" {
				buyDate, _ = parseDay(rec.BuyDate)
			}
			assetID, err := d.Store.Assets().Create(ctx, &domain.Asset{
				TagID: tagID, Name: strings.TrimSpace(rec.Name),
				BuyDate: buyDate, Keeper: rec.Keeper, Status: domain.AssetInUse, Unit: rec.Unit,
			})
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error(), "created": created})
				return
			}
			note := "批量导入（来源行序号 " + rec.Seq + "）"
			_ = d.Store.Events().Create(ctx, &domain.AssetEvent{
				AssetID: assetID, Type: domain.EventCreate, Actor: "admin",
				NewValue: map[string]any{"name": rec.Name, "keeper": deref(rec.Keeper), "status": "in_use"},
				Note:     &note,
			})
			created++
			lastTag = tagID
		}
	}
	c.JSON(200, gin.H{"created": created, "last_tag": lastTag})
}

// stats GET /api/admin/stats —— 状态分布 + 总数。
func (d Deps) stats(c *gin.Context) {
	rows, err := d.Store.DB.QueryContext(c.Request.Context(),
		`SELECT status, count(*) FROM assets GROUP BY status`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	byStatus := []gin.H{}
	total := int64(0)
	for rows.Next() {
		var s string
		var n int64
		if err := rows.Scan(&s, &n); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		byStatus = append(byStatus, gin.H{"status": s, "count": n})
		total += n
	}
	c.JSON(200, gin.H{"total": total, "by_status": byStatus})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func noteOf(changed map[string]any) *string {
	keys := make([]string, 0, len(changed))
	for k := range changed {
		keys = append(keys, k)
	}
	note := "变更字段：" + strings.Join(keys, "、")
	return &note
}
