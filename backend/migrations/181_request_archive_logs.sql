-- 请求文本存档表:全量记录用户 prompt 文本用于风控筛查。
-- 独立于 usage_logs(避免拖慢计费插入),异步批量写入,30天自动清理。
CREATE TABLE IF NOT EXISTS request_archive_logs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    request_id VARCHAR(128) NOT NULL,
    user_id BIGINT NOT NULL,
    user_email VARCHAR(255),
    api_key_id BIGINT NOT NULL,
    api_key_name VARCHAR(255),
    group_id BIGINT,
    endpoint VARCHAR(128),
    protocol VARCHAR(32),
    model VARCHAR(128),
    ip_address VARCHAR(64),
    prompt_text TEXT NOT NULL DEFAULT '',
    truncated BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_request_archive_created_at ON request_archive_logs (created_at);
CREATE INDEX IF NOT EXISTS idx_request_archive_user_id ON request_archive_logs (user_id);
CREATE INDEX IF NOT EXISTS idx_request_archive_api_key_id ON request_archive_logs (api_key_id);
-- GIN 全文搜索索引(用于关键词筛查)
CREATE INDEX IF NOT EXISTS idx_request_archive_prompt_search ON request_archive_logs USING GIN (to_tsvector('simple', prompt_text));
