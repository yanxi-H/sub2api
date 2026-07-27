-- 监控中心性能诊断：逐请求性能采样（端到端/队列/TTFT/上游耗时分解）。
-- 由网关在请求结束时写入（ops_performance 采集链路），供慢请求排行/影响表/性能诊断面板查询。
-- 全部 IF NOT EXISTS，幂等可重复执行。
CREATE TABLE IF NOT EXISTS ops_request_performance (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    request_id VARCHAR(128) NOT NULL,
    user_id BIGINT NOT NULL,
    api_key_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    group_id BIGINT,
    platform VARCHAR(32) NOT NULL DEFAULT 'openai',
    model VARCHAR(100) NOT NULL DEFAULT '',
    stream BOOLEAN NOT NULL DEFAULT FALSE,
    request_body_lane VARCHAR(16) NOT NULL DEFAULT 'normal',
    request_body_bytes BIGINT NOT NULL DEFAULT 0,
    logical_status_code INTEGER NOT NULL DEFAULT 200,
    end_to_end_ms BIGINT NOT NULL DEFAULT 0,
    body_read_ms BIGINT NOT NULL DEFAULT 0,
    user_queue_ms BIGINT NOT NULL DEFAULT 0,
    body_lane_wait_ms BIGINT NOT NULL DEFAULT 0,
    account_queue_ms BIGINT NOT NULL DEFAULT 0,
    routing_ms BIGINT NOT NULL DEFAULT 0,
    upstream_ms BIGINT NOT NULL DEFAULT 0,
    time_to_first_token_ms BIGINT NOT NULL DEFAULT 0,
    stream_duration_ms BIGINT NOT NULL DEFAULT 0,
    max_stream_gap_ms BIGINT NOT NULL DEFAULT 0,
    failover_ms BIGINT NOT NULL DEFAULT 0,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    account_switch_count INTEGER NOT NULL DEFAULT 0,
    failure_cause VARCHAR(32) NOT NULL DEFAULT '',
    slow_cause VARCHAR(32) NOT NULL DEFAULT 'healthy',
    CONSTRAINT ops_request_performance_request_api_key_unique UNIQUE (request_id, api_key_id)
);

CREATE INDEX IF NOT EXISTS idx_ops_request_performance_created_at
    ON ops_request_performance (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_request_performance_user_created
    ON ops_request_performance (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_request_performance_account_created
    ON ops_request_performance (account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ops_request_performance_lane_created
    ON ops_request_performance (request_body_lane, created_at DESC);
