-- 0005 (SQLite) 扫码渠道扩 app：青沐码 APK WebView UA 标识（QingMuApp）→ channel='app'。
-- SQLite 不能 ALTER CHECK，标准重建表法（本迁移在事务内执行）。
CREATE TABLE scan_logs_new (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    tag_id     TEXT NOT NULL,
    actor      TEXT,
    channel    TEXT NOT NULL DEFAULT 'h5' CHECK (channel IN ('weapp', 'h5', 'gun', 'app')),
    ip         TEXT,
    user_agent TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%f+00:00', 'now'))
);
INSERT INTO scan_logs_new (id, tag_id, actor, channel, ip, user_agent, created_at)
    SELECT id, tag_id, actor, channel, ip, user_agent, created_at FROM scan_logs;
DROP TABLE scan_logs;
ALTER TABLE scan_logs_new RENAME TO scan_logs;
CREATE INDEX IF NOT EXISTS idx_scan_logs_tag ON scan_logs (tag_id, created_at);
