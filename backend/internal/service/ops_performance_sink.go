package service

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	opsPerformanceQueueCapacity = 32 * 1024
	opsPerformanceBatchSize     = 200
	opsPerformanceFlushInterval = 500 * time.Millisecond
	opsPerformanceWriteTimeout  = 5 * time.Second
	opsPerformanceDrainTimeout  = 10 * time.Second
)

type OpsRequestPerformanceSinkHealth struct {
	QueueDepth    int64  `json:"queue_depth"`
	QueueCapacity int64  `json:"queue_capacity"`
	DroppedCount  uint64 `json:"dropped_count"`
	WriteFailed   uint64 `json:"write_failed_count"`
	WrittenCount  uint64 `json:"written_count"`
}

// OpsRequestPerformanceSink isolates optional diagnostics persistence from the
// request and usage-recording paths. Queue saturation drops telemetry only.
type OpsRequestPerformanceSink struct {
	repo opsRequestPerformanceBatchRepository

	queue         chan *OpsRequestPerformanceInput
	batchSize     int
	flushInterval time.Duration
	drainTimeout  time.Duration

	ctx       context.Context
	cancel    context.CancelFunc
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	acceptMu  sync.RWMutex
	accepting bool

	droppedCount atomic.Uint64
	writeFailed  atomic.Uint64
	writtenCount atomic.Uint64
}

func NewOpsRequestPerformanceSink(repo opsRequestPerformanceBatchRepository) *OpsRequestPerformanceSink {
	ctx, cancel := context.WithCancel(context.Background())
	return &OpsRequestPerformanceSink{
		repo:          repo,
		queue:         make(chan *OpsRequestPerformanceInput, opsPerformanceQueueCapacity),
		batchSize:     opsPerformanceBatchSize,
		flushInterval: opsPerformanceFlushInterval,
		drainTimeout:  opsPerformanceDrainTimeout,
		ctx:           ctx,
		cancel:        cancel,
		accepting:     true,
	}
}

func (s *OpsRequestPerformanceSink) Start() {
	if s == nil || s.repo == nil {
		return
	}
	s.startOnce.Do(func() {
		s.acceptMu.Lock()
		defer s.acceptMu.Unlock()
		if !s.accepting {
			return
		}
		s.wg.Add(1)
		go s.run()
	})
}

func (s *OpsRequestPerformanceSink) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		s.acceptMu.Lock()
		s.accepting = false
		if s.cancel != nil {
			s.cancel()
		}
		s.acceptMu.Unlock()
		s.wg.Wait()
	})
}

func (s *OpsRequestPerformanceSink) Enqueue(input *OpsRequestPerformanceInput) bool {
	if s == nil || input == nil || s.queue == nil || s.repo == nil {
		return false
	}
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if !s.accepting {
		return false
	}

	cloned := *input
	if input.GroupID != nil {
		groupID := *input.GroupID
		cloned.GroupID = &groupID
	}
	select {
	case s.queue <- &cloned:
		return true
	default:
		s.droppedCount.Add(1)
		return false
	}
}

func (s *OpsRequestPerformanceSink) run() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	batch := make([]*OpsRequestPerformanceInput, 0, s.batchSize)
	flush := func(baseCtx context.Context) {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(baseCtx, opsPerformanceWriteTimeout)
		inserted, err := s.repo.BatchInsertRequestPerformance(ctx, batch)
		cancel()
		if err != nil {
			s.writeFailed.Add(uint64(len(batch)))
			log.Printf("[Ops] request performance sink flush failed: batch=%d err=%v", len(batch), err)
		} else {
			s.writtenCount.Add(uint64(inserted))
		}
		batch = batch[:0]
	}
	dropBuffered := func() {
		dropped := len(batch)
		batch = batch[:0]
		for {
			select {
			case <-s.queue:
				dropped++
			default:
				if dropped > 0 {
					s.droppedCount.Add(uint64(dropped))
				}
				return
			}
		}
	}
	drain := func() {
		drainTimeout := s.drainTimeout
		if drainTimeout <= 0 {
			drainTimeout = opsPerformanceDrainTimeout
		}
		drainCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		defer cancel()
		for {
			if drainCtx.Err() != nil {
				dropBuffered()
				return
			}
			select {
			case input := <-s.queue:
				if input != nil {
					batch = append(batch, input)
				}
				if len(batch) >= s.batchSize {
					flush(drainCtx)
				}
			default:
				flush(drainCtx)
				return
			}
		}
	}

	for {
		select {
		case <-s.ctx.Done():
			drain()
			return
		case input := <-s.queue:
			if input != nil {
				batch = append(batch, input)
			}
			if len(batch) >= s.batchSize {
				flush(s.ctx)
			}
		case <-ticker.C:
			flush(s.ctx)
		}
	}
}

func (s *OpsRequestPerformanceSink) Health() OpsRequestPerformanceSinkHealth {
	if s == nil {
		return OpsRequestPerformanceSinkHealth{}
	}
	return OpsRequestPerformanceSinkHealth{
		QueueDepth:    int64(len(s.queue)),
		QueueCapacity: int64(cap(s.queue)),
		DroppedCount:  s.droppedCount.Load(),
		WriteFailed:   s.writeFailed.Load(),
		WrittenCount:  s.writtenCount.Load(),
	}
}
