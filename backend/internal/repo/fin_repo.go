// fin_repo.go 财务域数据访问：模板（内置默认播种）、导出审计、导入批次（preview→confirm→rollback）。
// 财务合规纪律：导出留操作人与过滤条件；导入两段式（先预览落库、确认才入库），回滚按明细反向操作。
package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"mutag/backend/internal/domain"
)

// ---------------------------------------------------------------------------
// 导入批次明细契约（api 层 preview 构造、本层 confirm/rollback 执行共用）。
// 字段值统一字符串表示（price "1234.56"、buy_date "2006-01-02"），入库时转类型。
// ---------------------------------------------------------------------------

// ImportCreate 预览判定为新增的行（confirm 后回填 tag/asset_id）。
type ImportCreate struct {
	Row     int               `json:"row"`
	TagID   string            `json:"tag_id,omitempty"`
	AssetID int64             `json:"asset_id,omitempty"`
	Fields  map[string]string `json:"fields"` // 系统字段 → 值（白名单内）
}

// ImportUpdate 预览判定为更新的行：只存变化字段（new/old 同 key 对齐）。
type ImportUpdate struct {
	Row     int               `json:"row"`
	AssetID int64             `json:"asset_id"`
	TagID   string            `json:"tag_id"`
	New     map[string]string `json:"new"`
	Old     map[string]string `json:"old"`
}

// ImportError 预览判定的错误行（不入库，带原因）。
type ImportError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

// ImportItems 批次全量明细（import_batches.items JSON）。
type ImportItems struct {
	Creates []ImportCreate `json:"creates"`
	Updates []ImportUpdate `json:"updates"`
	Errors  []ImportError  `json:"errors"`
}

// importableFields 导入可写字段白名单（tag_id 由系统分配，永不导入——纪律 1）。
var importableFields = map[string]bool{
	"name": true, "category": true, "model": true, "sn": true,
	"buy_date": true, "price": true, "dept_id": true, "keeper": true,
	"location": true, "status": true, "unit": true, "warranty_end": true, "remark": true,
}

// ---------------------------------------------------------------------------
// 内置模板播种
// ---------------------------------------------------------------------------

// statusMapDefault 系统状态 → 财务模板中文（导入反向查）。
func statusMapDefault() map[string]string {
	return map[string]string{
		"in_use": "在用", "idle": "闲置", "repair": "维修中", "borrowed": "借出", "scrapped": "报废",
	}
}

func col(name, field string) domain.FinColumn { return domain.FinColumn{Name: name, Field: field} }

func colRO(name, field string) domain.FinColumn {
	return domain.FinColumn{Name: name, Field: field, Readonly: true}
}

// defaultFinDefs 内置四模板（用友 U8 固定资产卡片 / 金蝶云星空资产清单 / 管家婆 / 通用台账）。
func defaultFinDefs() []domain.FinMapping {
	common := statusMapDefault()
	mk := func(kind, name string, def domain.FinMappingDef) domain.FinMapping {
		def.StatusMap = common
		if def.DateFormat == "" {
			def.DateFormat = "YYYY-MM-DD"
		}
		return domain.FinMapping{MappingID: kind, Name: name, Kind: kind, Def: def, Version: 1}
	}
	return []domain.FinMapping{
		mk("yonyou_u8", "用友 U8 固定资产卡片", domain.FinMappingDef{
			Columns: []domain.FinColumn{
				colRO("资产编号", "tag_id"), col("资产名称", "name"), col("类别", "category"),
				col("规格型号", "model"), col("部门编码", "dept_id"), col("启用日期", "buy_date"),
				col("原值", "price"), col("保管人", "keeper"), col("存放地点", "location"),
				col("使用状态", "status"),
			},
			FixedRows: [][]string{{"单位名称：", "", "账套日期：", ""}},
		}),
		mk("kingdee", "金蝶云星空资产清单", domain.FinMappingDef{
			Columns: []domain.FinColumn{
				colRO("资产编码", "tag_id"), col("资产名称", "name"), col("资产类别", "category"),
				col("规格型号", "model"), col("SN码", "sn"), col("计量单位", "unit"),
				col("入账日期", "buy_date"), col("原币金额", "price"), col("部门", "dept_id"),
				col("保管人", "keeper"), col("存放地点", "location"), col("资产状态", "status"),
			},
		}),
		mk("guanjiapo", "管家婆资产台账", domain.FinMappingDef{
			Columns: []domain.FinColumn{
				colRO("编号", "tag_id"), col("名称", "name"), col("分类", "category"),
				col("单位", "unit"), col("购置日期", "buy_date"), col("金额", "price"),
				col("经手人", "keeper"), col("存放位置", "location"), col("状态", "status"),
			},
		}),
		mk("generic", "通用资产台账", domain.FinMappingDef{
			Columns: []domain.FinColumn{
				colRO("资产编号", "tag_id"), col("名称", "name"), col("类别", "category"),
				col("规格型号", "model"), col("序列号", "sn"), col("单位", "unit"),
				col("购置日期", "buy_date"), col("原值", "price"), col("部门编码", "dept_id"),
				col("保管人", "keeper"), col("存放地点", "location"), col("保修到期", "warranty_end"),
				col("状态", "status"), col("备注", "remark"),
			},
		}),
	}
}

// EnsureFinDefaults 幂等播种内置模板（kind 已存在则跳过，保留管理员改动）。
func EnsureFinDefaults(ctx context.Context, db *sql.DB) error {
	for _, m := range defaultFinDefs() {
		var exists bool
		if err := db.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM fin_mappings WHERE kind = ?)`, m.Kind).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := insertMapping(ctx, db, &m); err != nil {
			return fmt.Errorf("seed fin mapping %s: %w", m.Kind, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 模板
// ---------------------------------------------------------------------------

// MappingRepo 财务模板数据访问。
type MappingRepo struct{ db *sql.DB }

func (r *MappingRepo) scanOne(row *sql.Row) (*domain.FinMapping, error) {
	var m domain.FinMapping
	var defText, updatedAt string
	if err := row.Scan(&m.MappingID, &m.Name, &m.Kind, &defText, &m.Version, &updatedAt); err != nil {
		return nil, mapErr(err)
	}
	if err := json.Unmarshal([]byte(defText), &m.Def); err != nil {
		return nil, fmt.Errorf("mapping def json: %w", err)
	}
	t, err := timeFromText(updatedAt)
	if err != nil {
		return nil, err
	}
	m.UpdatedAt = t
	return &m, nil
}

// Get 按模板 ID 取。
func (r *MappingRepo) Get(ctx context.Context, mappingID string) (*domain.FinMapping, error) {
	return r.scanOne(r.db.QueryRowContext(ctx,
		`SELECT mapping_id, name, kind, mapping, version, updated_at FROM fin_mappings WHERE mapping_id = ?`,
		mappingID))
}

// List 全部模板（kind 升序）。
func (r *MappingRepo) List(ctx context.Context) ([]domain.FinMapping, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT mapping_id, name, kind, mapping, version, updated_at FROM fin_mappings ORDER BY kind`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.FinMapping
	for rows.Next() {
		var m domain.FinMapping
		var defText, updatedAt string
		if err := rows.Scan(&m.MappingID, &m.Name, &m.Kind, &defText, &m.Version, &updatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(defText), &m.Def); err != nil {
			return nil, fmt.Errorf("mapping def json: %w", err)
		}
		if m.UpdatedAt, err = timeFromText(updatedAt); err != nil {
			return nil, err
		}
		items = append(items, m)
	}
	return items, rows.Err()
}

// Update 编辑模板映射（version 递增，可追溯）。
func (r *MappingRepo) Update(ctx context.Context, mappingID string, def *domain.FinMappingDef) error {
	defText, err := json.Marshal(def)
	if err != nil {
		return err
	}
	res, err := r.db.ExecContext(ctx,
		`UPDATE fin_mappings SET mapping=?, version=version+1,
		 updated_at=? WHERE mapping_id=?`,
		string(defText), nowText(), mappingID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func insertMapping(ctx context.Context, q dbtx, m *domain.FinMapping) error {
	defText, err := json.Marshal(m.Def)
	if err != nil {
		return err
	}
	now := nowText()
	_, err = q.ExecContext(ctx,
		`INSERT INTO fin_mappings (mapping_id, name, kind, mapping, version, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.MappingID, m.Name, m.Kind, string(defText), m.Version, now, now)
	return err
}

// ---------------------------------------------------------------------------
// 导出审计
// ---------------------------------------------------------------------------

// ExportJobRepo 导出任务审计数据访问。
type ExportJobRepo struct{ db *sql.DB }

// Create 记录一次导出行为（财务合规留痕）。
func (r *ExportJobRepo) Create(ctx context.Context, job *domain.ExportJob) error {
	filters, err := json.Marshal(job.Filters)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO export_jobs (job_id, mapping_id, filters, actor, row_count, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		job.JobID, job.MappingID, string(filters), job.Actor, job.RowCount, nowText())
	return err
}

// List 最近导出记录（新→旧）。
func (r *ExportJobRepo) List(ctx context.Context, limit int) ([]domain.ExportJob, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT job_id, mapping_id, filters, actor, row_count, created_at
		 FROM export_jobs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.ExportJob
	for rows.Next() {
		var j domain.ExportJob
		var filters, createdAt string
		if err := rows.Scan(&j.JobID, &j.MappingID, &filters, &j.Actor, &j.RowCount, &createdAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(filters), &j.Filters)
		if j.CreatedAt, err = timeFromText(createdAt); err != nil {
			return nil, err
		}
		items = append(items, j)
	}
	return items, rows.Err()
}

// ---------------------------------------------------------------------------
// 导入批次
// ---------------------------------------------------------------------------

// ImportBatchRepo 导入批次数据访问。
type ImportBatchRepo struct{ db *sql.DB }

// CreatePreview 预览落库（status=pending，等待人工确认）。
func (r *ImportBatchRepo) CreatePreview(ctx context.Context, b *domain.ImportBatch, items *ImportItems) error {
	b.Status = domain.BatchPending
	summary, _ := json.Marshal(map[string]int{
		"creates": len(items.Creates), "updates": len(items.Updates), "errors": len(items.Errors),
	})
	itemsText, err := json.Marshal(items)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO import_batches (batch_id, mapping_id, file_name, summary, items, status, actor, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b.BatchID, b.MappingID, b.FileName, string(summary), string(itemsText), string(b.Status), b.Actor, nowText())
	return err
}

// List 最近批次（新→旧）。
func (r *ImportBatchRepo) List(ctx context.Context, limit int) ([]domain.ImportBatch, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, batchCols+` FROM import_batches ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.ImportBatch
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *b)
	}
	return items, rows.Err()
}

const batchCols = `SELECT batch_id, mapping_id, file_name, summary, items, status, actor,
	created_at, confirmed_at, rolled_back_at`

func scanBatch(row assetScanner) (*domain.ImportBatch, error) {
	b := &domain.ImportBatch{}
	var summary, items, createdAt string
	var fileName, confirmedAt, rolledBackAt sql.NullString
	if err := row.Scan(&b.BatchID, &b.MappingID, &fileName, &summary, &items, &b.Status,
		&b.Actor, &createdAt, &confirmedAt, &rolledBackAt); err != nil {
		return nil, mapErr(err)
	}
	if fileName.Valid {
		b.FileName = &fileName.String
	}
	_ = json.Unmarshal([]byte(summary), &b.Summary)
	_ = json.Unmarshal([]byte(items), &b.Items)
	var err error
	if b.CreatedAt, err = timeFromText(createdAt); err != nil {
		return nil, err
	}
	if b.ConfirmedAt, err = timeFromNullText(confirmedAt); err != nil {
		return nil, err
	}
	if b.RolledBackAt, err = timeFromNullText(rolledBackAt); err != nil {
		return nil, err
	}
	return b, nil
}

// timeFromNullText 可空 TEXT 时间 → *time.Time。
func timeFromNullText(s sql.NullString) (*time.Time, error) {
	if !s.Valid || s.String == "" {
		return nil, nil
	}
	t, err := timeFromText(s.String)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ConfirmBatch 事务确认入库：新增分配码号建资产、更新按变化字段回写，全部写履历；
// 明细回填实际 tag/asset_id 后落回 import_batches.items（回滚依据）。
func (r *ImportBatchRepo) ConfirmBatch(ctx context.Context, batchID, actor string) (created, updated int, err error) {
	b, err := r.Get(ctx, batchID)
	if err != nil {
		return 0, 0, err
	}
	if b.Status != domain.BatchPending {
		return 0, 0, fmt.Errorf("批次状态 %s 不可确认（仅 pending 可确认）", b.Status)
	}
	items, err := itemsOf(b.Items)
	if err != nil {
		return 0, 0, err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback() //nolint:errcheck // 出错路径统一回滚

	for i := range items.Creates {
		cr := &items.Creates[i]
		tagID, err := nextTagID(ctx, tx)
		if err != nil {
			return 0, 0, err
		}
		if err := insertTagTx(ctx, tx, tagID); err != nil {
			return 0, 0, err
		}
		assetID, err := insertAssetFromFields(ctx, tx, tagID, cr.Fields)
		if err != nil {
			return 0, 0, fmt.Errorf("行 %d: %w", cr.Row, err)
		}
		note := "财务导入（批次 " + batchID + " 来源行 " + strconv.Itoa(cr.Row) + "）"
		if err := insertEventTx(ctx, tx, assetID, domain.EventCreate, actor,
			nil, cr.Fields, note); err != nil {
			return 0, 0, err
		}
		cr.TagID, cr.AssetID = tagID, assetID
		created++
	}

	for i := range items.Updates {
		up := &items.Updates[i]
		if err := updateAssetFields(ctx, tx, up.AssetID, up.New); err != nil {
			return 0, 0, fmt.Errorf("行 %d: %w", up.Row, err)
		}
		note := "财务导入（批次 " + batchID + " 来源行 " + strconv.Itoa(up.Row) + "）"
		if err := insertEventTx(ctx, tx, up.AssetID, domain.EventUpdate, actor,
			up.Old, up.New, note); err != nil {
			return 0, 0, err
		}
		updated++
	}

	// 回填明细（回滚依据）+ 状态推进
	itemsText, err := json.Marshal(items)
	if err != nil {
		return 0, 0, err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE import_batches SET items=?, status='confirmed',
		 confirmed_at=? WHERE batch_id=?`,
		string(itemsText), nowText(), batchID); err != nil {
		return 0, 0, err
	}
	return created, updated, tx.Commit()
}

// RollbackBatch 事务回滚已确认批次：
// 新增行 → 删履历与资产、码置 void（码号已消耗永不复用——纪律 1）；
// 更新行 → 按 old 快照回写字段并写回滚履历。
func (r *ImportBatchRepo) RollbackBatch(ctx context.Context, batchID, actor string) error {
	b, err := r.Get(ctx, batchID)
	if err != nil {
		return err
	}
	if b.Status != domain.BatchConfirmed {
		return fmt.Errorf("批次状态 %s 不可回滚（仅 confirmed 可回滚）", b.Status)
	}
	items, err := itemsOf(b.Items)
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // 出错路径统一回滚

	for _, cr := range items.Creates {
		if cr.AssetID == 0 || cr.TagID == "" {
			return fmt.Errorf("行 %d 明细缺回填 ID，无法回滚", cr.Row)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM asset_events WHERE asset_id=?`, cr.AssetID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM assets WHERE asset_id=?`, cr.AssetID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE tags SET status='void' WHERE tag_id=?`, cr.TagID); err != nil {
			return err
		}
	}
	for _, up := range items.Updates {
		if err := updateAssetFields(ctx, tx, up.AssetID, up.Old); err != nil {
			return fmt.Errorf("行 %d: %w", up.Row, err)
		}
		note := "导入批次回滚（批次 " + batchID + "）"
		if err := insertEventTx(ctx, tx, up.AssetID, domain.EventUpdate, actor,
			up.New, up.Old, note); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE import_batches SET status='rolled_back',
		 rolled_back_at=? WHERE batch_id=?`, nowText(), batchID); err != nil {
		return err
	}
	return tx.Commit()
}

// Get 单批次。
func (r *ImportBatchRepo) Get(ctx context.Context, batchID string) (*domain.ImportBatch, error) {
	return scanBatch(r.db.QueryRowContext(ctx, batchCols+` FROM import_batches WHERE batch_id=?`, batchID))
}

// itemsOf 泛化 JSON → 类型化明细。
func itemsOf(raw map[string]any) (*ImportItems, error) {
	text, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var items ImportItems
	if err := json.Unmarshal(text, &items); err != nil {
		return nil, fmt.Errorf("批次明细解析: %w", err)
	}
	return &items, nil
}

// ---------------------------------------------------------------------------
// 事务内写入助手（confirm/rollback 共用）
// ---------------------------------------------------------------------------

func insertTagTx(ctx context.Context, tx *sql.Tx, tagID string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO tags (tag_id, qr_scene, url_code, status, created_at) VALUES (?, ?, ?, 'active', ?)`,
		tagID, "t:"+tagID, tagID, nowText())
	return err
}

// insertAssetFromFields 按白名单字段字符串建资产，返回 asset_id。
func insertAssetFromFields(ctx context.Context, tx *sql.Tx, tagID string, fields map[string]string) (int64, error) {
	name := strings.TrimSpace(fields["name"])
	if name == "" {
		return 0, fmt.Errorf("name 必填")
	}
	price, err := parsePrice(fields["price"])
	if err != nil {
		return 0, err
	}
	buyDate, err := parseDayField(fields["buy_date"])
	if err != nil {
		return 0, err
	}
	warrantyEnd, err := parseDayField(fields["warranty_end"])
	if err != nil {
		return 0, err
	}
	status := fields["status"]
	if status == "" {
		status = string(domain.AssetInUse)
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO assets
		(tag_id, name, category, model, sn, buy_date, price, dept_id, keeper, location, status, unit, warranty_end, remark, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		tagID, name, nullIfEmpty(fields["category"]), nullIfEmpty(fields["model"]), nullIfEmpty(fields["sn"]),
		dateArg(buyDate), price, nullIfEmpty(fields["dept_id"]), nullIfEmpty(fields["keeper"]),
		nullIfEmpty(fields["location"]), domain.AssetStatus(status), nullIfEmpty(fields["unit"]),
		dateArg(warrantyEnd), nullIfEmpty(fields["remark"]), nowText(), nowText())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// updateAssetFields 按白名单动态回写变化字段（confirm 写 new / rollback 写 old）。
func updateAssetFields(ctx context.Context, tx *sql.Tx, assetID int64, fields map[string]string) error {
	if len(fields) == 0 {
		return nil
	}
	sets, args := []string{}, []any{}
	for _, f := range sortedKeys(fields) {
		if !importableFields[f] {
			continue // 白名单外字段防御性忽略（preview 已拦，此处兜底）
		}
		switch f {
		case "price":
			v, err := parsePrice(fields[f])
			if err != nil {
				return err
			}
			sets, args = append(sets, "price=?"), append(args, v)
		case "buy_date", "warranty_end":
			t, err := parseDayField(fields[f])
			if err != nil {
				return err
			}
			sets, args = append(sets, f+"=?"), append(args, dateArg(t))
		case "status":
			sets, args = append(sets, "status=?"), append(args, domain.AssetStatus(fields[f]))
		default:
			sets, args = append(sets, f+"=?"), append(args, nullIfEmpty(fields[f]))
		}
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, nowText(), assetID) // 占位符顺序：字段... → updated_at → WHERE asset_id
	res, err := tx.ExecContext(ctx,
		`UPDATE assets SET `+strings.Join(sets, ", ")+`,
		 updated_at=? WHERE asset_id=?`, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("资产 %d 不存在，更新 0 行", assetID)
	}
	return nil
}

// insertEventTx 事务内写履历快照（old/new 为字段字符串表）。
func insertEventTx(ctx context.Context, tx *sql.Tx, assetID int64, typ domain.EventType,
	actor string, oldV, newV map[string]string, note string) error {
	toAny := func(m map[string]string) map[string]any {
		out := make(map[string]any, len(m))
		for k, v := range m {
			out[k] = v
		}
		return out
	}
	oldJSON, newJSON := "null", "null"
	if len(oldV) > 0 {
		if b, err := json.Marshal(toAny(oldV)); err == nil {
			oldJSON = string(b)
		}
	}
	if len(newV) > 0 {
		if b, err := json.Marshal(toAny(newV)); err == nil {
			newJSON = string(b)
		}
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO asset_events (asset_id, type, actor, old_value, new_value, note, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		assetID, typ, actor, oldJSON, newJSON, note, nowText())
	return err
}

// ---------------------------------------------------------------------------
// 字段格式助手（映射层格式转换收敛点——纪律 7）
// ---------------------------------------------------------------------------

// ImportableFields 导入可写字段白名单（api 校验用）。
func ImportableFields() map[string]bool { return importableFields }

// ParsePrice 金额字符串 → float64（空串合法 → nil 语义；非法格式报错）。
func ParsePrice(s string) (any, error) { return parsePrice(s) }

// ParseDayField 日期字符串（YYYY-MM-DD）→ time（空串合法；非法格式报错）。
func ParseDayField(s string) (*time.Time, error) { return parseDayField(s) }

// parsePrice 金额字符串 → float64（空 → nil 语义，非法报错）。
func parsePrice(s string) (any, error) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("金额非数字：%q", s)
	}
	return v, nil
}

// parseDayField 日期字符串（YYYY-MM-DD）→ time（空 → nil 语义，非法报错）。
func parseDayField(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return nil, fmt.Errorf("日期格式应为 YYYY-MM-DD：%q", s)
	}
	return &t, nil
}

// dateArg *time.Time → TEXT 参数（nil → NULL）。
func dateArg(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format("2006-01-02")
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// sortedKeys map 键排序（动态 UPDATE 稳定拼接）。
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
