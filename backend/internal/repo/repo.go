// Package repo 提供领域模型的数据访问层（database/sql，SQLite 驱动）。
package repo

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrNotFound 统一的"查无此物"哨兵错误。
var ErrNotFound = errors.New("not found")

// mapErr 驱动层错误 → 领域哨兵错误。
func mapErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// Store 数据访问总入口（各 Repo 共享同一连接）。
type Store struct {
	DB *sql.DB
}

// NewStore 构造。
func NewStore(db *sql.DB) *Store { return &Store{DB: db} }

// TagRepo 码表数据访问。
func (s *Store) Tags() *TagRepo { return &TagRepo{db: s.DB} }

// AssetRepo 资产数据访问。
func (s *Store) Assets() *AssetRepo { return &AssetRepo{db: s.DB} }

// EventRepo 履历数据访问。
func (s *Store) Events() *EventRepo { return &EventRepo{db: s.DB} }

// ScanLogRepo 扫码流水数据访问。
func (s *Store) ScanLogs() *ScanLogRepo { return &ScanLogRepo{db: s.DB} }

// UserRepo 微信用户数据访问。
func (s *Store) Users() *UserRepo { return &UserRepo{db: s.DB} }

// MappingRepo 财务模板数据访问。
func (s *Store) Mappings() *MappingRepo { return &MappingRepo{db: s.DB} }

// ExportJobRepo 导出审计数据访问。
func (s *Store) ExportJobs() *ExportJobRepo { return &ExportJobRepo{db: s.DB} }

// ImportBatchRepo 导入批次数据访问。
func (s *Store) ImportBatches() *ImportBatchRepo { return &ImportBatchRepo{db: s.DB} }

// Count 返回资产总数。
func (s *Store) Count(ctx context.Context) (int64, error) {
	var n int64
	err := s.DB.QueryRowContext(ctx, `SELECT count(*) FROM assets`).Scan(&n)
	return n, err
}

// timeFromText SQLite TEXT 时间 → time.Time（RFC3339）。
func timeFromText(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// nowText 方言中立的当前时间戳（与 SQLite strftime 默认值同格式：UTC 毫秒 +00:00）。
// 应用层显式传值，SQL 不含任何方言函数——PG 切换点收敛在 internal/db。
func nowText() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000+00:00")
}

// dateFromNull TEXT 日期（YYYY-MM-DD）→ *time.Time。
func dateFromNull(s sql.NullString) (*time.Time, error) {
	if !s.Valid || s.String == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", s.String, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// dateToText *time.Time → TEXT 日期（nil → NULL）。
func dateToText(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format("2006-01-02")
}

// nullToPtr NullString → *string（NULL → nil）。
func nullToPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	v := s.String
	return &v
}
