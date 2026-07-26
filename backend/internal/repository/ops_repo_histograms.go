package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const opsLatencyAvailableUserLimit = 100

type adaptiveLatencyBucket struct {
	minMs int
	maxMs *int
	label string
}

type rankedLatencyUser struct {
	summary     *service.OpsLatencyUserSummary
	avgRank     int
	requestRank int
}

func (r *opsRepository) GetLatencyHistogram(ctx context.Context, filter *service.OpsDashboardFilter) (*service.OpsLatencyHistogramResponse, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil ops repository")
	}
	if filter == nil {
		return nil, fmt.Errorf("nil filter")
	}
	if filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return nil, fmt.Errorf("start_time/end_time required")
	}

	start := filter.StartTime.UTC()
	end := filter.EndTime.UTC()

	users, err := r.queryLatencyUsers(ctx, filter, start, end)
	if err != nil {
		return nil, err
	}

	topAvgUsers := make([]*service.OpsLatencyUserSummary, 0, 3)
	availableUsers := make([]*service.OpsLatencyUserSummary, 0, len(users))
	for _, user := range users {
		if user.avgRank <= 3 {
			topAvgUsers = append(topAvgUsers, user.summary)
		}
		if user.requestRank <= opsLatencyAvailableUserLimit {
			availableUsers = append(availableUsers, user.summary)
		}
	}
	sort.Slice(topAvgUsers, func(i, j int) bool {
		return findLatencyUserRank(users, topAvgUsers[i].UserID, true) < findLatencyUserRank(users, topAvgUsers[j].UserID, true)
	})
	sort.Slice(availableUsers, func(i, j int) bool {
		return findLatencyUserRank(users, availableUsers[i].UserID, false) < findLatencyUserRank(users, availableUsers[j].UserID, false)
	})

	join, where, args, _ := buildUsageWhere(filter, start, end, 1)
	statsQuery := `
SELECT
  COUNT(*),
  AVG(ul.duration_ms)::double precision,
  percentile_cont(0.95) WITHIN GROUP (ORDER BY ul.duration_ms)::double precision,
  MAX(ul.duration_ms)
FROM usage_logs ul
` + join + `
` + where + `
AND ul.duration_ms IS NOT NULL`

	var total int64
	var avgDuration sql.NullFloat64
	var p95Duration sql.NullFloat64
	var maxDuration sql.NullInt64
	if err := r.db.QueryRowContext(ctx, statsQuery, args...).Scan(&total, &avgDuration, &p95Duration, &maxDuration); err != nil {
		return nil, err
	}

	response := &service.OpsLatencyHistogramResponse{
		StartTime:      start,
		EndTime:        end,
		Platform:       strings.TrimSpace(filter.Platform),
		GroupID:        filter.GroupID,
		UserID:         filter.UserID,
		TotalRequests:  total,
		TopAvgUsers:    topAvgUsers,
		AvailableUsers: availableUsers,
		Buckets:        []*service.OpsLatencyHistogramBucket{},
	}
	if avgDuration.Valid {
		value := int(math.Round(avgDuration.Float64))
		response.AvgDurationMs = &value
	}
	if total == 0 || !maxDuration.Valid {
		return response, nil
	}

	p95Ms := int(math.Round(p95Duration.Float64))
	maxMs := int(maxDuration.Int64)
	bucketDefs := buildAdaptiveLatencyBuckets(p95Ms, maxMs)
	counts, err := r.queryAdaptiveLatencyBucketCounts(ctx, join, where, args, bucketDefs)
	if err != nil {
		return nil, err
	}

	response.Buckets = make([]*service.OpsLatencyHistogramBucket, 0, len(bucketDefs))
	for i, bucket := range bucketDefs {
		response.Buckets = append(response.Buckets, &service.OpsLatencyHistogramBucket{
			Range: bucket.label,
			MinMs: bucket.minMs,
			MaxMs: bucket.maxMs,
			Count: counts[i],
		})
	}
	return response, nil
}

func (r *opsRepository) queryLatencyUsers(ctx context.Context, filter *service.OpsDashboardFilter, start, end time.Time) ([]rankedLatencyUser, error) {
	baseFilter := *filter
	baseFilter.UserID = nil
	join, where, args, _ := buildUsageWhere(&baseFilter, start, end, 1)
	selectedUserClause := ""
	if filter.UserID != nil && *filter.UserID > 0 {
		args = append(args, *filter.UserID)
		selectedUserClause = fmt.Sprintf(" OR user_id = $%d", len(args))
	}

	query := `
WITH user_stats AS (
  SELECT
    ul.user_id,
    COALESCE(u.username, '') AS username,
    COALESCE(u.email, '') AS email,
    COALESCE(u.deleted_at IS NOT NULL, FALSE) AS deleted,
    COUNT(*) AS request_count,
    AVG(ul.duration_ms)::double precision AS avg_duration_ms
  FROM usage_logs ul
  ` + join + `
  LEFT JOIN users u ON u.id = ul.user_id
  ` + where + `
    AND ul.duration_ms IS NOT NULL
  GROUP BY ul.user_id, u.username, u.email, u.deleted_at
), ranked AS (
  SELECT
    *,
    ROW_NUMBER() OVER (ORDER BY avg_duration_ms DESC, request_count DESC, user_id ASC) AS avg_rank,
    ROW_NUMBER() OVER (ORDER BY request_count DESC, user_id ASC) AS request_rank
  FROM user_stats
)
SELECT user_id, username, email, deleted, request_count, avg_duration_ms, avg_rank, request_rank
FROM ranked
WHERE avg_rank <= 3 OR request_rank <= ` + fmt.Sprintf("%d", opsLatencyAvailableUserLimit) + selectedUserClause + `
ORDER BY request_rank ASC, user_id ASC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]rankedLatencyUser, 0, opsLatencyAvailableUserLimit)
	for rows.Next() {
		var user service.OpsLatencyUserSummary
		var avg float64
		var avgRank, requestRank int
		if err := rows.Scan(
			&user.UserID,
			&user.Username,
			&user.Email,
			&user.Deleted,
			&user.RequestCount,
			&avg,
			&avgRank,
			&requestRank,
		); err != nil {
			return nil, err
		}
		user.AvgDurationMs = int(math.Round(avg))
		result = append(result, rankedLatencyUser{summary: &user, avgRank: avgRank, requestRank: requestRank})
	}
	return result, rows.Err()
}

func findLatencyUserRank(users []rankedLatencyUser, userID int64, average bool) int {
	for _, user := range users {
		if user.summary.UserID != userID {
			continue
		}
		if average {
			return user.avgRank
		}
		return user.requestRank
	}
	return math.MaxInt
}

func (r *opsRepository) queryAdaptiveLatencyBucketCounts(
	ctx context.Context,
	join string,
	where string,
	baseArgs []any,
	buckets []adaptiveLatencyBucket,
) ([]int64, error) {
	args := append([]any(nil), baseArgs...)
	expressions := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		args = append(args, bucket.minMs)
		minPlaceholder := fmt.Sprintf("$%d", len(args))
		condition := "ul.duration_ms >= " + minPlaceholder
		if bucket.maxMs != nil {
			args = append(args, *bucket.maxMs)
			condition += fmt.Sprintf(" AND ul.duration_ms < $%d", len(args))
		}
		expressions = append(expressions, "COUNT(*) FILTER (WHERE "+condition+")")
	}

	query := "SELECT\n  " + strings.Join(expressions, ",\n  ") + "\nFROM usage_logs ul\n" + join + "\n" + where + "\nAND ul.duration_ms IS NOT NULL"
	counts := make([]int64, len(buckets))
	targets := make([]any, len(buckets))
	for i := range counts {
		targets[i] = &counts[i]
	}
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(targets...); err != nil {
		return nil, err
	}
	return counts, nil
}

func buildAdaptiveLatencyBuckets(p95Ms, maxMs int) []adaptiveLatencyBucket {
	if p95Ms <= 0 {
		p95Ms = maxMs
	}
	if p95Ms <= 0 {
		p95Ms = 1
	}

	step := niceLatencyStep(float64(p95Ms) / 6)
	coreUpper := int(math.Ceil(float64(p95Ms)/float64(step))) * step
	if coreUpper <= 0 {
		coreUpper = step
	}

	buckets := make([]adaptiveLatencyBucket, 0, 8)
	for minMs := 0; minMs < coreUpper; minMs += step {
		maxValue := minMs + step
		buckets = append(buckets, adaptiveLatencyBucket{
			minMs: minMs,
			maxMs: &maxValue,
			label: formatLatencyBoundary(minMs) + " - " + formatLatencyBoundary(maxValue),
		})
	}
	if maxMs >= coreUpper {
		buckets = append(buckets, adaptiveLatencyBucket{
			minMs: coreUpper,
			label: ">= " + formatLatencyBoundary(coreUpper),
		})
	}
	return buckets
}

func niceLatencyStep(raw float64) int {
	if raw <= 1 {
		return 1
	}
	power := math.Pow(10, math.Floor(math.Log10(raw)))
	fraction := raw / power
	niceFraction := 10.0
	switch {
	case fraction <= 1:
		niceFraction = 1
	case fraction <= 2:
		niceFraction = 2
	case fraction <= 5:
		niceFraction = 5
	}
	return max(1, int(math.Round(niceFraction*power)))
}

func formatLatencyBoundary(ms int) string {
	if ms < 1000 {
		return fmt.Sprintf("%d ms", ms)
	}
	if ms%1000 == 0 {
		return fmt.Sprintf("%d s", ms/1000)
	}
	if ms%100 == 0 {
		return fmt.Sprintf("%.1f s", float64(ms)/1000)
	}
	return fmt.Sprintf("%d ms", ms)
}
