// 管理后台 API：台账 CRUD、码管理、批量导入、统计。
// 鉴权由 adminAuth 中间件统一处理；履历纪律：create/update/scrap 全写 asset_events。
package api

import (
	"fmt"
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

	// 0006 增强
	ParentTagID   *string  `json:"parent_tag_id"`
	LocationID    *int64   `json:"location_id"`
	Qty           *int     `json:"qty"`
	Brand         *string  `json:"brand"`
	KeeperPhone   *string  `json:"keeper_phone"`
	WarrantyUntil *string  `json:"warranty_until"` // YYYY-MM-DD
	Lat           *float64 `json:"lat"`
	Lng           *float64 `json:"lng"`
}

func (r *assetReq) toAsset() (*domain.Asset, error) {
	a := &domain.Asset{
		Name:        r.Name,
		Category:    r.Category,
		Model:       r.Model,
		Sn:          r.Sn,
		Price:       r.Price,
		DeptID:      r.DeptID,
		Keeper:      r.Keeper,
		Location:    r.Location,
		Unit:        r.Unit,
		Remark:      r.Remark,
		ParentTagID: r.ParentTagID,
		LocationID:  r.LocationID,
		Brand:       r.Brand,
		KeeperPhone: r.KeeperPhone,
		Lat:         r.Lat,
		Lng:         r.Lng,
	}
	if r.Qty != nil && *r.Qty > 0 {
		a.Qty = *r.Qty
	} else {
		a.Qty = 1
	}
	var err error
	if a.BuyDate, err = parseDay(r.BuyDate); err != nil {
		return nil, err
	}
	if a.WarrantyEnd, err = parseDay(r.WarrantyEnd); err != nil {
		return nil, err
	}
	if a.WarrantyUntil, err = parseDay(r.WarrantyUntil); err != nil {
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
		AssetID: assetID, Type: domain.EventCreate, Actor: actorOf(c),
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

	// 字段级 diff 快照（无旁路纪律）——含 0006 新字段（漏 diff = 变更不写履历，违反纪律 2）
	changed := map[string]any{}
	oldMap := map[string]any{
		"name": old.Name, "keeper": deref(old.Keeper), "location": deref(old.Location),
		"sn": deref(old.Sn), "model": deref(old.Model), "unit": deref(old.Unit),
		"remark": deref(old.Remark), "status": string(old.Status),
		"parent_tag_id": deref(old.ParentTagID), "location_id": idS(old.LocationID),
		"qty": strconv.Itoa(old.Qty), "brand": deref(old.Brand),
		"keeper_phone": deref(old.KeeperPhone), "warranty_until": dayStr(old.WarrantyUntil),
		"buy_date": dayStr(old.BuyDate), "price": priceS(old.Price),
	}
	newMap := map[string]any{
		"name": updated.Name, "keeper": deref(updated.Keeper), "location": deref(updated.Location),
		"sn": deref(updated.Sn), "model": deref(updated.Model), "unit": deref(updated.Unit),
		"remark": deref(updated.Remark), "status": string(updated.Status),
		"parent_tag_id": deref(updated.ParentTagID), "location_id": idS(updated.LocationID),
		"qty": strconv.Itoa(updated.Qty), "brand": deref(updated.Brand),
		"keeper_phone": deref(updated.KeeperPhone), "warranty_until": dayStr(updated.WarrantyUntil),
		"buy_date": dayStr(updated.BuyDate), "price": priceS(updated.Price),
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
			AssetID: old.AssetID, Type: domain.EventUpdate, Actor: actorOf(c),
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
		AssetID: asset.AssetID, Type: domain.EventScrap, Actor: actorOf(c),
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
// 幂等去重（dedup 默认 true）：每键跳过 min(C_db, C_in) 条、新建补差条（规划 P1 任务 2.3）。
func (d Deps) importAssets(c *gin.Context) {
	var req struct {
		Items []importItem `json:"items"`
		Dedup *bool        `json:"dedup"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		c.JSON(400, gin.H{"error": "items 为空"})
		return
	}
	dedup := req.Dedup == nil || *req.Dedup
	ctx := c.Request.Context()

	units := expandUnits(req.Items)
	existing, err := loadExistingCounts(ctx, d.Store.DB)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// create 侧按"额度耗尽即新建"在插入循环隐式处理，这里只需 skip 额度
	_, skip := classify(units, existing, dedup)
	// skip/create 各自保序；create 需要知道每条的 Seq/Name/Keeper/BuyDate/Unit——
	// 按 Key 反查原 item（同键多条共享 item 字段，qty 拆分仅影响条数）。

	created := 0
	skipped := len(skip)
	var lastTag string
	skipCountByKey := make(map[string]int)
	for _, u := range skip {
		skipCountByKey[u.Key]++
	}

	for _, rec := range req.Items {
		if strings.TrimSpace(rec.Name) == "" {
			continue
		}
		qty := rec.Qty
		if qty < 1 {
			qty = 1
		}
		bd, keeper := "", ""
		if rec.BuyDate != nil {
			bd = strings.TrimSpace(*rec.BuyDate)
		}
		if rec.Keeper != nil {
			keeper = strings.TrimSpace(*rec.Keeper)
		}
		key := dedupeKey(strings.TrimSpace(rec.Name), keeper, bd)
		var buyDate *time.Time
		if rec.BuyDate != nil && *rec.BuyDate != "" {
			buyDate, _ = parseDay(rec.BuyDate)
		}
		for j := 0; j < qty; j++ {
			if dedup && skipCountByKey[key] > 0 {
				// 该键还有"跳过额度"：本条为库内已有重复 → 跳过（顺序消耗与 classify 一致）
				skipCountByKey[key]--
				continue
			}
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
				AssetID: assetID, Type: domain.EventCreate, Actor: actorOf(c),
				NewValue: map[string]any{"name": rec.Name, "keeper": deref(rec.Keeper), "status": "in_use"},
				Note:     &note,
			})
			created++
			lastTag = tagID
		}
	}

	dupDetail := make([]gin.H, 0)
	for _, g := range buildGroups(units, existing) {
		if g.Existing > 0 {
			dupDetail = append(dupDetail, gin.H{
				"key": g.Key, "name": g.Name, "keeper": g.Keeper, "buy_date": g.BuyDate,
				"existing": g.Existing, "incoming": g.Incoming, "verdict": g.Verdict,
			})
		}
	}
	c.JSON(200, gin.H{
		"created": created, "skipped": skipped, "duplicates": dupDetail, "last_tag": lastTag,
	})
}

// importPrecheck POST /api/admin/import/precheck —— 只读预检：解析结果与库内比对的重复全貌（规划 P1 任务 2.3）。
func (d Deps) importPrecheck(c *gin.Context) {
	var req struct {
		Items []importItem `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		c.JSON(400, gin.H{"error": "items 为空"})
		return
	}
	ctx := c.Request.Context()
	units := expandUnits(req.Items)
	existing, err := loadExistingCounts(ctx, d.Store.DB)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	create, skip := classify(units, existing, true)
	c.JSON(200, gin.H{
		"groups":  buildGroups(units, existing),
		"summary": gin.H{"create": len(create), "skip_exact": len(skip), "suspect": countSuspect(create, existing)},
	})
}

// stats GET /api/admin/stats —— 状态分布 + 总数。
func (d Deps) stats(c *gin.Context) {	rows, err := d.Store.DB.QueryContext(c.Request.Context(),
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

// idS *int64 → 字符串（nil → ""，diff 比较用）。
func idS(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

// priceS *float64 → 两位小数字符串（nil → ""，diff 比较用）。
func priceS(p *float64) string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *p)
}

// dayStr *time.Time → YYYY-MM-DD（nil → ""，diff 比较用）。
func dayStr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func noteOf(changed map[string]any) *string {
	keys := make([]string, 0, len(changed))
	for k := range changed {
		keys = append(keys, k)
	}
	note := "变更字段：" + strings.Join(keys, "、")
	return &note
}

// scanStats GET /api/admin/tags/scan-stats —— 码扫码统计（全量：次数 + 最近扫码时间）。
// 码版面/码管理页展示用；scan_logs 体量小（内网资产系统），全量聚合一次返回。
func (d Deps) scanStats(c *gin.Context) {
	rows, err := d.Store.DB.QueryContext(c.Request.Context(),
		`SELECT tag_id, count(*), max(created_at) FROM scan_logs GROUP BY tag_id`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	stats := []gin.H{}
	for rows.Next() {
		var tagID, last string
		var count int64
		if err := rows.Scan(&tagID, &count, &last); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		stats = append(stats, gin.H{"tag_id": tagID, "count": count, "last": last})
	}
	c.JSON(200, gin.H{"items": stats})
}

// appConfig GET /api/admin/config —— 活码对外域名等前端配置（单一事实源：服务端 BASE_URL）。
// 码版面打印/批量生成的活码内容用它构造，避免随后台入口域名漂移。
func (d Deps) appConfig(c *gin.Context) {
	c.JSON(200, gin.H{"base_url": d.BaseURL})
}
