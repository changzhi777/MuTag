// Package repo 存量清洗：审计 job + 快照/回滚（规划 P2 任务 3.1）。
// 清理口径（纪律 1 推论）：assets/asset_events/scan_logs 物理删，tags 置 void 保留号簿
// （nextTagID = MAX(tag_id)+1，物理删 tags 必致码号回池）。
// 快照策略：执行前 CREATE TABLE _bak_{suffix}_{table} AS SELECT（双方言同语法，执行期动态建表）。
package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// CleanupJob 清理任务审计行。
type CleanupJob struct {
	JobID      string
	Actor      string
	Mode       string // all | align
	KeepStrategy string
	Summary    string // JSON
	Planned    string // JSON：plan 草案
	RemovedCount int64
	VoidedCount  int64
	SnapshotSuffix string
	Status     string // planned | executed | restored | failed
	CreatedAt  string
	ExecutedAt string
	RestoredAt string
}

// CleanupRepo 清理数据访问。
type CleanupRepo struct{ db *sql.DB }

// Cleanup 构造。
func (s *Store) Cleanup() *CleanupRepo { return &CleanupRepo{db: s.DB} }

const cleanupJobCols = `job_id, actor, mode, keep_strategy, summary, planned,
	removed_count, voided_count, snapshot_suffix, status, created_at, executed_at, restored_at`

func scanCleanupJob(row interface{ Scan(...any) error }) (*CleanupJob, error) {
	j := &CleanupJob{}
	if err := row.Scan(&j.JobID, &j.Actor, &j.Mode, &j.KeepStrategy, &j.Summary, &j.Planned,
		&j.RemovedCount, &j.VoidedCount, &j.SnapshotSuffix, &j.Status,
		&j.CreatedAt, &j.ExecutedAt, &j.RestoredAt); err != nil {
		return nil, err
	}
	return j, nil
}

// CreateJob 落 planned 草案。
func (r *CleanupRepo) CreateJob(ctx context.Context, j *CleanupJob) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO cleanup_jobs (`+cleanupJobCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.JobID, j.Actor, j.Mode, j.KeepStrategy, j.Summary, j.Planned,
		j.RemovedCount, j.VoidedCount, j.SnapshotSuffix, j.Status,
		j.CreatedAt, j.ExecutedAt, j.RestoredAt)
	return err
}

// Get 读单条。
func (r *CleanupRepo) Get(ctx context.Context, jobID string) (*CleanupJob, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT `+cleanupJobCols+` FROM cleanup_jobs WHERE job_id = ?`, jobID)
	j, err := scanCleanupJob(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job 不存在: %s", jobID)
	}
	return j, err
}

// List 历史列表（审计，新在前）。
func (r *CleanupRepo) List(ctx context.Context) ([]CleanupJob, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+cleanupJobCols+` FROM cleanup_jobs ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CleanupJob{}
	for rows.Next() {
		j, err := scanCleanupJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *j)
	}
	return out, rows.Err()
}

// MarkExecuted 执行完成落账。
func (r *CleanupRepo) MarkExecuted(ctx context.Context, jobID, suffix string, removed, voided int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE cleanup_jobs SET status='executed', snapshot_suffix=?, removed_count=?, voided_count=?,
		 executed_at=? WHERE job_id=?`,
		suffix, removed, voided, time.Now().Format("2006-01-02T15:04:05.000+00:00"), jobID)
	return err
}

// MarkRestored 回滚完成落账。
func (r *CleanupRepo) MarkRestored(ctx context.Context, jobID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE cleanup_jobs SET status='restored', restored_at=? WHERE job_id=?`,
	time.Now().Format("2006-01-02T15:04:05.000+00:00"), jobID)
	return err
}

// SnapshotAndExecute 快照 + 原子清理（单事务：TEMP del 表 + 4 表快照 + 3 表删 + tags 置 void）。
// tagIDs 为本轮要清理的资产码；返回实际删除/作废计数。
// 快照表名 _bak_{suffix}_{table}：suffix 由调用方生成（jobID 前 8 位+时间戳），保证幂等可回滚。
func (r *CleanupRepo) SnapshotAndExecute(ctx context.Context, suffix string, tagIDs []string) (removed, voided int64, err error) {
	if len(tagIDs) == 0 {
		return 0, 0, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// TEMP del 表：双方言同语法，事务结束自动消失（快照表为永久表不受影响）
	if _, err = tx.ExecContext(ctx, `CREATE TEMP TABLE del_tags (tag_id TEXT PRIMARY KEY)`); err != nil {
		return 0, 0, fmt.Errorf("create temp del: %w", err)
	}
	// 逐行插入（modernc 对多 VALUES 组占位符展开有兼容坑；百级数量事务内代价可忽略）
	for _, t := range tagIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO del_tags (tag_id) VALUES (?)`, t); err != nil {
			return 0, 0, fmt.Errorf("fill del: %w", err)
		}
	}

	// 快照 4 表（IF NOT EXISTS：同 suffix 重跑幂等；asset_events 走 assets 中转——无 tag_id 列）
	snapQueries := []struct{ tbl, join string }{
		{"assets", "JOIN del_tags d ON t.tag_id = d.tag_id"},
		{"tags", "JOIN del_tags d ON t.tag_id = d.tag_id"},
		{"scan_logs", "JOIN del_tags d ON t.tag_id = d.tag_id"},
		{"asset_events", "JOIN assets a ON t.asset_id = a.asset_id JOIN del_tags d ON a.tag_id = d.tag_id"},
	}
	for _, sq := range snapQueries {
		q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS _bak_%s_%s AS SELECT t.* FROM %s t %s`,
			suffix, sq.tbl, sq.tbl, sq.join)
		if _, err = tx.ExecContext(ctx, q); err != nil {
			return 0, 0, fmt.Errorf("snapshot %s: %w", sq.tbl, err)
		}
	}

	// 清理：3 表物理删 + tags 置 void（号簿保留，纪律 1）
	for _, q := range []string{
		`DELETE FROM asset_events WHERE asset_id IN (SELECT a.asset_id FROM assets a JOIN del_tags d ON a.tag_id = d.tag_id)`,
		`DELETE FROM scan_logs WHERE tag_id IN (SELECT tag_id FROM del_tags)`,
		`DELETE FROM assets WHERE tag_id IN (SELECT tag_id FROM del_tags)`,
		`UPDATE tags SET status='void' WHERE tag_id IN (SELECT tag_id FROM del_tags)`,
	} {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			return 0, 0, fmt.Errorf("execute cleanup: %w", err)
		}
	}

	// 计数（快照行数 = 删除数；voided = 快照中 active 的行数）
	if err = tx.QueryRowContext(ctx,
		`SELECT count(*), COALESCE(sum(CASE WHEN status='active' THEN 1 ELSE 0 END), 0) FROM _bak_` + suffix + `_tags`).
		Scan(&removed, &voided); err != nil {
		return 0, 0, fmt.Errorf("count: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return 0, 0, err
	}
	return removed, voided, nil
}

// isPostgres 运行时判方言（restore 后 PG 序列需 setval 重推）。
// 探针 version() 为 PG 独有函数；SQLite 报 no such function → 非 PG。
func (r *CleanupRepo) isPostgres() bool {
	var v string
	return r.db.QueryRowContext(context.Background(), "SELECT version()").Scan(&v) == nil
}

// resetSequences PG 专用：显式插入 BIGSERIAL 行后序列不推进，重推到 MAX(id)（SQLite 自增无此问题）。
func (r *CleanupRepo) resetSequences(ctx context.Context) error {
	if !r.isPostgres() {
		return nil
	}
	for _, pair := range [][2]string{
		{"assets", "asset_id"}, {"asset_events", "event_id"}, {"scan_logs", "id"},
	} {
		tbl, col := pair[0], pair[1]
		if _, err := r.db.ExecContext(ctx,
			`SELECT setval(pg_get_serial_sequence(?, ?), COALESCE((SELECT MAX(`+col+`) FROM `+tbl+`), 1))`,
			tbl, col); err != nil {
			return fmt.Errorf("reset sequence %s: %w", tbl, err)
		}
	}
	return nil
}

// RestoreFromSnapshot 从快照回补被删行（已存在不覆盖），并把本次置 void 的 tags 恢复原状态。
func (r *CleanupRepo) RestoreFromSnapshot(ctx context.Context, suffix string) (restored int64, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// tags 状态恢复：快照为 active 且当前已 void 的（本 job 弄 void 的）
	if _, err = tx.ExecContext(ctx,
		`UPDATE tags SET status='active'
		 WHERE tag_id IN (SELECT tag_id FROM _bak_`+suffix+`_tags WHERE status='active')
		 AND status='void'`); err != nil {
		return 0, fmt.Errorf("restore tags: %w", err)
	}
	// 资产/履历/扫码：只补当前缺的行（NOT EXISTS 防覆盖）
	for _, q := range []string{
		`INSERT INTO assets SELECT b.* FROM _bak_` + suffix + `_assets b
		 WHERE NOT EXISTS (SELECT 1 FROM assets a WHERE a.asset_id = b.asset_id)`,
		`INSERT INTO asset_events SELECT b.* FROM _bak_` + suffix + `_asset_events b
		 WHERE NOT EXISTS (SELECT 1 FROM asset_events e WHERE e.event_id = b.event_id)`,
		`INSERT INTO scan_logs SELECT b.* FROM _bak_` + suffix + `_scan_logs b
		 WHERE NOT EXISTS (SELECT 1 FROM scan_logs s WHERE s.id = b.id)`,
	} {
		if _, err = tx.ExecContext(ctx, q); err != nil {
			return 0, fmt.Errorf("restore insert: %w", err)
		}
	}
	if err = tx.QueryRowContext(ctx,
		`SELECT count(*) FROM assets a JOIN _bak_` + suffix + `_assets b ON a.asset_id = b.asset_id`).
		Scan(&restored); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	// PG：显式插入 BIGSERIAL 行后序列停在旧值，重推避免后续 Create 撞 id
	if err := r.resetSequences(ctx); err != nil {
		return restored, err
	}
	return restored, nil
}

// MarshalPlan plan 草案 JSON 化（api 层结构 → cleanup_jobs.planned）。
func MarshalPlan(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}
