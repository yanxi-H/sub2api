package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestBodyAdmissionPolicyDefaultsAndClassification(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Extra: map[string]any{
			RequestBodyAdmissionEnabledExtraKey: true,
		},
	}
	policy := account.GetRequestBodyAdmissionPolicy()
	require.Equal(t, DefaultRequestBodyNormalLimitBytes, policy.NormalLimitBytes)
	require.Equal(t, DefaultRequestBodyHeavyLimitBytes, policy.HeavyLimitBytes)
	require.Equal(t, DefaultRequestBodyRecoveryLimitBytes, policy.RecoveryLimitBytes)
	require.Equal(t, RequestBodyLaneNormal, policy.Classify(policy.NormalLimitBytes, false))
	require.Equal(t, RequestBodyLaneHeavy, policy.Classify(policy.NormalLimitBytes+1, false))
	require.Equal(t, RequestBodyLaneRejected, policy.Classify(policy.HeavyLimitBytes+1, false))
	require.Equal(t, RequestBodyLaneRecovery, policy.Classify(1, true))
	require.Equal(t, RequestBodyLaneRejected, policy.Classify(policy.RecoveryLimitBytes+1, true))
}

func TestRequestBodyHeavyConcurrencyLimit(t *testing.T) {
	require.Equal(t, 1, RequestBodyHeavyConcurrencyLimit(1))
	require.Equal(t, 1, RequestBodyHeavyConcurrencyLimit(5))
	require.Equal(t, 1, RequestBodyHeavyConcurrencyLimit(10))
	require.Equal(t, 1, RequestBodyHeavyConcurrencyLimit(20))
}

func TestRequestBodyLargeAccountConcurrencyLimitReservesOrdinarySlot(t *testing.T) {
	require.Equal(t, 0, RequestBodyLargeAccountConcurrencyLimit(0))
	require.Equal(t, 1, RequestBodyLargeAccountConcurrencyLimit(1))
	require.Equal(t, 1, RequestBodyLargeAccountConcurrencyLimit(2))
	require.Equal(t, 4, RequestBodyLargeAccountConcurrencyLimit(5))
}

func TestRequestBodyLaneWaitLimit(t *testing.T) {
	require.Equal(t, 1, RequestBodyLaneWaitLimit(0))
	require.Equal(t, 1, RequestBodyLaneWaitLimit(1))
	require.Equal(t, 4, RequestBodyLaneWaitLimit(4))
}

func TestNormalizeRequestBodyAdmissionExtraRemovesLegacyFields(t *testing.T) {
	extra, err := normalizeRequestBodyAdmissionExtra(PlatformOpenAI, map[string]any{
		LegacyRequestBodyLimitExtraKey:       int64(5),
		LegacyCompactBodyLimitBypassExtraKey: true,
		RequestBodyAdmissionEnabledExtraKey:  true,
		RequestBodyNormalLimitExtraKey:       int64(10),
		RequestBodyHeavyLimitExtraKey:        int64(20),
		RequestBodyRecoveryLimitExtraKey:     int64(30),
	})
	require.NoError(t, err)
	require.NotContains(t, extra, LegacyRequestBodyLimitExtraKey)
	require.NotContains(t, extra, LegacyCompactBodyLimitBypassExtraKey)
	require.Equal(t, true, extra[RequestBodyAdmissionEnabledExtraKey])
}

func TestNormalizeRequestBodyAdmissionExtraRemovesPolicyFromOtherPlatforms(t *testing.T) {
	extra, err := normalizeRequestBodyAdmissionExtra(PlatformAnthropic, map[string]any{
		LegacyRequestBodyLimitExtraKey:       int64(5),
		LegacyCompactBodyLimitBypassExtraKey: true,
		RequestBodyAdmissionEnabledExtraKey:  true,
		RequestBodyNormalLimitExtraKey:       int64(10),
		RequestBodyHeavyLimitExtraKey:        int64(20),
		RequestBodyRecoveryLimitExtraKey:     int64(30),
		"another_setting":                    true,
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"another_setting": true}, extra)
}

func TestNormalizeRequestBodyAdmissionExtraRejectsUnorderedLimits(t *testing.T) {
	_, err := normalizeRequestBodyAdmissionExtra(PlatformOpenAI, map[string]any{
		RequestBodyAdmissionEnabledExtraKey: true,
		RequestBodyNormalLimitExtraKey:      int64(20),
		RequestBodyHeavyLimitExtraKey:       int64(10),
		RequestBodyRecoveryLimitExtraKey:    int64(30),
	})
	require.Error(t, err)
}

func TestNormalizeRequestBodyAdmissionExtraRejectsLimitAboveRuntimeMaximum(t *testing.T) {
	_, err := normalizeRequestBodyAdmissionExtra(PlatformOpenAI, map[string]any{
		RequestBodyAdmissionEnabledExtraKey: true,
		RequestBodyNormalLimitExtraKey:      int64(10),
		RequestBodyHeavyLimitExtraKey:       int64(20),
		RequestBodyRecoveryLimitExtraKey:    MaxRequestBodyAdmissionLimitBytes + 1,
	})
	require.Error(t, err)
}

func TestNormalizeRequestBodyAdmissionExtraRejectsRecoveryLimitAboveSafeMaximum(t *testing.T) {
	_, err := normalizeRequestBodyAdmissionExtra(PlatformOpenAI, map[string]any{
		RequestBodyAdmissionEnabledExtraKey: true,
		RequestBodyNormalLimitExtraKey:      int64(10),
		RequestBodyHeavyLimitExtraKey:       int64(20),
		RequestBodyRecoveryLimitExtraKey:    MaxRequestBodyRecoveryLimitBytes + 1,
	})
	require.Error(t, err)
}

func TestRequestBodyAdmissionPolicyFallsBackWhenPersistedLimitExceedsRuntimeMaximum(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Extra: map[string]any{
			RequestBodyAdmissionEnabledExtraKey: true,
			RequestBodyNormalLimitExtraKey:      int64(10),
			RequestBodyHeavyLimitExtraKey:       int64(20),
			RequestBodyRecoveryLimitExtraKey:    MaxRequestBodyAdmissionLimitBytes + 1,
		},
	}

	policy := account.GetRequestBodyAdmissionPolicy()
	require.Equal(t, DefaultRequestBodyNormalLimitBytes, policy.NormalLimitBytes)
	require.Equal(t, DefaultRequestBodyHeavyLimitBytes, policy.HeavyLimitBytes)
	require.Equal(t, DefaultRequestBodyRecoveryLimitBytes, policy.RecoveryLimitBytes)
}

func TestNormalizeRequestBodyAdmissionUpdatePreservesUnrelatedPolicyFields(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Extra: map[string]any{
			RequestBodyAdmissionEnabledExtraKey: true,
			RequestBodyNormalLimitExtraKey:      int64(10),
			RequestBodyHeavyLimitExtraKey:       int64(20),
			RequestBodyRecoveryLimitExtraKey:    int64(30),
		},
	}
	input := &UpdateAccountInput{Extra: map[string]any{"another_setting": true}}
	normalized, err := normalizeRequestBodyAdmissionUpdateExtra(account, input, input.Extra)
	require.NoError(t, err)
	require.Equal(t, true, normalized[RequestBodyAdmissionEnabledExtraKey])
	require.EqualValues(t, 10, normalized[RequestBodyNormalLimitExtraKey])
	require.EqualValues(t, 20, normalized[RequestBodyHeavyLimitExtraKey])
	require.EqualValues(t, 30, normalized[RequestBodyRecoveryLimitExtraKey])
}
