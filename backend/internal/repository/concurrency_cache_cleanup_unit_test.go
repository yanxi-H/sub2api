//go:build unit

package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestBodyAdmissionUserIDFromKey(t *testing.T) {
	userPrefix := requestBodyLaneWaitKeyPrefix + "{admission}:user:"
	tests := []struct {
		name   string
		key    string
		userID int64
		ok     bool
	}{
		{name: "direct user key", key: userPrefix + "1002", userID: 1002, ok: true},
		{
			name:   "integration namespace user key",
			key:    "it:TestConcurrencyCacheSuite:47:" + userPrefix + "1002",
			userID: 1002,
			ok:     true,
		},
		{
			name: "scope wait key",
			key:  requestBodyLaneWaitKeyPrefix + "{admission}:recovery:scope:0",
		},
		{name: "missing user id", key: userPrefix},
		{name: "nested suffix", key: userPrefix + "1002:extra"},
		{name: "zero user id", key: userPrefix + "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, ok := requestBodyAdmissionUserIDFromKey(tt.key, userPrefix)
			require.Equal(t, tt.ok, ok)
			require.Equal(t, tt.userID, userID)
		})
	}
}
