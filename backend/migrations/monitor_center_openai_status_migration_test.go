package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMonitorCenterOpenAIStatusMigration(t *testing.T) {
	content, err := FS.ReadFile("192_monitor_center_openai_status.sql")
	require.NoError(t, err)
	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS monitor_center_openai_history")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS monitor_center_openai_events")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS monitor_center_openai_incident_updates")
	require.Contains(t, sql, "UNIQUE (incident_id, updated_at)")
}

func TestMonitorCenterIncidentLifecycleMigration(t *testing.T) {
	content, err := FS.ReadFile("192_monitor_center_incident_lifecycle.sql")
	require.NoError(t, err)
	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS failure_reason")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS incident_refs")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS monitor_center_openai_incidents")
	require.Contains(t, sql, "affected_components JSONB")
	require.Contains(t, sql, "last_seen_at TIMESTAMPTZ")
}
