-- M2：微信用户表（员工端登录态与角色）
-- 上线形态 PG 时由 0002 对应 PG 方言版本覆盖，字段语义一致

CREATE TABLE IF NOT EXISTS users (
    openid     TEXT PRIMARY KEY,              -- 微信 openid（dev 模式为 mock 值）
    nickname   TEXT,
    role       TEXT NOT NULL DEFAULT 'staff' CHECK (role IN ('admin', 'asset_mgr', 'staff')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now'))
);
