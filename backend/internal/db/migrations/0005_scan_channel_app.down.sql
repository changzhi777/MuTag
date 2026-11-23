-- 0005 (SQLite down) 恢复旧渠道白名单（channel='app' 记录随重建丢弃）。
ALTER TABLE scan_logs RENAME TO scan_logs_new;
CREATE TABLE scan_logs (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_id     TEXT NOT NULL,
    actor      TEXT,
    channel    TEXT NOT NULL DEFAULT 'h5' CHECK (channel IN ('weapp', 'h5', 'gun')),
    ip         TEXT,
    user_agent TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now'))
);
INSERT INTO scan_logs (id, tag_id, actor, channel, ip, user_agent, created_at)
    SELECT id, tag_id, actor, channel, ip, user_agent, created_at FROM scan_logs_new WHERE channel != 'app';
DROP TABLE scan_logs_new;
CREATE INDEX IF NOT EXISTS idx_scan_logs_tag ON scan_logs (tag_id, created_at);
