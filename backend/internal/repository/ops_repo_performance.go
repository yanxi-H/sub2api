package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"golang.org/x/sync/errgroup"
)

func (r *opsRepository) InsertRequestPerformance(ctx context.Context, input *service.OpsRequestPerformanceInput) error {
	if input == nil {
		return nil
	}
	_, err := r.BatchInsertRequestPerformance(ctx, []*service.OpsRequestPerformanceInput{input})
	return err
}

func (r *opsRepository) BatchInsertRequestPerformance(ctx context.Context, inputs []*service.OpsRequestPerformanceInput) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("nil ops repository")
	}
	if len(inputs) == 0 {
		return 0, nil
	}

	const columnCount = 27
	var query strings.Builder
	query.WriteString(`INSERT INTO ops_request_performance (
  created_at, request_id, user_id, api_key_id, account_id, group_id,
  platform, model, stream, request_body_lane, request_body_bytes, logical_status_code,
  end_to_end_ms, body_read_ms, user_queue_ms, body_lane_wait_ms,
  account_queue_ms, routing_ms, upstream_ms, time_to_first_token_ms,
  stream_duration_ms, max_stream_gap_ms, failover_ms, attempt_count,
  account_switch_count, failure_cause, slow_cause
) VALUES `)
	args := make([]any, 0, len(inputs)*columnCount)
	rowCount := 0
	for _, input := range inputs {
		if input == nil || strings.TrimSpace(input.RequestID) == "" {
			continue
		}
		if rowCount > 0 {
			query.WriteByte(',')
		}
		query.WriteByte('(')
		for column := 0; column < columnCount; column++ {
			if column > 0 {
				query.WriteByte(',')
			}
			query.WriteByte('$')
			query.WriteString(strconv.Itoa(len(args) + column + 1))
		}
		query.WriteByte(')')
		args = append(args,
			input.CreatedAt, strings.TrimSpace(input.RequestID), input.UserID, input.APIKeyID, input.AccountID, input.GroupID,
			input.Platform, input.Model, input.Stream, string(input.RequestBodyLane), input.RequestBodyBytes, input.LogicalStatusCode,
			input.EndToEndMs, input.BodyReadMs, input.UserQueueMs, input.BodyLaneWaitMs,
			input.AccountQueueMs, input.RoutingMs, input.UpstreamMs, input.TimeToFirstTokenMs,
			input.StreamDurationMs, input.MaxStreamGapMs, input.FailoverMs, input.AttemptCount,
			input.AccountSwitchCount, input.FailureCause, input.SlowCause,
		)
		rowCount++
	}
	if rowCount == 0 {
		return 0, nil
	}
	query.WriteString(`
ON CONFLICT (request_id, api_key_id) DO UPDATE SET
  created_at = EXCLUDED.created_at,
  user_id = EXCLUDED.user_id,
  account_id = EXCLUDED.account_id,
  group_id = EXCLUDED.group_id,
  platform = EXCLUDED.platform,
  model = EXCLUDED.model,
  stream = EXCLUDED.stream,
  request_body_lane = EXCLUDED.request_body_lane,
  request_body_bytes = EXCLUDED.request_body_bytes,
  logical_status_code = EXCLUDED.logical_status_code,
  end_to_end_ms = EXCLUDED.end_to_end_ms,
  body_read_ms = EXCLUDED.body_read_ms,
  user_queue_ms = EXCLUDED.user_queue_ms,
  body_lane_wait_ms = EXCLUDED.body_lane_wait_ms,
  account_queue_ms = EXCLUDED.account_queue_ms,
  routing_ms = EXCLUDED.routing_ms,
  upstream_ms = EXCLUDED.upstream_ms,
  time_to_first_token_ms = EXCLUDED.time_to_first_token_ms,
  stream_duration_ms = EXCLUDED.stream_duration_ms,
  max_stream_gap_ms = EXCLUDED.max_stream_gap_ms,
  failover_ms = EXCLUDED.failover_ms,
  attempt_count = EXCLUDED.attempt_count,
  account_switch_count = EXCLUDED.account_switch_count,
  failure_cause = EXCLUDED.failure_cause,
  slow_cause = EXCLUDED.slow_cause`)
	result, err := r.db.ExecContext(ctx, query.String(), args...)
	if err != nil {
		return 0, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return int64(rowCount), nil
	}
	return affected, nil
}

func (r *opsRepository) GetPerformanceDiagnostics(ctx context.Context, filter *service.OpsDashboardFilter, bucketSeconds int) (*service.OpsPerformanceDiagnosticsResponse, error) {
	if r == nil || r.db == nil || filter == nil {
		return nil, fmt.Errorf("invalid performance diagnostics query")
	}
	if bucketSeconds <= 0 {
		bucketSeconds = 300
	}
	where, args := performanceWhere(filter)
	response := &service.OpsPerformanceDiagnosticsResponse{
		StartTime: filter.StartTime,
		EndTime:   filter.EndTime,
		Bucket:    time.Duration(bucketSeconds * int(time.Second)).String(),
		Causes:    []service.OpsSlowCauseSummary{},
		Trend:     []service.OpsSlowCauseTrendPoint{},
		Impacts:   []service.OpsPerformanceImpact{},
	}
	group, queryCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return r.scanPerformanceSummary(queryCtx, where, args, &response.Summary)
	})
	group.Go(func() error {
		causes, err := r.queryPerformanceCauses(queryCtx, where, args)
		if err == nil {
			response.Causes = causes
		}
		return err
	})
	group.Go(func() error {
		trend, err := r.queryPerformanceTrend(queryCtx, where, args, filter.StartTime, filter.EndTime, bucketSeconds)
		if err == nil {
			response.Trend = trend
		}
		return err
	})
	group.Go(func() error {
		impacts, err := r.queryPerformanceImpacts(queryCtx, where, args)
		if err == nil {
			response.Impacts = impacts
		}
		return err
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}
	return response, nil
}

func performanceWhere(filter *service.OpsDashboardFilter) (string, []any) {
	conditions := []string{"p.created_at >= $1", "p.created_at < $2"}
	args := []any{filter.StartTime, filter.EndTime}
	if platform := strings.TrimSpace(filter.Platform); platform != "" {
		args = append(args, platform)
		conditions = append(conditions, fmt.Sprintf("p.platform = $%d", len(args)))
	}
	if filter.GroupID != nil {
		args = append(args, *filter.GroupID)
		conditions = append(conditions, fmt.Sprintf("p.group_id = $%d", len(args)))
	}
	return strings.Join(conditions, " AND "), args
}

func (r *opsRepository) scanPerformanceSummary(ctx context.Context, where string, args []any, out *service.OpsPerformanceSummary) error {
	var slowRate sql.NullFloat64
	values := []*sql.NullFloat64{{}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}, {}}
	query := `SELECT COUNT(*), COUNT(*) FILTER (WHERE slow_cause <> 'healthy'),
COALESCE(100.0 * COUNT(*) FILTER (WHERE slow_cause <> 'healthy') / NULLIF(COUNT(*), 0), 0),
percentile_cont(0.50) WITHIN GROUP (ORDER BY end_to_end_ms),
percentile_cont(0.90) WITHIN GROUP (ORDER BY end_to_end_ms),
percentile_cont(0.95) WITHIN GROUP (ORDER BY end_to_end_ms),
percentile_cont(0.99) WITHIN GROUP (ORDER BY end_to_end_ms), AVG(end_to_end_ms), MAX(end_to_end_ms),
percentile_cont(0.50) WITHIN GROUP (ORDER BY time_to_first_token_ms) FILTER (WHERE time_to_first_token_ms > 0),
percentile_cont(0.90) WITHIN GROUP (ORDER BY time_to_first_token_ms) FILTER (WHERE time_to_first_token_ms > 0),
percentile_cont(0.95) WITHIN GROUP (ORDER BY time_to_first_token_ms) FILTER (WHERE time_to_first_token_ms > 0),
percentile_cont(0.99) WITHIN GROUP (ORDER BY time_to_first_token_ms) FILTER (WHERE time_to_first_token_ms > 0),
AVG(time_to_first_token_ms) FILTER (WHERE time_to_first_token_ms > 0), MAX(time_to_first_token_ms) FILTER (WHERE time_to_first_token_ms > 0)
FROM ops_request_performance p WHERE ` + where
	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&out.SampleCount, &out.SlowCount, &slowRate,
		values[0], values[1], values[2], values[3], values[4], values[5],
		values[6], values[7], values[8], values[9], values[10], values[11],
	)
	if err != nil {
		return err
	}
	out.SlowRate = slowRate.Float64
	out.EndToEnd = performancePercentiles(values[:6])
	out.TTFT = performancePercentiles(values[6:])
	return nil
}

func performancePercentiles(values []*sql.NullFloat64) service.OpsPercentiles {
	toInt := func(value sql.NullFloat64) *int {
		if !value.Valid {
			return nil
		}
		result := int(value.Float64)
		return &result
	}
	return service.OpsPercentiles{P50: toInt(*values[0]), P90: toInt(*values[1]), P95: toInt(*values[2]), P99: toInt(*values[3]), Avg: toInt(*values[4]), Max: toInt(*values[5])}
}

func (r *opsRepository) queryPerformanceCauses(ctx context.Context, where string, args []any) ([]service.OpsSlowCauseSummary, error) {
	query := `SELECT slow_cause, COUNT(*),
100.0 * COUNT(*) / NULLIF(SUM(COUNT(*)) OVER (), 0),
percentile_cont(0.95) WITHIN GROUP (ORDER BY end_to_end_ms),
percentile_cont(0.95) WITHIN GROUP (ORDER BY GREATEST(user_queue_ms, body_lane_wait_ms, account_queue_ms)),
percentile_cont(0.95) WITHIN GROUP (ORDER BY time_to_first_token_ms) FILTER (WHERE time_to_first_token_ms > 0)
FROM ops_request_performance p WHERE ` + where + ` AND slow_cause <> 'healthy'
GROUP BY slow_cause ORDER BY COUNT(*) DESC, slow_cause LIMIT 12`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	result := make([]service.OpsSlowCauseSummary, 0)
	for rows.Next() {
		var item service.OpsSlowCauseSummary
		var e2e, queue, ttft sql.NullFloat64
		if err := rows.Scan(&item.Cause, &item.Count, &item.Share, &e2e, &queue, &ttft); err != nil {
			_ = rows.Close()
			return nil, err
		}
		item.E2EP95Ms = nullFloatToInt(e2e)
		item.QueueP95Ms = nullFloatToInt(queue)
		item.TTFTP95Ms = nullFloatToInt(ttft)
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *opsRepository) queryPerformanceTrend(ctx context.Context, where string, args []any, startTime, endTime time.Time, bucketSeconds int) ([]service.OpsSlowCauseTrendPoint, error) {
	args = append(args, bucketSeconds)
	bucketArg := "$" + strconv.Itoa(len(args))
	query := `SELECT date_bin(make_interval(secs => ` + bucketArg + `), p.created_at, TIMESTAMPTZ '1970-01-01'), slow_cause, COUNT(*)
FROM ops_request_performance p WHERE ` + where + ` AND slow_cause <> 'healthy'
GROUP BY 1, 2 ORDER BY 1, 2`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	bucketDuration := time.Duration(bucketSeconds) * time.Second
	firstBucket := time.Unix((startTime.Unix()/int64(bucketSeconds))*int64(bucketSeconds), 0).UTC()
	byTime := make(map[time.Time]*service.OpsSlowCauseTrendPoint)
	order := make([]time.Time, 0, int(endTime.Sub(firstBucket)/bucketDuration)+1)
	for bucket := firstBucket; bucket.Before(endTime); bucket = bucket.Add(bucketDuration) {
		byTime[bucket] = &service.OpsSlowCauseTrendPoint{BucketStart: bucket, Causes: map[string]int64{}}
		order = append(order, bucket)
	}
	for rows.Next() {
		var bucket time.Time
		var cause string
		var count int64
		if err := rows.Scan(&bucket, &cause, &count); err != nil {
			_ = rows.Close()
			return nil, err
		}
		bucket = bucket.UTC()
		point := byTime[bucket]
		if point == nil {
			point = &service.OpsSlowCauseTrendPoint{BucketStart: bucket, Causes: map[string]int64{}}
			byTime[bucket] = point
			order = append(order, bucket)
		}
		point.Causes[cause] = count
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	result := make([]service.OpsSlowCauseTrendPoint, 0, len(order))
	for _, bucket := range order {
		result = append(result, *byTime[bucket])
	}
	return result, nil
}

func (r *opsRepository) queryPerformanceImpacts(ctx context.Context, where string, args []any) ([]service.OpsPerformanceImpact, error) {
	dimensions := []struct {
		name  string
		id    string
		label string
		join  string
	}{
		{name: "user", id: "p.user_id::text", label: "COALESCE(NULLIF(u.username, ''), NULLIF(u.email, ''), '#' || p.user_id::text)", join: "LEFT JOIN users u ON u.id = p.user_id"},
		{name: "account", id: "p.account_id::text", label: "COALESCE(NULLIF(a.name, ''), '#' || p.account_id::text)", join: "LEFT JOIN accounts a ON a.id = p.account_id"},
		{name: "model", id: "p.model", label: "p.model", join: ""},
	}
	result := make([]service.OpsPerformanceImpact, 0)
	for _, dimension := range dimensions {
		query := `SELECT ` + dimension.id + `, ` + dimension.label + `, COUNT(*),
COALESCE(100.0 * COUNT(*) FILTER (WHERE p.slow_cause <> 'healthy') / NULLIF(COUNT(*), 0), 0),
	percentile_cont(0.95) WITHIN GROUP (ORDER BY p.end_to_end_ms),
	percentile_cont(0.95) WITHIN GROUP (ORDER BY p.time_to_first_token_ms) FILTER (WHERE p.time_to_first_token_ms > 0),
	percentile_cont(0.95) WITHIN GROUP (ORDER BY GREATEST(p.user_queue_ms, p.body_lane_wait_ms, p.account_queue_ms)),
	COALESCE(mode() WITHIN GROUP (ORDER BY NULLIF(p.slow_cause, 'healthy')), 'healthy')
FROM ops_request_performance p ` + dimension.join + ` WHERE ` + where + ` GROUP BY ` + dimension.id + `, ` + dimension.label + `
ORDER BY COUNT(*) FILTER (WHERE p.slow_cause <> 'healthy') DESC, COUNT(*) DESC`
		rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			item := service.OpsPerformanceImpact{Dimension: dimension.name}
			var e2e, ttft, queue sql.NullFloat64
			if err := rows.Scan(&item.ID, &item.Name, &item.RequestCount, &item.SlowRate, &e2e, &ttft, &queue, &item.MainCause); err != nil {
				_ = rows.Close()
				return nil, err
			}
			item.E2EP95Ms = nullFloatToInt(e2e)
			item.TTFTP95Ms = nullFloatToInt(ttft)
			item.QueueP95Ms = nullFloatToInt(queue)
			result = append(result, item)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func nullFloatToInt(value sql.NullFloat64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Float64)
	return &result
}
