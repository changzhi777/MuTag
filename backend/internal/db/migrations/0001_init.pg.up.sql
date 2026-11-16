-- M1 核心四表（PG 方言 · 上线形态；开发/验收 SQLite 方言见 0001_init.up.sql）
-- 架构裁决：双方言同构 TEXT 时间戳（RFC3339 UTC 毫秒 +00:00，应用层显式传值——repo 层零方言函数）；
-- DEFAULT 仅为防御兜底（now()::text 服务器时区格式，主链路不会触发）。
-- 纪律：tagId 永不 recycle（报废置 void 归档，无复用）

CREATE TABLE IF NOT EXISTS tags (
    tag_id     TEXT PRIMARY KEY,             -- 码号，如 MT-2026-000001，永不复用
    qr_scene   TEXT UNIQUE,                  -- 小程序码 scene：t:{tag_id}
    url_code   TEXT UNIQUE NOT NULL,         -- 活码 URL 路径段
    status     TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'void')),
    created_at TEXT NOT NULL DEFAULT (now()::text)
);

CREATE TABLE IF NOT EXISTS assets (
    asset_id     BIGSERIAL PRIMARY KEY,
    tag_id       TEXT NOT NULL UNIQUE REFERENCES tags (tag_id),
    name         TEXT NOT NULL,
    category     TEXT,
    model        TEXT,
    sn           TEXT,
    buy_date     TEXT,                       -- YYYY-MM-DD
    price        DOUBLE PRECISION,
    dept_id      TEXT,
    keeper       TEXT,                       -- 保管人
    location     TEXT,
    status       TEXT NOT NULL DEFAULT 'in_use'
                 CHECK (status IN ('in_use', 'idle', 'repair', 'borrowed', 'scrapped')),
    unit         TEXT,                       -- 计量单位：个/条/台/套/项/根
    warranty_end TEXT,
    remark       TEXT,
    created_at   TEXT NOT NULL DEFAULT (now()::text),
    updated_at   TEXT NOT NULL DEFAULT (now()::text)
);

CREATE INDEX IF NOT EXISTS idx_assets_keeper ON assets (keeper);
CREATE INDEX IF NOT EXISTS idx_assets_status ON assets (status);

CREATE TABLE IF NOT EXISTS asset_events (
    event_id   BIGSERIAL PRIMARY KEY,
    asset_id   BIGINT NOT NULL REFERENCES assets (asset_id),
    type       TEXT NOT NULL CHECK (type IN
               ('create', 'receive', 'return', 'repair', 'transfer', 'scrap', 'update')),
    actor      TEXT NOT NULL,
    old_value  TEXT,                         -- JSON
    new_value  TEXT,                         -- JSON
    note       TEXT,
    created_at TEXT NOT NULL DEFAULT (now()::text)
);

CREATE INDEX IF NOT EXISTS idx_events_asset ON asset_events (asset_id, created_at);

CREATE TABLE IF NOT EXISTS scan_logs (
    id         BIGSERIAL PRIMARY KEY,
    tag_id     TEXT NOT NULL,
    actor      TEXT,
    channel    TEXT NOT NULL DEFAULT 'h5' CHECK (channel IN ('weapp', 'h5', 'gun')),
    ip         TEXT,
    user_agent TEXT,
    created_at TEXT NOT NULL DEFAULT (now()::text)
);

CREATE INDEX IF NOT EXISTS idx_scan_logs_tag ON scan_logs (tag_id, created_at);
