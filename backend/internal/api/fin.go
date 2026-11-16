// fin.go 财务对接 API：模板配置、导出（含审计留痕）、导入（预览→确认→按批次回滚）。
// 纪律：格式转换全部收敛在映射层（模板 def）；tag_id 只读永不可导入；导出必留审计。
package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/domain"
	"mutag/backend/internal/repo"
)

// newID 业务主键生成：前缀 + 纳秒基36 + 随机后缀。
func newID(prefix string) string {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	return prefix + "-" + strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + hex.EncodeToString(buf)
}

// ---------------------------------------------------------------------------
// 模板配置
// ---------------------------------------------------------------------------

// listMappings GET /api/admin/fin/mappings —— 全部模板（含内置默认）。
func (d Deps) listMappings(c *gin.Context) {
	items, err := d.Store.Mappings().List(c.Request.Context())
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []domain.FinMapping{}
	}
	c.JSON(200, gin.H{"items": items})
}

// updateMappingReq 模板编辑请求：整份 def 替换（version 递增）。
type updateMappingReq struct {
	Def domain.FinMappingDef `json:"def"`
}

var validFinFields = func() map[string]bool {
	m := map[string]bool{"tag_id": true}
	for f := range repo.ImportableFields() {
		m[f] = true
	}
	return m
}()

// updateMapping PUT /api/admin/fin/mappings/:mappingID
func (d Deps) updateMapping(c *gin.Context) {
	var req updateMappingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// 白名单校验：字段映射只允许系统已知字段
	for _, col := range req.Def.Columns {
		if strings.TrimSpace(col.Name) == "" || strings.TrimSpace(col.Field) == "" {
			c.JSON(400, gin.H{"error": "列名与系统字段均必填"})
			return
		}
		if !validFinFields[col.Field] {
			c.JSON(400, gin.H{"error": "未知系统字段：" + col.Field})
			return
		}
	}
	if err := d.Store.Mappings().Update(c.Request.Context(), c.Param("mappingID"), &req.Def); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	m, _ := d.Store.Mappings().Get(c.Request.Context(), c.Param("mappingID"))
	c.JSON(200, gin.H{"ok": true, "mapping": m})
}

// ---------------------------------------------------------------------------
// 导出（映射层格式转换 + 审计留痕）
// ---------------------------------------------------------------------------

// exportReq 导出请求：过滤条件 + 模板。
type exportReq struct {
	MappingID string `json:"mapping_id"`
	Status    string `json:"status"`
	Dept      string `json:"dept"`
	BuyFrom   string `json:"buy_from"` // YYYY-MM-DD
	BuyTo     string `json:"buy_to"`
	Keyword   string `json:"keyword"`
}

// exportRows POST /api/admin/fin/export —— 按模板输出行 + 写导出审计。
// XLSX 由前端 SheetJS 生成下载（M3 裁决：xlsx 解析/生成在前端，后端零新依赖）。
func (d Deps) exportRows(c *gin.Context) {
	var req exportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()

	mapping, err := d.Store.Mappings().Get(ctx, req.MappingID)
	if err != nil {
		c.JSON(404, gin.H{"error": "模板不存在"})
		return
	}

	assets, _, err := d.Store.Assets().List(ctx, repo.AssetFilter{
		Keyword: strings.TrimSpace(req.Keyword), Status: req.Status, Dept: req.Dept,
		BuyFrom: req.BuyFrom, BuyTo: req.BuyTo,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	rows := make([][]string, 0, len(assets))
	for _, a := range assets {
		rows = append(rows, exportRowOf(mapping, &a))
	}

	// 审计留痕：谁 / 何时 / 哪个模板 / 什么过滤条件 / 导出几行（财务合规）
	job := &domain.ExportJob{
		JobID: newID("job"), MappingID: req.MappingID, Actor: "admin",
		Filters: map[string]any{
			"status": req.Status, "dept": req.Dept,
			"buy_from": req.BuyFrom, "buy_to": req.BuyTo, "keyword": req.Keyword,
		},
		RowCount: int64(len(rows)),
	}
	if err := d.Store.ExportJobs().Create(ctx, job); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"job_id": job.JobID, "row_count": len(rows),
		"columns":    mapping.Def.Columns,
		"fixed_rows": mapping.Def.FixedRows,
		"rows":       rows,
	})
}

// exportRowOf 资产 → 模板行（日期/金额/状态格式转换全部收敛在此——纪律 7）。
func exportRowOf(m *domain.FinMapping, a *domain.Asset) []string {
	row := make([]string, 0, len(m.Def.Columns))
	for _, col := range m.Def.Columns {
		row = append(row, fieldText(m, a, col.Field))
	}
	return row
}

// fieldText 系统字段 → 模板文本。
func fieldText(m *domain.FinMapping, a *domain.Asset, field string) string {
	switch field {
	case "tag_id":
		return a.TagID
	case "name":
		return a.Name
	case "price":
		if a.Price == nil {
			return ""
		}
		return fmt.Sprintf("%.2f", *a.Price)
	case "buy_date", "warranty_end":
		t := a.BuyDate
		if field == "warranty_end" {
			t = a.WarrantyEnd
		}
		if t == nil {
			return ""
		}
		return fmtDateByFormat(*t, m.Def.DateFormat)
	case "status":
		if s, ok := m.Def.StatusMap[string(a.Status)]; ok && s != "" {
			return s
		}
		return string(a.Status)
	default:
		// category/model/sn/dept_id/keeper/location/unit/remark 同构文本字段
		switch field {
		case "category":
			return deref(a.Category)
		case "model":
			return deref(a.Model)
		case "sn":
			return deref(a.Sn)
		case "dept_id":
			return deref(a.DeptID)
		case "keeper":
			return deref(a.Keeper)
		case "location":
			return deref(a.Location)
		case "unit":
			return deref(a.Unit)
		case "remark":
			return deref(a.Remark)
		}
		return ""
	}
}

// fmtDateByFormat 按模板日期格式输出（支持 YYYY-MM-DD / YYYY/MM/DD，默认前者）。
func fmtDateByFormat(t time.Time, format string) string {
	switch format {
	case "YYYY/MM/DD":
		return t.Format("2006/01/02")
	default:
		return t.Format("2006-01-02")
	}
}

// listExportJobs GET /api/admin/fin/export/jobs —— 导出审计记录。
func (d Deps) listExportJobs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, err := d.Store.ExportJobs().List(c.Request.Context(), limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []domain.ExportJob{}
	}
	c.JSON(200, gin.H{"items": items})
}

// ---------------------------------------------------------------------------
// 导入（preview → confirm → rollback）
// ---------------------------------------------------------------------------

// previewReq 导入预览请求：前端 SheetJS 解析出的原始行列 JSON + 模板。
type previewReq struct {
	MappingID string           `json:"mapping_id"`
	FileName  string           `json:"file_name"`
	Rows      []map[string]any `json:"rows"` // 模板列名 → 单元格值
}

// importPreview POST /api/admin/fin/import/preview —— 反向解析 + 差异比对，落库 pending。
func (d Deps) importPreview(c *gin.Context) {
	var req previewReq
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Rows) == 0 {
		c.JSON(400, gin.H{"error": "rows 为空"})
		return
	}
	ctx := c.Request.Context()

	mapping, err := d.Store.Mappings().Get(ctx, req.MappingID)
	if err != nil {
		c.JSON(404, gin.H{"error": "模板不存在"})
		return
	}

	items, err := d.diffRows(ctx, mapping, req.Rows)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if len(items.Creates)+len(items.Updates) == 0 {
		c.JSON(400, gin.H{
			"error": "无可入库行", "errors": items.Errors,
		})
		return
	}

	batch := &domain.ImportBatch{
		BatchID: newID("imp"), MappingID: req.MappingID,
		FileName: &req.FileName, Actor: "admin",
	}
	if err := d.Store.ImportBatches().CreatePreview(ctx, batch, items); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"batch_id": batch.BatchID,
		"creates":  items.Creates, "updates": items.Updates, "errors": items.Errors,
		"summary": gin.H{
			"creates": len(items.Creates), "updates": len(items.Updates), "errors": len(items.Errors),
		},
	})
}

// diffRows 反向解析（模板列名 → 系统字段）+ 与库内比对（SN 匹配）→ 差异明细。
func (d Deps) diffRows(ctx context.Context, m *domain.FinMapping, rows []map[string]any) (*repo.ImportItems, error) {
	// 反查表：模板列名 → 字段；状态反向字典：中文 → 系统值
	fieldByCol := map[string]domain.FinColumn{}
	for _, col := range m.Def.Columns {
		fieldByCol[col.Name] = col
	}
	revStatus := map[string]string{}
	for sys, zh := range m.Def.StatusMap {
		revStatus[zh] = sys
		revStatus[sys] = sys // 系统值直填同样接受
	}

	items := &repo.ImportItems{}
	seenSN := map[string]bool{}

	for i, raw := range rows {
		rowNo := i + 2 // 表头占第 1 行，与 Excel 行号对齐
		fields, rowErr := parseRowFields(fieldByCol, revStatus, raw)
		if rowErr != nil {
			items.Errors = append(items.Errors, repo.ImportError{Row: rowNo, Reason: rowErr.Error()})
			continue
		}
		if len(fields) == 0 || strings.TrimSpace(fields["name"]) == "" {
			items.Errors = append(items.Errors, repo.ImportError{Row: rowNo, Reason: "资产名称缺失"})
			continue
		}

		sn := strings.TrimSpace(fields["sn"])
		if sn == "" {
			items.Creates = append(items.Creates, repo.ImportCreate{Row: rowNo, Fields: fields})
			continue
		}
		if seenSN[sn] {
			items.Errors = append(items.Errors, repo.ImportError{Row: rowNo, Reason: "文件内 SN 重复：" + sn})
			continue
		}
		seenSN[sn] = true

		matched, err := d.Store.Assets().GetBySN(ctx, sn)
		if err != nil {
			return nil, err
		}
		switch len(matched) {
		case 0:
			items.Creates = append(items.Creates, repo.ImportCreate{Row: rowNo, Fields: fields})
		case 1:
			if up := diffUpdate(rowNo, &matched[0], fields); up != nil {
				items.Updates = append(items.Updates, *up)
			}
			// 无差异 → 静默跳过（不做无意义回写）
		default:
			items.Errors = append(items.Errors,
				repo.ImportError{Row: rowNo, Reason: fmt.Sprintf("SN 命中 %d 条资产：%s", len(matched), sn)})
		}
	}
	return items, nil
}

// parseRowFields 单行反向解析：模板列 → 系统字段字符串表（带格式校验）。
func parseRowFields(fieldByCol map[string]domain.FinColumn, revStatus map[string]string, raw map[string]any) (map[string]string, error) {
	fields := map[string]string{}
	for colName, v := range raw {
		col, ok := fieldByCol[colName]
		if !ok || col.Readonly {
			continue // 未知列 / 只读列（tag_id）忽略
		}
		s := strings.TrimSpace(cellText(v))
		if s == "" {
			continue
		}
		switch col.Field {
		case "price":
			if _, err := repo.ParsePrice(s); err != nil {
				return nil, err
			}
		case "buy_date", "warranty_end":
			if _, err := repo.ParseDayField(s); err != nil {
				return nil, err
			}
		case "status":
			sys, ok := revStatus[s]
			if !ok {
				return nil, fmt.Errorf("未知状态：%q", s)
			}
			s = sys
		}
		fields[col.Field] = s
	}
	return fields, nil
}

// cellText SheetJS 单元格值 → 文本（数字浮点去尾零）。
func cellText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", x)
	}
}

// diffUpdate 库内资产 vs 导入字段逐项比对 → 变化字段 update（无变化返回 nil）。
func diffUpdate(rowNo int, a *domain.Asset, fields map[string]string) *repo.ImportUpdate {
	current := map[string]string{
		"name": a.Name, "category": deref(a.Category), "model": deref(a.Model),
		"sn": deref(a.Sn), "dept_id": deref(a.DeptID), "keeper": deref(a.Keeper),
		"location": deref(a.Location), "unit": deref(a.Unit), "remark": deref(a.Remark),
		"status": string(a.Status),
	}
	if a.Price != nil {
		current["price"] = fmt.Sprintf("%.2f", *a.Price)
	}
	if a.BuyDate != nil {
		current["buy_date"] = a.BuyDate.Format("2006-01-02")
	}
	if a.WarrantyEnd != nil {
		current["warranty_end"] = a.WarrantyEnd.Format("2006-01-02")
	}

	up := &repo.ImportUpdate{Row: rowNo, AssetID: a.AssetID, TagID: a.TagID,
		New: map[string]string{}, Old: map[string]string{}}
	for f, nv := range fields {
		if cv, ok := current[f]; ok && cv != nv {
			up.New[f], up.Old[f] = nv, cv
		}
	}
	if len(up.New) == 0 {
		return nil
	}
	return up
}

// confirmBatch POST /api/admin/fin/import/batches/:batchID/confirm —— 确认入库。
func (d Deps) confirmBatch(c *gin.Context) {
	created, updated, err := d.Store.ImportBatches().
		ConfirmBatch(c.Request.Context(), c.Param("batchID"), "admin")
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true, "created": created, "updated": updated})
}

// rollbackBatch POST /api/admin/fin/import/batches/:batchID/rollback —— 按批次回滚。
func (d Deps) rollbackBatch(c *gin.Context) {
	if err := d.Store.ImportBatches().
		RollbackBatch(c.Request.Context(), c.Param("batchID"), "admin"); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// listBatches GET /api/admin/fin/import/batches —— 批次列表。
func (d Deps) listBatches(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	items, err := d.Store.ImportBatches().List(c.Request.Context(), limit)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if items == nil {
		items = []domain.ImportBatch{}
	}
	c.JSON(200, gin.H{"items": items})
}
