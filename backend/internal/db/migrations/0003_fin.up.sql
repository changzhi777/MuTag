-- M4：财务模板 + 导出审计 + 导入批次（差异预览/确认/按批次回滚）
-- 上线形态 PG 方言时需对齐本文件语义（SQLite TEXT/JSON 均存 TEXT）

-- 财务模板：column_map 存 JSON（模板列名→系统字段 + 日期格式 + 状态字典 + fixed_rows）
-- kind 为模板标识（yonyou_u8/kingdee/guanjiapo/generic），内置默认由 repo 层 EnsureDefaults 播种
CREATE TABLE IF NOT EXISTS fin_mappings (
    mapping_id TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    kind       TEXT NOT NULL UNIQUE,
    mapping    TEXT NOT NULL,              -- JSON
    version    INTEGER NOT NULL DEFAULT 1, -- 每次编辑 +1（可追溯）
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now'))
);

-- 导出任务审计：财务合规留痕（谁/何时/用哪个模板/什么过滤条件/导出几行）
CREATE TABLE IF NOT EXISTS export_jobs (
    job_id     TEXT PRIMARY KEY,
    mapping_id TEXT NOT NULL,
    filters    TEXT NOT NULL,              -- JSON：status/dept/buy_from/buy_to/keyword
    actor      TEXT NOT NULL,
    row_count  INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_export_jobs_time ON export_jobs (created_at DESC);

-- 导入批次：preview 时落库（pending）→ confirm 确认入库 → 支持按批次回滚
-- items 存 JSON 明细：creates（confirm 后回填 tag/asset_id）+ updates（含 old 值快照）+ errors
CREATE TABLE IF NOT EXISTS import_batches (
    batch_id       TEXT PRIMARY KEY,
    mapping_id     TEXT NOT NULL,
    file_name      TEXT,
    summary        TEXT NOT NULL DEFAULT '{}', -- JSON：{creates,updates,errors} 计数
    items          TEXT NOT NULL DEFAULT '{}', -- JSON：全量明细
    status         TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending', 'confirmed', 'rolled_back')),
    actor          TEXT NOT NULL,
    created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now')),
    confirmed_at   TEXT,
    rolled_back_at TEXT
);
