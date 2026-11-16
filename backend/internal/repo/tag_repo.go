package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"mutag/backend/internal/domain"
)

// TagRepo 码表数据访问。
type TagRepo struct {
	db *sql.DB
}

// GetByURLCode 按活码路径段取码。
func (r *TagRepo) GetByURLCode(ctx context.Context, urlCode string) (*domain.Tag, error) {
	return r.scanRow(ctx, "SELECT tag_id, qr_scene, url_code, status, created_at FROM tags WHERE url_code = ?", urlCode)
}

// GetByTagID 按码号取码。
func (r *TagRepo) GetByTagID(ctx context.Context, tagID string) (*domain.Tag, error) {
	return r.scanRow(ctx, "SELECT tag_id, qr_scene, url_code, status, created_at FROM tags WHERE tag_id = ?", tagID)
}

func (r *TagRepo) scanRow(ctx context.Context, query, key string) (*domain.Tag, error) {
	t := &domain.Tag{}
	var createdAt string
	err := r.db.QueryRowContext(ctx, query, key).
		Scan(&t.TagID, &t.QrScene, &t.UrlCode, &t.Status, &createdAt)
	if err != nil {
		return nil, mapErr(err)
	}
	if t.CreatedAt, err = timeFromText(createdAt); err != nil {
		return nil, err
	}
	return t, nil
}

// Insert 新建码（qr_scene 可空）。
func (r *TagRepo) Insert(ctx context.Context, t *domain.Tag) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO tags (tag_id, qr_scene, url_code, status, created_at) VALUES (?, NULLIF(?, ''), ?, ?, ?)`,
		t.TagID, t.QrScene, t.UrlCode, t.Status, nowText())
	return err
}

// NextTagID 分配下一个码号：MAX(tag_id) 序号 +1（tagId 永不 recycle，只前进）。
func (r *TagRepo) NextTagID(ctx context.Context) (string, error) {
	return nextTagID(ctx, r.db)
}

// dbtx 单连接事务/库通用接口（供事务内复用码号分配等助手）。
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// nextTagID 在指定连接（库或事务）上分配下一码号。
func nextTagID(ctx context.Context, q dbtx) (string, error) {
	var maxID sql.NullString
	if err := q.QueryRowContext(ctx,
		`SELECT MAX(tag_id) FROM tags`).Scan(&maxID); err != nil {
		return "", err
	}
	seq := 0
	if maxID.Valid {
		// 形如 MT-2026-000074，取末段序号
		parts := strings.Split(maxID.String, "-")
		if n, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
			seq = n
		}
	}
	return fmt.Sprintf("MT-2026-%06d", seq+1), nil
}

// List 码分页列表（status 可选筛选）。
func (r *TagRepo) List(ctx context.Context, status string, page, size int) ([]domain.Tag, int64, error) {
	where := "WHERE 1=1"
	args := []any{}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx,
		`SELECT count(*) FROM tags `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, size, (page-1)*size)
	rows, err := r.db.QueryContext(ctx,
		`SELECT tag_id, qr_scene, url_code, status, created_at FROM tags `+where+
			` ORDER BY tag_id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tags []domain.Tag
	for rows.Next() {
		t := domain.Tag{}
		var createdAt string
		if err := rows.Scan(&t.TagID, &t.QrScene, &t.UrlCode, &t.Status, &createdAt); err != nil {
			return nil, 0, err
		}
		if t.CreatedAt, err = timeFromText(createdAt); err != nil {
			return nil, 0, err
		}
		tags = append(tags, t)
	}
	return tags, total, rows.Err()
}

// SetStatus 更新码状态（active ↔ void）。
func (r *TagRepo) SetStatus(ctx context.Context, tagID string, status domain.TagStatus) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tags SET status = ? WHERE tag_id = ?`, status, tagID)
	return err
}
