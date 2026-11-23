package repo

import (
	"context"
	"database/sql"
	"errors"

	"mutag/backend/internal/domain"
)

// ErrAdminNotFound 账号不存在。
var ErrAdminNotFound = errors.New("admin not found")

// AdminRepo 后台管理员数据访问（0007 多管理员；与微信 users 体系无关）。
type AdminRepo struct {
	db *sql.DB
}

// GetByUsername 按账号取管理员。
func (r *AdminRepo) GetByUsername(ctx context.Context, username string) (*domain.Admin, error) {
	a := &domain.Admin{}
	var displayName sql.NullString
	var createdAt sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, username, password_hash, display_name, created_at FROM admins WHERE username=?`,
		username).Scan(&a.ID, &a.Username, &a.PasswordHash, &displayName, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAdminNotFound
		}
		return nil, err
	}
	a.DisplayName = displayName.String
	if a.CreatedAt, err = timeFromText(createdAt.String); err != nil {
		return nil, err
	}
	return a, nil
}

// Create 新增管理员（password_hash 由调用方生成）。
func (r *AdminRepo) Create(ctx context.Context, a *domain.Admin) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO admins (username, password_hash, display_name, created_at)
		 VALUES (?, ?, ?, ?) RETURNING id`,
		a.Username, a.PasswordHash, displayNamePtr(a.DisplayName), nowText()).Scan(&id)
	return id, err
}

// displayNamePtr string → any（空串 → NULL）。
func displayNamePtr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Count 管理员总数（播种/单密码兼容判定用）。
func (r *AdminRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM admins`).Scan(&n)
	return n, err
}

// nullStr *string → any（nil → NULL）。
func nullStr(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}
