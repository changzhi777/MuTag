-- 0007 (PG) 后台多管理员：admins 表（兼容旧 ADMIN_PASSWORD 单密码形态双轨）。
CREATE TABLE admins (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,           -- 格式 sha256$<salt-hex>$<hash-hex>（salt 16B 随机）
    display_name  TEXT,
    created_at    TEXT NOT NULL DEFAULT (now()::text)
);
