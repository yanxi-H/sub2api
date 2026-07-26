package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *opsRepository) GetLatencyTrend(ctx context.Context, filter *service.OpsDashboardFilter, bucketSeconds int) (*service.OpsLatencyTrendResponse, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if filter == nil {
		return nil, fmt.Errorf("nil filter")
	}
	if filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return nil, fmt.Errorf("start_time/end_time required")
	}

	if bucketSeconds != 60 && bucketSeconds != 300 && bucketSeconds != 3600 {
		bucketSeconds = 60
	}
	start := filter.StartTime.UTC()
	end := filter.EndTime.UTC()
	where, args := performanceWhere(filter)
	bucketExpr := opsPerformanceBucketExpr(bucketSeconds)

	rows, err := r.db.QueryContext(ctx, `
SELECT
  `+bucketExpr+` AS bucket,
  percentile_cont(0.50) WITHIN GROUP (ORDER BY p.end_to_end_ms),
  percentile_cont(0.90) WITHIN GROUP (ORDER BY p.end_to_end_ms),
  percentile_cont(0.95) WITHIN GROUP (ORDER BY p.end_to_end_ms),
  AVG(p.end_to_end_ms),
  MAX(p.end_to_end_ms),
  COUNT(*),
  percentile_cont(0.50) WITHIN GROUP (ORDER BY p.time_to_first_token_ms) FILTER (WHERE p.time_to_first_token_ms > 0),
  percentile_cont(0.90) WITHIN GROUP (ORDER BY p.time_to_first_token_ms) FILTER (WHERE p.time_to_first_token_ms > 0),
  percentile_cont(0.95) WITHIN GROUP (ORDER BY p.time_to_first_token_ms) FILTER (WHERE p.time_to_first_token_ms > 0),
  AVG(p.time_to_first_token_ms) FILTER (WHERE p.time_to_first_token_ms > 0),
  MAX(p.time_to_first_token_ms) FILTER (WHERE p.time_to_first_token_ms > 0)
FROM ops_request_performance p
WHERE `+where+`
  AND p.logical_status_code >= 200
  AND p.logical_status_code < 400
GROUP BY 1
ORDER BY 1 ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	points := make([]*service.OpsLatencyTrendPoint, 0, 256)
	for rows.Next() {
		var bucket time.Time
		var p50, p90, p95, avg sql.NullFloat64
		var max sql.NullInt64
		var ttftP50, ttftP90, ttftP95, ttftAvg sql.NullFloat64
		var ttftMax sql.NullInt64
		var sampleCount int64
		if err := rows.Scan(
			&bucket, &p50, &p90, &p95, &avg, &max, &sampleCount,
			&ttftP50, &ttftP90, &ttftP95, &ttftAvg, &ttftMax,
		); err != nil {
			return nil, err
		}
		point := &service.OpsLatencyTrendPoint{
			BucketStart: bucket.UTC(),
			P50:         floatToIntPtr(p50),
			P90:         floatToIntPtr(p90),
			P95:         floatToIntPtr(p95),
			Avg:         floatToIntPtr(avg),
			TTFT: service.OpsPercentiles{
				P50: floatToIntPtr(ttftP50),
				P90: floatToIntPtr(ttftP90),
				P95: floatToIntPtr(ttftP95),
				Avg: floatToIntPtr(ttftAvg),
			},
			SampleCount: sampleCount,
		}
		if max.Valid {
			value := int(max.Int64)
			point.Max = &value
		}
		if ttftMax.Valid {
			value := int(ttftMax.Int64)
			point.TTFT.Max = &value
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &service.OpsLatencyTrendResponse{
		Bucket: opsBucketLabel(bucketSeconds),
		Points: fillOpsLatencyBuckets(start, end, bucketSeconds, points),
	}, nil
}

func opsPerformanceBucketExpr(bucketSeconds int) string {
	switch bucketSeconds {
	case 3600:
		return "date_trunc('hour', p.created_at)"
	case 300:
		return "to_timestamp(floor(extract(epoch from p.created_at) / 300) * 300)"
	default:
		return "date_trunc('minute', p.created_at)"
	}
}

func fillOpsLatencyBuckets(start, end time.Time, bucketSeconds int, points []*service.OpsLatencyTrendPoint) []*service.OpsLatencyTrendPoint {
	if bucketSeconds <= 0 {
		bucketSeconds = 60
	}
	if !start.Before(end) {
		return points
	}

	lastInstant := end.Add(-time.Nanosecond)
	if lastInstant.Before(start) {
		return points
	}
	first := opsFloorToBucketStart(start, bucketSeconds)
	last := opsFloorToBucketStart(lastInstant, bucketSeconds)
	step := time.Duration(bucketSeconds) * time.Second
	existing := make(map[int64]*service.OpsLatencyTrendPoint, len(points))
	for _, point := range points {
		if point != nil {
			existing[point.BucketStart.UTC().Unix()] = point
		}
	}

	out := make([]*service.OpsLatencyTrendPoint, 0, int(last.Sub(first)/step)+1)
	for cursor := first; !cursor.After(last); cursor = cursor.Add(step) {
		if point := existing[cursor.Unix()]; point != nil {
			out = append(out, point)
			continue
		}
		out = append(out, &service.OpsLatencyTrendPoint{BucketStart: cursor})
	}
	return out
}
