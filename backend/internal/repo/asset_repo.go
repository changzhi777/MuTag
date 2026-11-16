package repo

import (
	"context"
	"database/sql"

	"mutag/backend/internal/domain"
)

// AssetRepo 资产数据访问。
type AssetRepo struct {
	db *sql.DB
}

const assetCols = `asset_id, tag_id, name, category, model, sn, buy_date, price,
	dept_id, keeper, location, status, unit, warranty_end, remark, created_at, updated_at`

// GetByTagID 按绑定码取资产。
func (r *AssetRepo) GetByTagID(ctx context.Context, tagID string) (*domain.Asset, error) {
	a, err := scanAsset(r.db.QueryRowContext(ctx,
		`SELECT `+assetCols+` FROM assets WHERE tag_id = ?`, tagID))
	if err != nil {
		return nil, mapErr(err)
	}
	return a, nil
}

// Create 新建资产，返回 asset_id。
func (r *AssetRepo) Create(ctx context.Context, a *domain.Asset) (int64, error) {
	now := nowText()
	// RETURNING 双方言通用（SQLite 3.35+ / PG）——PG 无 LastInsertId
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO assets (tag_id, name, category, model, sn, buy_date, price,
			dept_id, keeper, location, status, unit, warranty_end, remark, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 RETURNING asset_id`,
		a.TagID, a.Name, a.Category, a.Model, a.Sn, dateToText(a.BuyDate), a.Price,
		a.DeptID, a.Keeper, a.Location, a.Status, a.Unit, dateToText(a.WarrantyEnd), a.Remark,
		now, now).Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

// AssetFilter 资产列表筛选条件（财务导出与台账列表共用）。
type AssetFilter struct {
	Keyword string // 模糊匹配 name/tag_id/keeper/sn
	Status  string
	Dept    string
	BuyFrom string // 入账日期下界 YYYY-MM-DD（TEXT 字典序即时间序）
	BuyTo   string
	Page    int
	Size    int // <=0 时不分页（导出全量）
}

// List 分页筛选列表（tag_id DESC = 新码在前；Size<=0 全量不分页）。
func (r *AssetRepo) List(ctx context.Context, f AssetFilter) ([]domain.Asset, int64, error) {
	where := "WHERE 1=1"
	args := []any{}
	if f.Keyword != "" {
		where += " AND (name LIKE ? OR tag_id LIKE ? OR keeper LIKE ? OR sn LIKE ?)"
		kw := "%" + f.Keyword + "%"
		args = append(args, kw, kw, kw, kw)
	}
	if f.Status != "" {
		where += " AND status = ?"
		args = append(args, f.Status)
	}
	if f.Dept != "" {
		where += " AND dept_id = ?"
		args = append(args, f.Dept)
	}
	if f.BuyFrom != "" {
		where += " AND buy_date >= ?"
		args = append(args, f.BuyFrom)
	}
	if f.BuyTo != "" {
		where += " AND buy_date <= ?"
		args = append(args, f.BuyTo)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT count(*) FROM assets `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT ` + assetCols + ` FROM assets ` + where + ` ORDER BY asset_id DESC`
	if f.Size > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, f.Size, (f.Page-1)*f.Size)
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []domain.Asset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, *a)
	}
	return items, total, rows.Err()
}

// GetBySN 按序列号查资产（财务导入匹配键；SN 非唯一约束，命中多条时由调用方报错）。
func (r *AssetRepo) GetBySN(ctx context.Context, sn string) ([]domain.Asset, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+assetCols+` FROM assets WHERE sn = ? ORDER BY asset_id`, sn)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Asset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *a)
	}
	return items, rows.Err()
}

// Update 全字段更新（updated_at 由 SQL 刷新）。
func (r *AssetRepo) Update(ctx context.Context, a *domain.Asset) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE assets SET name=?, category=?, model=?, sn=?, buy_date=?, price=?,
		 dept_id=?, keeper=?, location=?, status=?, unit=?, warranty_end=?, remark=?,
		 updated_at=?
		 WHERE tag_id=?`,
		a.Name, a.Category, a.Model, a.Sn, dateToText(a.BuyDate), a.Price,
		a.DeptID, a.Keeper, a.Location, a.Status, a.Unit, dateToText(a.WarrantyEnd), a.Remark,
		nowText(), a.TagID)
	return err
}

// SetStatus 更新资产状态（作废联动用）。
func (r *AssetRepo) SetStatus(ctx context.Context, tagID string, status domain.AssetStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE assets SET status=?, updated_at=? WHERE tag_id=?`,
		status, nowText(), tagID)
	return err
}

// assetScanner 行扫描复用（List 多行 / GetByTagID 单行共用）。
type assetScanner interface{ Scan(dest ...any) error }

func scanAsset(row assetScanner) (*domain.Asset, error) {
	a := &domain.Asset{}
	var category, model, sn, keeper, location, unit, remark, deptID sql.NullString
	var price sql.NullFloat64
	var buyDate, warrantyEnd, createdAt, updatedAt sql.NullString
	if err := row.Scan(&a.AssetID, &a.TagID, &a.Name, &category, &model, &sn,
		&buyDate, &price, &deptID, &keeper, &location, &a.Status, &unit,
		&warrantyEnd, &remark, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	a.Category = nullToPtr(category)
	a.Model = nullToPtr(model)
	a.Sn = nullToPtr(sn)
	a.DeptID = nullToPtr(deptID)
	a.Keeper = nullToPtr(keeper)
	a.Location = nullToPtr(location)
	a.Unit = nullToPtr(unit)
	a.Remark = nullToPtr(remark)
	if price.Valid {
		a.Price = &price.Float64
	}
	var err error
	if a.BuyDate, err = dateFromNull(buyDate); err != nil {
		return nil, err
	}
	if a.WarrantyEnd, err = dateFromNull(warrantyEnd); err != nil {
		return nil, err
	}
	if a.CreatedAt, err = timeFromText(createdAt.String); err != nil {
		return nil, err
	}
	if a.UpdatedAt, err = timeFromText(updatedAt.String); err != nil {
		return nil, err
	}
	return a, nil
}
