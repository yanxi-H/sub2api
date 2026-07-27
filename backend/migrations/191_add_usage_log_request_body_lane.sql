-- 记录三通道分类(normal/heavy/recovery)用于运维监控。
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS request_body_lane VARCHAR(16);

COMMENT ON COLUMN usage_logs.request_body_lane IS
    'Three-lane body admission classification: normal, heavy, or recovery. NULL for non-OpenAI or pre-admission traffic.';
