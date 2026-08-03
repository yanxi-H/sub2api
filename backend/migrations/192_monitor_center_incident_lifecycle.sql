ALTER TABLE monitor_center_openai_history
    ADD COLUMN IF NOT EXISTS failure_reason TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS incident_refs JSONB NOT NULL DEFAULT '{}'::jsonb;

CREATE TABLE IF NOT EXISTS monitor_center_openai_incidents (
    incident_id VARCHAR(64) PRIMARY KEY,
    incident_name VARCHAR(300) NOT NULL DEFAULT '',
    incident_status VARCHAR(32) NOT NULL DEFAULT '',
    impact VARCHAR(32) NOT NULL DEFAULT '',
    affected_components JSONB NOT NULL DEFAULT '[]'::jsonb,
    affected_groups JSONB NOT NULL DEFAULT '[]'::jsonb,
    started_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL,
    resolved_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ NOT NULL,
    incident_url TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_monitor_center_openai_incidents_window
    ON monitor_center_openai_incidents (started_at, last_seen_at);

ALTER TABLE monitor_center_openai_incident_updates
    ADD COLUMN IF NOT EXISTS affected_components JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS affected_groups JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS incident_url TEXT NOT NULL DEFAULT '';
