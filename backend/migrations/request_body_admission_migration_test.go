package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration187ReplacesLegacyBodyLimitAndCompactBypass(t *testing.T) {
	content, err := FS.ReadFile("187_migrate_request_body_admission.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "request_body_admission_enabled")
	require.Contains(t, sql, "request_body_normal_limit_bytes")
	require.Contains(t, sql, "request_body_heavy_limit_bytes")
	require.Contains(t, sql, "request_body_recovery_limit_bytes")
	require.Contains(t, sql, "- 'request_body_limit_bytes'")
	require.Contains(t, sql, "- 'allow_compact_request_body_limit_bypass'")
	require.Contains(t, sql, "legacy.legacy_limit > 0")
	require.Contains(t, sql, "|| (\n        COALESCE(account.extra")
	require.Contains(t, sql, "platform IS DISTINCT FROM 'openai'")
}

func TestMigration188BoundsRecoveryChannelForExistingAccounts(t *testing.T) {
	content, err := FS.ReadFile("188_bound_request_body_recovery_channel.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "request_body_admission_enabled")
	require.Contains(t, sql, "request_body_normal_limit_bytes")
	require.Contains(t, sql, "request_body_heavy_limit_bytes")
	require.Contains(t, sql, "request_body_recovery_limit_bytes")
	require.Contains(t, sql, "33554432")
	require.Contains(t, sql, "platform = 'openai'")
}
