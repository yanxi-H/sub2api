package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type requestBodyLeaseRefreshStub struct {
	refreshed bool
	err       error
}

func (s *requestBodyLeaseRefreshStub) RefreshRequestBodyLane(context.Context, RequestBodyLane, int64, int64, int, string) (bool, error) {
	return s.refreshed, s.err
}

func TestRefreshRequestBodyAdmissionLeaseCancelsOwnerWhenLeaseIsLost(t *testing.T) {
	ownerCtx, cancelOwner := context.WithCancel(context.Background())
	ctx := WithRequestBodyAdmissionLeaseLossCancel(context.Background(), cancelOwner)

	refreshed, err := refreshRequestBodyAdmissionLease(ctx, &requestBodyLeaseRefreshStub{}, RequestBodyLaneHeavy, 1, 2, 1, "lost")

	require.NoError(t, err)
	require.False(t, refreshed)
	require.ErrorIs(t, ownerCtx.Err(), context.Canceled)
}

func TestRefreshRequestBodyAdmissionLeaseKeepsOwnerWhenLeaseExists(t *testing.T) {
	ownerCtx, cancelOwner := context.WithCancel(context.Background())
	t.Cleanup(cancelOwner)
	ctx := WithRequestBodyAdmissionLeaseLossCancel(context.Background(), cancelOwner)

	refreshed, err := refreshRequestBodyAdmissionLease(ctx, &requestBodyLeaseRefreshStub{refreshed: true}, RequestBodyLaneRecovery, 1, 2, 1, "active")

	require.NoError(t, err)
	require.True(t, refreshed)
	require.NoError(t, ownerCtx.Err())
}
