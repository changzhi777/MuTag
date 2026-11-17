-- P2：存量重复清洗——审计 job 表（规划任务 3.1）
-- 快照表（_bak_{suffix}_{table}）执行期动态创建，不入迁移
CREATE TABLE IF NOT EXISTS cleanup_jobs (
    job_id         TEXT PRIMARY KEY,
    actor          TEXT NOT NULL,
    mode           TEXT NOT NULL DEFAULT 'all',      -- all | align
    keep_strategy  TEXT NOT NULL DEFAULT 'earliest_1', -- earliest_1 | earliest_n | latest_n | manual
    summary        TEXT NOT NULL DEFAULT '{}',       -- JSON：{groups, delete, void, keep}
    planned        TEXT NOT NULL DEFAULT '[]',       -- JSON：plan 草案（组+keep/remove+action）
    removed_count  BIGINT NOT NULL DEFAULT 0,
    voided_count   BIGINT NOT NULL DEFAULT 0,
    snapshot_suffix TEXT,                            -- 快照表后缀（执行时生成）
    status         TEXT NOT NULL DEFAULT 'planned'
                   CHECK (status IN ('planned', 'executed', 'restored', 'failed')),
    created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now')),
    executed_at    TEXT,
    restored_at    TEXT
);
