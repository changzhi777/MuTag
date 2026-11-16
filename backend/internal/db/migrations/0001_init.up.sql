-- M1 核心四表（SQLite 方言 · 开发/验收形态；上线形态 PG 方言见 git 历史）
-- 纪律：tagId 永不 recycle（报废置 void 归档，无复用）

CREATE TABLE IF NOT EXISTS tags (
    tag_id     TEXT PRIMARY KEY,                -- 码号，如 MT-2026-000001，永不复用
    qr_scene   TEXT UNIQUE,                     -- 小程序码 scene：t:{tag_id}
    url_code   TEXT UNIQUE NOT NULL,            -- 活码 URL 路径段
    status     TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'void')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now'))
);

CREATE TABLE IF NOT EXISTS assets (
    asset_id     INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_id       TEXT NOT NULL UNIQUE REFERENCES tags (tag_id),
    name         TEXT NOT NULL,
    category     TEXT,
    model        TEXT,
    sn           TEXT,
    buy_date     TEXT,                          -- YYYY-MM-DD
    price        REAL,
    dept_id      TEXT,
    keeper       TEXT,                          -- 保管人（M1 存原文本，M2+ 规范化关联）
    location     TEXT,
    status       TEXT NOT NULL DEFAULT 'in_use'
                 CHECK (status IN ('in_use', 'idle', 'repair', 'borrowed', 'scrapped')),
    unit         TEXT,                          -- 计量单位：个/条/台/套/项/根
    warranty_end TEXT,
    remark       TEXT,
    created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now')),
    updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_assets_keeper ON assets (keeper);
CREATE INDEX IF NOT EXISTS idx_assets_status ON assets (status);

CREATE TABLE IF NOT EXISTS asset_events (
    event_id   INTEGER PRIMARY KEY AUTOINCREMENT,
    asset_id   INTEGER NOT NULL REFERENCES assets (asset_id),
    type       TEXT NOT NULL CHECK (type IN
               ('create', 'receive', 'return', 'repair', 'transfer', 'scrap', 'update')),
    actor      TEXT NOT NULL,
    old_value  TEXT,                          -- JSON
    new_value  TEXT,                          -- JSON
    note       TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_events_asset ON asset_events (asset_id, created_at);

CREATE TABLE IF NOT EXISTS scan_logs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_id     TEXT NOT NULL,
    actor      TEXT,
    channel    TEXT NOT NULL DEFAULT 'h5' CHECK (channel IN ('weapp', 'h5', 'gun')),
    ip         TEXT,
    user_agent TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now'))
);

CREATE INDEX IF NOT EXISTS idx_scan_logs_tag ON scan_logs (tag_id, created_at);
