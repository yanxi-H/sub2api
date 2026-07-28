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
