package service

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	userConcurrencyTrendFlushInterval = time.Second
	userConcurrencyTrendSampleTimeout = 3 * time.Second
	userConcurrencyTrendEventBuffer   = 4096
)

// ConcurrencyPeak is the peak concurrency observed within one minute.
type ConcurrencyPeak struct {
	PeakInUse   int `json:"peak_in_use"`
	PeakWaiting int `json:"peak_waiting"`
	PeakDemand  int `json:"peak_demand"`
}

type ConcurrencySnapshot struct {
	InUse   int `json:"in_use"`
	Waiting int `json:"waiting"`
	Demand  int `json:"demand"`
}

type ConcurrencyLanePeaks struct {
	Normal   ConcurrencyPeak `json:"normal"`
	Heavy    ConcurrencyPeak `json:"heavy"`
	Recovery ConcurrencyPeak `json:"recovery"`
}

type ConcurrencyLaneSnapshots struct {
	Normal   ConcurrencySnapshot `json:"normal"`
	Heavy    ConcurrencySnapshot `json:"heavy"`
	Recovery ConcurrencySnapshot `json:"recovery"`
}

type UserConcurrencyTrendPoint struct {
	BucketStart time.Time                      `json:"bucket_start"`
	System      ConcurrencyPeak                `json:"system"`
	Users       map[int64]ConcurrencyPeak      `json:"users"`
	SystemLanes ConcurrencyLanePeaks           `json:"system_lanes"`
	UserLanes   map[int64]ConcurrencyLanePeaks `json:"user_lanes"`
}

type UserConcurrencyTrend struct {
	StartTime        time.Time                   `json:"start_time"`
	EndTime          time.Time                   `json:"end_time"`
	CoverageStart    *time.Time                  `json:"coverage_start,omitempty"`
	CoverageEnd      *time.Time                  `json:"coverage_end,omitempty"`
	CoverageComplete bool                        `json:"coverage_complete"`
	Bucket           string                      `json:"bucket"`
	Points           []UserConcurrencyTrendPoint `json:"points"`
}

// UserConcurrencyTrendCache is implemented by Redis-backed concurrency caches.
// It is optional so lightweight test stubs and alternative caches remain compatible.
type UserConcurrencyTrendCache interface {
	GetActiveUserLoads(ctx context.Context) (map[int64]*UserLoadInfo, error)
	GetActiveRequestBodyLaneLoads(ctx context.Context) (map[int64]RequestBodyLaneUserLoad, error)
	MergeUserConcurrencyTrend(ctx context.Context, bucketStart time.Time, users map[int64]ConcurrencyPeak, system ConcurrencyPeak, userLanes map[int64]ConcurrencyLanePeaks, systemLanes ConcurrencyLanePeaks) error
	GetUserConcurrencyTrend(ctx context.Context, start, end time.Time) (*UserConcurrencyTrend, error)
}

type userConcurrencyStateCache interface {
	TrackUserSlotWithState(ctx context.Context, userID int64, requestID string) (int, time.Time, error)
	AcquireUserSlotWithState(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, int, time.Time, error)
	ReleaseUserSlotWithState(ctx context.Context, userID int64, requestID string) (int, time.Time, error)
	IncrementWaitCountWithState(ctx context.Context, userID int64, maxWait int) (bool, int, time.Time, error)
	DecrementWaitCountWithState(ctx context.Context, userID int64) (int, time.Time, error)
}

type userConcurrencyStateEvent struct {
	userID          int64
	active          *int
	waiting         *int
	requestBodyLoad *RequestBodyLaneUserLoad
	at              time.Time
}

type userConcurrencyLiveState struct {
	active          int
	waiting         int
	requestBodyLoad RequestBodyLaneUserLoad
}

type userConcurrencyTrendRecorder struct {
	cache UserConcurrencyTrendCache

	events   chan userConcurrencyStateEvent
	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
	dropped  atomic.Uint64
}

func newUserConcurrencyTrendRecorder(cache ConcurrencyCache) *userConcurrencyTrendRecorder {
	trendCache, ok := cache.(UserConcurrencyTrendCache)
	if !ok || trendCache == nil {
		return nil
	}
	r := &userConcurrencyTrendRecorder{
		cache:  trendCache,
		events: make(chan userConcurrencyStateEvent, userConcurrencyTrendEventBuffer),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	go r.run()
	return r
}

func (r *userConcurrencyTrendRecorder) observe(event userConcurrencyStateEvent) {
	if r == nil || event.userID <= 0 {
		return
	}
	if event.at.IsZero() {
		event.at = time.Now().UTC()
	}
	select {
	case r.events <- event:
	default:
		r.dropped.Add(1)
	}
}

func (r *userConcurrencyTrendRecorder) stop() {
	if r == nil {
		return
	}
	r.stopOnce.Do(func() { close(r.stopCh) })
	<-r.doneCh
}

func (r *userConcurrencyTrendRecorder) run() {
	defer close(r.doneCh)

	live := make(map[int64]userConcurrencyLiveState)
	pending := make(map[time.Time]map[int64]ConcurrencyPeak)
	systemPending := make(map[time.Time]ConcurrencyPeak)
	userLanesPending := make(map[time.Time]map[int64]ConcurrencyLanePeaks)
	systemLanesPending := make(map[time.Time]ConcurrencyLanePeaks)
	totalActive := 0
	totalWaiting := 0
	totalLanes := ConcurrencyLaneSnapshots{}

	flushTicker := time.NewTicker(userConcurrencyTrendFlushInterval)
	defer flushTicker.Stop()

	reconcile := func(now time.Time) {
		ctx, cancel := context.WithTimeout(context.Background(), userConcurrencyTrendSampleTimeout)
		loads, err := r.cache.GetActiveUserLoads(ctx)
		if err != nil {
			cancel()
			log.Printf("[ConcurrencyTrend] reconcile active users failed: %v", err)
			return
		}
		bodyLoads, err := r.cache.GetActiveRequestBodyLaneLoads(ctx)
		cancel()
		if err != nil {
			log.Printf("[ConcurrencyTrend] reconcile request body lanes failed: %v", err)
			return
		}

		live = make(map[int64]userConcurrencyLiveState, max(len(loads), len(bodyLoads)))
		totalActive = 0
		totalWaiting = 0
		totalLanes = ConcurrencyLaneSnapshots{}
		for userID, load := range loads {
			if load == nil || (load.CurrentConcurrency <= 0 && load.WaitingCount <= 0) {
				continue
			}
			state := userConcurrencyLiveState{
				active:  max(load.CurrentConcurrency, 0),
				waiting: max(load.WaitingCount, 0),
			}
			live[userID] = state
			totalActive += state.active
			totalWaiting += state.waiting
		}
		for userID, bodyLoad := range bodyLoads {
			state := live[userID]
			state.requestBodyLoad = bodyLoad
			live[userID] = state
		}
		for _, state := range live {
			addConcurrencyLaneSnapshots(&totalLanes, concurrencyLaneSnapshotsForState(state), 1)
		}
		recordConcurrencyTrendSample(pending, systemPending, userLanesPending, systemLanesPending, now, live, 0, totalActive, totalWaiting, totalLanes)
	}

	flush := func() {
		for bucket, users := range pending {
			ctx, cancel := context.WithTimeout(context.Background(), userConcurrencyTrendSampleTimeout)
			err := r.cache.MergeUserConcurrencyTrend(ctx, bucket, users, systemPending[bucket], userLanesPending[bucket], systemLanesPending[bucket])
			cancel()
			if err != nil {
				log.Printf("[ConcurrencyTrend] flush bucket %s failed: %v", bucket.Format(time.RFC3339), err)
				continue
			}
			delete(pending, bucket)
			delete(systemPending, bucket)
			delete(userLanesPending, bucket)
			delete(systemLanesPending, bucket)
		}
	}

	reconcile(time.Now().UTC())
	for {
		select {
		case event := <-r.events:
			state := live[event.userID]
			previousLanes := concurrencyLaneSnapshotsForState(state)
			if event.active != nil {
				next := max(*event.active, 0)
				totalActive += next - state.active
				state.active = next
			}
			if event.waiting != nil {
				next := max(*event.waiting, 0)
				totalWaiting += next - state.waiting
				state.waiting = next
			}
			if event.requestBodyLoad != nil {
				state.requestBodyLoad = *event.requestBodyLoad
			}
			nextLanes := concurrencyLaneSnapshotsForState(state)
			addConcurrencyLaneSnapshots(&totalLanes, previousLanes, -1)
			addConcurrencyLaneSnapshots(&totalLanes, nextLanes, 1)
			if state.active == 0 && state.waiting == 0 && !requestBodyLaneUserLoadHasActivity(state.requestBodyLoad) {
				delete(live, event.userID)
			} else {
				live[event.userID] = state
			}
			recordConcurrencyTrendSample(pending, systemPending, userLanesPending, systemLanesPending, event.at, live, event.userID, totalActive, totalWaiting, totalLanes)
		case now := <-flushTicker.C:
			reconcile(now.UTC())
			flush()
			if dropped := r.dropped.Swap(0); dropped > 0 {
				log.Printf("[ConcurrencyTrend] dropped %d realtime events; one-second reconciliation applied", dropped)
			}
		case <-r.stopCh:
			reconcile(time.Now().UTC())
			flush()
			return
		}
	}
}

func recordConcurrencyTrendSample(
	pending map[time.Time]map[int64]ConcurrencyPeak,
	systemPending map[time.Time]ConcurrencyPeak,
	userLanesPending map[time.Time]map[int64]ConcurrencyLanePeaks,
	systemLanesPending map[time.Time]ConcurrencyLanePeaks,
	at time.Time,
	live map[int64]userConcurrencyLiveState,
	changedUserID int64,
	totalActive int,
	totalWaiting int,
	totalLanes ConcurrencyLaneSnapshots,
) {
	bucket := at.UTC().Truncate(time.Minute)
	users := pending[bucket]
	if users == nil {
		users = make(map[int64]ConcurrencyPeak)
		pending[bucket] = users
	}
	userLanes := userLanesPending[bucket]
	if userLanes == nil {
		userLanes = make(map[int64]ConcurrencyLanePeaks)
		userLanesPending[bucket] = userLanes
	}
	recordUser := func(userID int64, state userConcurrencyLiveState) {
		lanes := concurrencyLaneSnapshotsForState(state)
		if state.active <= 0 && state.waiting <= 0 && concurrencyLaneSnapshotsDemand(lanes) <= 0 {
			return
		}
		peak := users[userID]
		peak.PeakInUse = max(peak.PeakInUse, state.active)
		peak.PeakWaiting = max(peak.PeakWaiting, state.waiting)
		peak.PeakDemand = max(peak.PeakDemand, state.active+state.waiting)
		users[userID] = peak
		userLanes[userID] = mergeConcurrencyLanePeaks(userLanes[userID], lanes)
	}
	if changedUserID > 0 {
		if state, ok := live[changedUserID]; ok {
			recordUser(changedUserID, state)
		}
	} else {
		for userID, state := range live {
			recordUser(userID, state)
		}
	}

	system := systemPending[bucket]
	system.PeakInUse = max(system.PeakInUse, totalActive)
	system.PeakWaiting = max(system.PeakWaiting, totalWaiting)
	system.PeakDemand = max(system.PeakDemand, totalActive+totalWaiting)
	systemPending[bucket] = system
	systemLanesPending[bucket] = mergeConcurrencyLanePeaks(systemLanesPending[bucket], totalLanes)
}

func requestBodyLaneUserLoadHasActivity(load RequestBodyLaneUserLoad) bool {
	return load.HeavyActive > 0 || load.HeavyWaiting > 0 || load.RecoveryActive > 0 || load.RecoveryWaiting > 0 ||
		load.PendingActive > 0 || load.PendingWaiting > 0
}

func concurrencyLaneSnapshotsForState(state userConcurrencyLiveState) ConcurrencyLaneSnapshots {
	// OpenAI Responses requests reserve their base user demand while account
	// policy classification is pending and throughout heavy/recovery handling.
	// This prevents acquisition and failover transitions from appearing as
	// short-lived normal traffic.
	classifiedActive := max(
		max(state.requestBodyLoad.PendingActive, 0),
		max(state.requestBodyLoad.HeavyActive, 0)+max(state.requestBodyLoad.HeavyWaiting, 0)+
			max(state.requestBodyLoad.RecoveryActive, 0)+max(state.requestBodyLoad.RecoveryWaiting, 0),
	)
	normalActive := max(state.active-classifiedActive, 0)
	normalWaiting := max(state.waiting-max(state.requestBodyLoad.PendingWaiting, 0), 0)
	heavyActive := max(state.requestBodyLoad.HeavyActive, 0)
	heavyWaiting := max(state.requestBodyLoad.HeavyWaiting, 0)
	recoveryActive := max(state.requestBodyLoad.RecoveryActive, 0)
	recoveryWaiting := max(state.requestBodyLoad.RecoveryWaiting, 0)
	return ConcurrencyLaneSnapshots{
		Normal:   ConcurrencySnapshot{InUse: normalActive, Waiting: normalWaiting, Demand: normalActive + normalWaiting},
		Heavy:    ConcurrencySnapshot{InUse: heavyActive, Waiting: heavyWaiting, Demand: heavyActive + heavyWaiting},
		Recovery: ConcurrencySnapshot{InUse: recoveryActive, Waiting: recoveryWaiting, Demand: recoveryActive + recoveryWaiting},
	}
}

func addConcurrencyLaneSnapshots(total *ConcurrencyLaneSnapshots, value ConcurrencyLaneSnapshots, multiplier int) {
	if total == nil {
		return
	}
	add := func(target *ConcurrencySnapshot, source ConcurrencySnapshot) {
		target.InUse = max(target.InUse+source.InUse*multiplier, 0)
		target.Waiting = max(target.Waiting+source.Waiting*multiplier, 0)
		target.Demand = target.InUse + target.Waiting
	}
	add(&total.Normal, value.Normal)
	add(&total.Heavy, value.Heavy)
	add(&total.Recovery, value.Recovery)
}

func concurrencyLaneSnapshotsDemand(value ConcurrencyLaneSnapshots) int {
	return value.Normal.Demand + value.Heavy.Demand + value.Recovery.Demand
}

func mergeConcurrencyLanePeaks(current ConcurrencyLanePeaks, sample ConcurrencyLaneSnapshots) ConcurrencyLanePeaks {
	merge := func(peak ConcurrencyPeak, snapshot ConcurrencySnapshot) ConcurrencyPeak {
		peak.PeakInUse = max(peak.PeakInUse, snapshot.InUse)
		peak.PeakWaiting = max(peak.PeakWaiting, snapshot.Waiting)
		peak.PeakDemand = max(peak.PeakDemand, snapshot.Demand)
		return peak
	}
	current.Normal = merge(current.Normal, sample.Normal)
	current.Heavy = merge(current.Heavy, sample.Heavy)
	current.Recovery = merge(current.Recovery, sample.Recovery)
	return current
}

func (s *ConcurrencyService) observeUserConcurrencyState(userID int64, active, waiting *int, at time.Time) {
	s.observeUserConcurrencyEvent(userID, active, waiting, nil, at)
}

func (s *ConcurrencyService) observeUserConcurrencyEvent(
	userID int64,
	active, waiting *int,
	requestBodyLoad *RequestBodyLaneUserLoad,
	at time.Time,
) {
	if s == nil || s.trendRecorder == nil {
		return
	}
	s.trendRecorder.observe(userConcurrencyStateEvent{
		userID:          userID,
		active:          active,
		waiting:         waiting,
		requestBodyLoad: requestBodyLoad,
		at:              at,
	})
}

func (s *ConcurrencyService) observeRequestBodyLaneState(userID int64, load RequestBodyLaneUserLoad, at time.Time) {
	if s == nil || s.trendRecorder == nil {
		return
	}
	s.trendRecorder.observe(userConcurrencyStateEvent{userID: userID, requestBodyLoad: &load, at: at})
}

func (s *ConcurrencyService) GetUserConcurrencyTrend(ctx context.Context, start, end time.Time) (*UserConcurrencyTrend, error) {
	now := time.Now().UTC()
	if end.IsZero() || end.After(now) {
		end = now
	}
	end = end.UTC().Truncate(time.Minute)
	if start.IsZero() {
		start = end.Add(-59 * time.Minute)
	}
	start = start.UTC().Truncate(time.Minute)
	if end.Before(start) {
		end = start
	}
	requestedStart := start
	requestedEnd := end
	bucket := time.Minute
	if requestedEnd.Sub(requestedStart) > 6*time.Hour {
		bucket = 5 * time.Minute
	}
	retentionStart := now.Add(-24 * time.Hour).UTC().Truncate(time.Minute)
	if end.Before(retentionStart) {
		return &UserConcurrencyTrend{
			StartTime: requestedStart,
			EndTime:   requestedEnd,
			Bucket:    concurrencyTrendBucketName(bucket),
			Points:    []UserConcurrencyTrendPoint{},
		}, nil
	}
	coverageComplete := !start.Before(retentionStart)
	if start.Before(retentionStart) {
		start = retentionStart
	}
	coverageStart := start
	coverageEnd := end
	if s == nil || s.trendRecorder == nil || s.trendRecorder.cache == nil {
		points := make([]UserConcurrencyTrendPoint, 0, int(end.Sub(start)/bucket)+1)
		for pointAt := start.Truncate(bucket); !pointAt.After(end); pointAt = pointAt.Add(bucket) {
			points = append(points, UserConcurrencyTrendPoint{
				BucketStart: pointAt,
				Users:       map[int64]ConcurrencyPeak{},
				UserLanes:   map[int64]ConcurrencyLanePeaks{},
			})
		}
		return &UserConcurrencyTrend{
			StartTime:        requestedStart,
			EndTime:          requestedEnd,
			CoverageStart:    &coverageStart,
			CoverageEnd:      &coverageEnd,
			CoverageComplete: coverageComplete,
			Bucket:           concurrencyTrendBucketName(bucket),
			Points:           points,
		}, nil
	}
	trend, err := s.trendRecorder.cache.GetUserConcurrencyTrend(ctx, start, end)
	if err != nil {
		return trend, err
	}
	trend.StartTime = requestedStart
	trend.EndTime = requestedEnd
	trend.CoverageStart = &coverageStart
	trend.CoverageEnd = &coverageEnd
	trend.CoverageComplete = coverageComplete
	if bucket == time.Minute {
		return trend, nil
	}
	return aggregateUserConcurrencyTrend(trend, bucket), nil
}

func concurrencyTrendBucketName(bucket time.Duration) string {
	if bucket == 5*time.Minute {
		return "5m"
	}
	return "minute"
}

func aggregateUserConcurrencyTrend(trend *UserConcurrencyTrend, bucket time.Duration) *UserConcurrencyTrend {
	if trend == nil || bucket <= time.Minute {
		return trend
	}
	byBucket := make(map[time.Time]*UserConcurrencyTrendPoint)
	order := make([]time.Time, 0, len(trend.Points))
	for _, source := range trend.Points {
		bucketStart := source.BucketStart.UTC().Truncate(bucket)
		target := byBucket[bucketStart]
		if target == nil {
			target = &UserConcurrencyTrendPoint{
				BucketStart: bucketStart,
				Users:       make(map[int64]ConcurrencyPeak),
				UserLanes:   make(map[int64]ConcurrencyLanePeaks),
			}
			byBucket[bucketStart] = target
			order = append(order, bucketStart)
		}
		target.System = mergeConcurrencyPeak(target.System, source.System)
		target.SystemLanes = mergeConcurrencyLanePeakValues(target.SystemLanes, source.SystemLanes)
		for userID, peak := range source.Users {
			target.Users[userID] = mergeConcurrencyPeak(target.Users[userID], peak)
		}
		for userID, lanes := range source.UserLanes {
			target.UserLanes[userID] = mergeConcurrencyLanePeakValues(target.UserLanes[userID], lanes)
		}
	}
	points := make([]UserConcurrencyTrendPoint, 0, len(order))
	for _, bucketStart := range order {
		points = append(points, *byBucket[bucketStart])
	}
	return &UserConcurrencyTrend{
		StartTime:        trend.StartTime,
		EndTime:          trend.EndTime,
		CoverageStart:    trend.CoverageStart,
		CoverageEnd:      trend.CoverageEnd,
		CoverageComplete: trend.CoverageComplete,
		Bucket:           concurrencyTrendBucketName(bucket),
		Points:           points,
	}
}

func mergeConcurrencyPeak(current, sample ConcurrencyPeak) ConcurrencyPeak {
	current.PeakInUse = max(current.PeakInUse, sample.PeakInUse)
	current.PeakWaiting = max(current.PeakWaiting, sample.PeakWaiting)
	current.PeakDemand = max(current.PeakDemand, sample.PeakDemand)
	return current
}

func mergeConcurrencyLanePeakValues(current, sample ConcurrencyLanePeaks) ConcurrencyLanePeaks {
	current.Normal = mergeConcurrencyPeak(current.Normal, sample.Normal)
	current.Heavy = mergeConcurrencyPeak(current.Heavy, sample.Heavy)
	current.Recovery = mergeConcurrencyPeak(current.Recovery, sample.Recovery)
	return current
}

func (s *ConcurrencyService) GetCurrentUserConcurrencyLoads(ctx context.Context) (map[int64]*UserLoadInfo, error) {
	if s == nil || s.trendRecorder == nil || s.trendRecorder.cache == nil {
		return map[int64]*UserLoadInfo{}, nil
	}
	return s.trendRecorder.cache.GetActiveUserLoads(ctx)
}

func (s *ConcurrencyService) GetCurrentRequestBodyLaneLoads(ctx context.Context) (map[int64]RequestBodyLaneUserLoad, error) {
	if s == nil || s.trendRecorder == nil || s.trendRecorder.cache == nil {
		return map[int64]RequestBodyLaneUserLoad{}, nil
	}
	return s.trendRecorder.cache.GetActiveRequestBodyLaneLoads(ctx)
}
