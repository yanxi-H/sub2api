-- Record inbound request body size for dashboard diagnostics.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS request_body_bytes BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_usage_logs_user_created_request_body_bytes
    ON usage_logs (user_id, created_at)
    WHERE request_body_bytes > 0;
