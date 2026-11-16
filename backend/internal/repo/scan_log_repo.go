package repo

import (
	"context"
	"database/sql"
)

// ScanLogRepo 扫码流水数据访问。
type ScanLogRepo struct {
	db *sql.DB
}

// Insert 记录一次扫码（M1 同步写，量小可接受）。
func (r *ScanLogRepo) Insert(ctx context.Context, tagID, actor, channel, ip, ua string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO scan_logs (tag_id, actor, channel, ip, user_agent, created_at)
		 VALUES (?, NULLIF(?, ''), ?, ?, ?, ?)`,
		tagID, actor, channel, ip, ua, nowText())
	return err
}
