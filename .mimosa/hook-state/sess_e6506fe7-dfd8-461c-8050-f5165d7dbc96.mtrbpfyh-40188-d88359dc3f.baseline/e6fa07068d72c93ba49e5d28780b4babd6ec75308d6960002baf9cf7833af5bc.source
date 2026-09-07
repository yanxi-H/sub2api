package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration190CreatesRequestPerformanceTelemetry(t *testing.T) {
	content, err := FS.ReadFile("193_ops_request_performance.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS ops_request_performance")
	require.Contains(t, sql, "UNIQUE (request_id, api_key_id)")
	require.Contains(t, sql, "stream BOOLEAN")
	require.Contains(t, sql, "time_to_first_token_ms")
	require.Contains(t, sql, "max_stream_gap_ms")
	require.Contains(t, sql, "failover_ms")
	require.Contains(t, sql, "attempt_count")
	require.Contains(t, sql, "failure_cause")
	require.Contains(t, sql, "slow_cause")
	require.Contains(t, sql, "idx_ops_request_performance_created_at")
}
