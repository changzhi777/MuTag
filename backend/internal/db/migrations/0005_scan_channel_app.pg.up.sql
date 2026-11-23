-- 0005 (PG) 扫码渠道扩 app：青沐码 APK WebView UA 标识（QingMuApp）→ channel='app'。
-- 旧 CHECK 白名单 ('weapp','h5','gun') 拒绝 'app' 致分流器 Insert 静默失败（错误被吞），
-- 表现为 APK 扫码不入 scan_logs、后台计数不涨。本迁移重建约束白名单。
ALTER TABLE scan_logs DROP CONSTRAINT scan_logs_channel_check;
ALTER TABLE scan_logs ADD CONSTRAINT scan_logs_channel_check
    CHECK (channel IN ('weapp', 'h5', 'gun', 'app'));
