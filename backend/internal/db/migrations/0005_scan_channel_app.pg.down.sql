-- 0005 (PG down) 恢复旧渠道白名单（注意：若已存在 channel='app' 记录，本 down 会因 CHECK 失败——先清理再降级）。
ALTER TABLE scan_logs DROP CONSTRAINT scan_logs_channel_check;
ALTER TABLE scan_logs ADD CONSTRAINT scan_logs_channel_check
    CHECK (channel IN ('weapp', 'h5', 'gun'));
