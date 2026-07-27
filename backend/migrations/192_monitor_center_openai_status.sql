-- 监控中心：OpenAI 官方状态轮询历史与事件归档。
-- 由 MonitorCenterService 后台 goroutine 每分钟写入（status.openai.com）。
-- 全部 IF NOT EXISTS，幂等可重复执行。
CREATE TABLE IF NOT EXISTS monitor_center_openai_history (
    bucket_start TIMESTAMPTZ PRIMARY KEY,
    overall_status VARCHAR(32) NOT NULL,
    api_status VARCHAR(32) NOT NULL,
    chatgpt_status VARCHAR(32) NOT NULL,
    codex_status VARCHAR(32) NOT NULL,
    active_incident_count INTEGER NOT NULL DEFAULT 0,
    fetch_status VARCHAR(16) NOT NULL,
    latency_ms INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_monitor_center_openai_history_bucket_desc
    ON monitor_center_openai_history (bucket_start DESC);

CREATE TABLE IF NOT EXISTS monitor_center_openai_events (
    id BIGSERIAL PRIMARY KEY,
    observed_at TIMESTAMPTZ NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    normalized_state JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_monitor_center_openai_events_observed_desc
    ON monitor_center_openai_events (observed_at DESC);

CREATE TABLE IF NOT EXISTS monitor_center_openai_incident_updates (
    id BIGSERIAL PRIMARY KEY,
    incident_id VARCHAR(64) NOT NULL,
    incident_name VARCHAR(300) NOT NULL DEFAULT '',
    incident_status VARCHAR(32) NOT NULL DEFAULT '',
    impact VARCHAR(32) NOT NULL DEFAULT '',
    update_status VARCHAR(32) NOT NULL DEFAULT '',
    update_body TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT monitor_center_openai_incident_update_unique UNIQUE (incident_id, updated_at)
);

CREATE INDEX IF NOT EXISTS idx_monitor_center_openai_incident_updates_updated_desc
    ON monitor_center_openai_incident_updates (updated_at DESC);
