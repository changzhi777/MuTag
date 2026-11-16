-- M4：财务模板 + 导出审计 + 导入批次（差异预览/确认/按批次回滚）· PG 方言
-- 语义与 SQLite 方言（0003_fin.up.sql）一致；JSON 列保持 TEXT（无 JSON 查询需求，双方言同构）

CREATE TABLE IF NOT EXISTS fin_mappings (
    mapping_id TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    kind       TEXT NOT NULL UNIQUE,
    mapping    TEXT NOT NULL,              -- JSON
    version    BIGINT NOT NULL DEFAULT 1,  -- 每次编辑 +1（可追溯）
    updated_at TEXT NOT NULL DEFAULT (now()::text),
    created_at TEXT NOT NULL DEFAULT (now()::text)
);

CREATE TABLE IF NOT EXISTS export_jobs (
    job_id     TEXT PRIMARY KEY,
    mapping_id TEXT NOT NULL,
    filters    TEXT NOT NULL,              -- JSON：status/dept/buy_from/buy_to/keyword
    actor      TEXT NOT NULL,
    row_count  BIGINT NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (now()::text)
);

CREATE INDEX IF NOT EXISTS idx_export_jobs_time ON export_jobs (created_at DESC);

CREATE TABLE IF NOT EXISTS import_batches (
    batch_id       TEXT PRIMARY KEY,
    mapping_id     TEXT NOT NULL,
    file_name      TEXT,
    summary        TEXT NOT NULL DEFAULT '{}', -- JSON：{creates,updates,errors} 计数
    items          TEXT NOT NULL DEFAULT '{}', -- JSON：全量明细
    status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'confirmed', 'rolled_back')),
    actor          TEXT NOT NULL,
    created_at     TEXT NOT NULL DEFAULT (now()::text),
    confirmed_at   TEXT,
    rolled_back_at TEXT
);
