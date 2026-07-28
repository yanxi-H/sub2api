package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type performanceSinkTestRepo struct {
	opsRepoMock
	mu      sync.Mutex
	batches [][]OpsRequestPerformanceInput
	err     error
	gets    int
}

type blockingPerformanceSinkRepo struct {
	started chan struct{}
	once    sync.Once
}

func (r *blockingPerformanceSinkRepo) BatchInsertRequestPerformance(ctx context.Context, _ []*OpsRequestPerformanceInput) (int64, error) {
	r.once.Do(func() { close(r.started) })
	<-ctx.Done()
	return 0, ctx.Err()
}

func (r *performanceSinkTestRepo) InsertRequestPerformance(context.Context, *OpsRequestPerformanceInput) error {
	return nil
}

func (r *performanceSinkTestRepo) GetPerformanceDiagnostics(_ context.Context, filter *OpsDashboardFilter, _ int) (*OpsPerformanceDiagnosticsResponse, error) {
	r.mu.Lock()
	r.gets++
	r.mu.Unlock()
	return &OpsPerformanceDiagnosticsResponse{StartTime: filter.StartTime, EndTime: filter.EndTime}, nil
}

func (r *performanceSinkTestRepo) BatchInsertRequestPerformance(_ context.Context, inputs []*OpsRequestPerformanceInput) (int64, error) {
	batch := make([]OpsRequestPerformanceInput, 0, len(inputs))
	for _, input := range inputs {
		if input == nil {
			continue
		}
		cloned := *input
		if input.GroupID != nil {
			groupID := *input.GroupID
			cloned.GroupID = &groupID
		}
		batch = append(batch, cloned)
	}
	r.mu.Lock()
	r.batches = append(r.batches, batch)
	err := r.err
	r.mu.Unlock()
	if err != nil {
		return 0, err
	}
	return int64(len(batch)), nil
}

func (r *performanceSinkTestRepo) snapshot() [][]OpsRequestPerformanceInput {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([][]OpsRequestPerformanceInput, len(r.batches))
	for i := range r.batches {
		result[i] = append([]OpsRequestPerformanceInput(nil), r.batches[i]...)
	}
	return result
}

func (r *performanceSinkTestRepo) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.gets
}

func newPerformanceSinkForTest(repo opsRequestPerformanceBatchRepository, capacity, batchSize int, flushInterval time.Duration) *OpsRequestPerformanceSink {
	ctx, cancel := context.WithCancel(context.Background())
	return &OpsRequestPerformanceSink{
		repo:          repo,
		queue:         make(chan *OpsRequestPerformanceInput, capacity),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		ctx:           ctx,
		cancel:        cancel,
		accepting:     true,
	}
}

func TestOpsPerformanceSinkFlushesFullBatch(t *testing.T) {
	repo := &performanceSinkTestRepo{}
	sink := newPerformanceSinkForTest(repo, 4, 2, time.Hour)
	sink.Start()
	t.Cleanup(sink.Stop)

	require.True(t, sink.Enqueue(&OpsRequestPerformanceInput{RequestID: "one"}))
	require.True(t, sink.Enqueue(&OpsRequestPerformanceInput{RequestID: "two"}))
	require.Eventually(t, func() bool { return sink.Health().WrittenCount == 2 }, time.Second, 10*time.Millisecond)

	batches := repo.snapshot()
	require.Len(t, batches, 1)
	require.Len(t, batches[0], 2)
}

func TestOpsPerformanceSinkDropsOnlyTelemetryWhenQueueIsFull(t *testing.T) {
	repo := &performanceSinkTestRepo{}
	sink := newPerformanceSinkForTest(repo, 1, 10, time.Hour)

	require.True(t, sink.Enqueue(&OpsRequestPerformanceInput{RequestID: "accepted"}))
	require.False(t, sink.Enqueue(&OpsRequestPerformanceInput{RequestID: "dropped"}))
	require.Equal(t, uint64(1), sink.Health().DroppedCount)

	sink.Start()
	sink.Stop()
	require.Equal(t, uint64(1), sink.Health().WrittenCount)
}

func TestOpsPerformanceSinkStopDrainsAndClonesAcceptedInput(t *testing.T) {
	repo := &performanceSinkTestRepo{}
	sink := newPerformanceSinkForTest(repo, 4, 10, time.Hour)
	sink.Start()
	groupID := int64(7)
	input := &OpsRequestPerformanceInput{RequestID: "original", GroupID: &groupID}

	require.True(t, sink.Enqueue(input))
	input.RequestID = "mutated"
	groupID = 99
	sink.Stop()

	batches := repo.snapshot()
	require.Len(t, batches, 1)
	require.Len(t, batches[0], 1)
	require.Equal(t, "original", batches[0][0].RequestID)
	require.NotNil(t, batches[0][0].GroupID)
	require.Equal(t, int64(7), *batches[0][0].GroupID)
	require.False(t, sink.Enqueue(&OpsRequestPerformanceInput{RequestID: "after-stop"}))
}

func TestOpsPerformanceSinkWriteFailureDoesNotReachRequestCaller(t *testing.T) {
	repo := &performanceSinkTestRepo{err: errors.New("database unavailable")}
	sink := newPerformanceSinkForTest(repo, 4, 1, time.Hour)
	sink.Start()
	t.Cleanup(sink.Stop)
	svc := &OpsService{opsRepo: repo, performanceSink: sink}

	require.NoError(t, svc.RecordRequestPerformance(t.Context(), &OpsRequestPerformanceInput{RequestID: "request-1"}))
	require.Eventually(t, func() bool { return sink.Health().WriteFailed == 1 }, time.Second, 10*time.Millisecond)
	require.Zero(t, sink.Health().WrittenCount)
}

func TestOpsPerformanceSinkStopCancelsActiveWriteAndBoundsDrain(t *testing.T) {
	repo := &blockingPerformanceSinkRepo{started: make(chan struct{})}
	sink := newPerformanceSinkForTest(repo, 8, 1, time.Hour)
	sink.drainTimeout = 40 * time.Millisecond
	sink.Start()

	require.True(t, sink.Enqueue(&OpsRequestPerformanceInput{RequestID: "active"}))
	select {
	case <-repo.started:
	case <-time.After(time.Second):
		t.Fatal("performance sink did not start its first write")
	}
	for index := 0; index < 3; index++ {
		require.True(t, sink.Enqueue(&OpsRequestPerformanceInput{RequestID: fmt.Sprintf("queued-%d", index)}))
	}

	startedAt := time.Now()
	sink.Stop()
	require.Less(t, time.Since(startedAt), time.Second)

	health := sink.Health()
	require.Zero(t, health.QueueDepth)
	require.Equal(t, uint64(4), health.WriteFailed+health.DroppedCount)
}
