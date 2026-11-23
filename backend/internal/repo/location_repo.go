package repo

import (
	"context"
	"database/sql"

	"mutag/backend/internal/domain"
)

// LocationRepo 两级位置树数据访问（parent_id NULL=一级区域，非空=二级点位）。
type LocationRepo struct {
	db *sql.DB
}

// Create 新建位置节点，返回 id。
func (r *LocationRepo) Create(ctx context.Context, l *domain.Location) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO locations (name, parent_id, lat, lng, created_at)
		 VALUES (?, ?, ?, ?, ?) RETURNING id`,
		l.Name, l.ParentID, l.Lat, l.Lng, nowText()).Scan(&id)
	return id, err
}

// Update 更新位置（名称/父级/坐标）。
func (r *LocationRepo) Update(ctx context.Context, l *domain.Location) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE locations SET name=?, parent_id=?, lat=?, lng=? WHERE id=?`,
		l.Name, l.ParentID, l.Lat, l.Lng, l.ID)
	return err
}

// Delete 删除位置（有资产引用或子节点时由调用方先校验）。
func (r *LocationRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM locations WHERE id=?`, id)
	return err
}

// Get 取单个位置。
func (r *LocationRepo) Get(ctx context.Context, id int64) (*domain.Location, error) {
	l, err := scanLocation(r.db.QueryRowContext(ctx,
		`SELECT id, name, parent_id, lat, lng, created_at FROM locations WHERE id=?`, id))
	if err != nil {
		return nil, mapErr(err)
	}
	return l, nil
}

// ListAll 全量位置（体量小一次拉取，客户端自行组两级树）。
func (r *LocationRepo) ListAll(ctx context.Context) ([]domain.Location, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, parent_id, lat, lng, created_at FROM locations ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domain.Location
	for rows.Next() {
		l, err := scanLocation(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *l)
	}
	return items, rows.Err()
}

// CountAssets 引用某位置的资产数（删除前校验用）。
func (r *LocationRepo) CountAssets(ctx context.Context, id int64) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx,
		`SELECT count(*) FROM assets WHERE location_id=?`, id).Scan(&n)
	return n, err
}

// CountChildren 子节点数（删除前校验用）。
func (r *LocationRepo) CountChildren(ctx context.Context, id int64) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx,
		`SELECT count(*) FROM locations WHERE parent_id=?`, id).Scan(&n)
	return n, err
}

func scanLocation(row interface{ Scan(dest ...any) error }) (*domain.Location, error) {
	l := &domain.Location{}
	var parentID sql.NullInt64
	var lat, lng sql.NullFloat64
	var createdAt sql.NullString
	if err := row.Scan(&l.ID, &l.Name, &parentID, &lat, &lng, &createdAt); err != nil {
		return nil, err
	}
	if parentID.Valid {
		v := parentID.Int64
		l.ParentID = &v
	}
	if lat.Valid {
		v := lat.Float64
		l.Lat = &v
	}
	if lng.Valid {
		v := lng.Float64
		l.Lng = &v
	}
	var err error
	if l.CreatedAt, err = timeFromText(createdAt.String); err != nil {
		return nil, err
	}
	return l, nil
}
