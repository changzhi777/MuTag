-- M2：微信用户表（员工端登录态与角色）· PG 方言
-- 上线形态 PG；字段语义与 SQLite 方言（0002_users.up.sql）一致

CREATE TABLE IF NOT EXISTS users (
    openid     TEXT PRIMARY KEY,             -- 微信 openid（dev 模式为 mock 值）
    nickname   TEXT,
    role       TEXT NOT NULL DEFAULT 'staff' CHECK (role IN ('admin', 'asset_mgr', 'staff')),
    created_at TEXT NOT NULL DEFAULT (now()::text)
);
